package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMatchAsset(t *testing.T) {
	tests := []struct {
		filename string
		goos     string
		goarch   string
		expected bool
	}{
		{"wiki-engine_0.2.0_linux_amd64.tar.gz", "linux", "amd64", true},
		{"wiki-engine_0.2.0_darwin_arm64.tar.gz", "darwin", "arm64", true},
		{"wiki-engine_0.2.0_windows_386.zip", "windows", "386", true},
		{"wiki-engine_0.2.0_linux_arm64.tar.gz", "linux", "amd64", false},
		{"wiki-engine_0.2.0_darwin_amd64.tar.gz", "linux", "amd64", false},
		{"other-tool_linux_amd64.tar.gz", "linux", "amd64", false},
	}
	for _, tc := range tests {
		if res := matchAsset(tc.filename, tc.goos, tc.goarch); res != tc.expected {
			t.Errorf("matchAsset(%q, %q, %q) = %t; want %t", tc.filename, tc.goos, tc.goarch, res, tc.expected)
		}
	}
}

func newTarGzWithEntries(t *testing.T, entries []struct {
	name     string
	content  []byte
	typeflag byte
}) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for _, e := range entries {
		tf := e.typeflag
		if tf == 0 {
			tf = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     e.name,
			Mode:     0o755,
			Size:     int64(len(e.content)),
			Typeflag: tf,
		}
		if tf == tar.TypeSymlink {
			hdr.Size = 0
			hdr.Linkname = "/etc/passwd"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if tf == tar.TypeReg {
			if _, err := tw.Write(e.content); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func newTarGzAsset(t *testing.T, content []byte) []byte {
	t.Helper()
	return newTarGzWithEntries(t, []struct {
		name     string
		content  []byte
		typeflag byte
	}{{name: "wiki-engine", content: content}})
}

func newZipAsset(t *testing.T, entryName string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func stubSourceInstaller(t *testing.T) *string {
	t.Helper()
	var installed string
	old := sourceInstaller
	sourceInstaller = func(modulePath string) error {
		installed = modulePath
		return nil
	}
	t.Cleanup(func() { sourceInstaller = old })
	return &installed
}

func stubVersionVerifier(t *testing.T, versionOut string, err error) {
	t.Helper()
	old := versionVerifier
	versionVerifier = func(executablePath string) (string, error) {
		return versionOut, err
	}
	t.Cleanup(func() { versionVerifier = old })
}

func newUpgradeServer(t *testing.T, tag, assetName string, assetData []byte, checksumHex string, provenanceJSON []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/"+tag, http.StatusFound)
	})
	if checksumHex != "" {
		mux.HandleFunc("/releases/download/"+tag+"/checksums.txt", func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprintf(w, "%s  %s\n", checksumHex, assetName)
		})
	}
	mux.HandleFunc("/releases/download/"+tag+"/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(assetData)
	})
	if provenanceJSON != nil {
		mux.HandleFunc("/releases/download/"+tag+"/"+assetName+".intoto.jsonl", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(provenanceJSON)
		})
	}
	return httptest.NewServer(mux)
}

func TestRunSuccessWithProvenanceAndVersionCheck(t *testing.T) {
	binary := []byte("#!/bin/sh\necho v2.0.0\n")
	assetData := newTarGzAsset(t, binary)
	tag := "v2.0.0"
	assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	sum := sha256Hex(assetData)
	prov := []byte(fmt.Sprintf(`{"subject":[{"name":%q,"digest":{"sha256":%q}}]}`, assetName, sum))

	server := newUpgradeServer(t, tag, assetName, assetData, sum, prov)
	defer server.Close()

	sourceCalled := stubSourceInstaller(t)
	if runtime.GOOS == "windows" {
		stubVersionVerifier(t, "v2.0.0", nil)
	}

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	report, err := runWithOptions(server.URL, dest, Options{RequireAttestation: true})
	if err != nil {
		t.Fatalf("runWithOptions failed: %v", err)
	}
	if *sourceCalled != "" {
		t.Error("sourceInstaller invoked unexpectedly on release binary path")
	}
	if report.ChecksumStatus != StatusVerified || report.AttestationStatus != StatusVerified || report.VersionCheck != StatusVerified {
		t.Errorf("unexpected verification report: %+v", report)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(binary) {
		t.Errorf("binary not replaced: got %q", got)
	}
}

func TestRunFailClosedNoSilentFallback(t *testing.T) {
	t.Run("checksum mismatch fails closed", func(t *testing.T) {
		assetData := newTarGzAsset(t, []byte("new"))
		tag := "v2.0.0"
		assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
		server := newUpgradeServer(t, tag, assetName, assetData, strings.Repeat("0", 64), nil)
		defer server.Close()

		sourceCalled := stubSourceInstaller(t)
		dest := filepath.Join(t.TempDir(), "wiki-engine")
		_ = os.WriteFile(dest, []byte("old"), 0o755)

		report, err := runWithOptions(server.URL, dest, Options{})
		if err == nil || !strings.Contains(err.Error(), "checksum validation failed") {
			t.Fatalf("expected checksum error, got: %v", err)
		}
		if report.ChecksumStatus != StatusFailed {
			t.Errorf("ChecksumStatus = %q, want failed", report.ChecksumStatus)
		}
		if *sourceCalled != "" {
			t.Error("fail-closed upgrade must NEVER silently fall back to go install")
		}
	})

	t.Run("tampered provenance fails closed", func(t *testing.T) {
		assetData := newTarGzAsset(t, []byte("new"))
		tag := "v2.0.0"
		assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
		sum := sha256Hex(assetData)
		badProv := []byte(fmt.Sprintf(`{"subject":[{"name":%q,"digest":{"sha256":"deadbeef"}}]}`, assetName))
		server := newUpgradeServer(t, tag, assetName, assetData, sum, badProv)
		defer server.Close()

		sourceCalled := stubSourceInstaller(t)
		dest := filepath.Join(t.TempDir(), "wiki-engine")
		_ = os.WriteFile(dest, []byte("old"), 0o755)

		report, err := runWithOptions(server.URL, dest, Options{})
		if err == nil || !strings.Contains(err.Error(), "provenance attestation verification failed") {
			t.Fatalf("expected provenance error, got: %v", err)
		}
		if report.AttestationStatus != StatusFailed {
			t.Errorf("AttestationStatus = %q, want failed", report.AttestationStatus)
		}
		if *sourceCalled != "" {
			t.Error("fail-closed upgrade must not fall back on provenance failure")
		}
	})

	t.Run("missing platform asset fails closed instead of silent fallback", func(t *testing.T) {
		assetData := newTarGzAsset(t, []byte("other"))
		tag := "v2.0.0"
		other := "wiki-engine_v2.0.0_plan9_386.tar.gz"
		server := newUpgradeServer(t, tag, other, assetData, sha256Hex(assetData), nil)
		defer server.Close()

		sourceCalled := stubSourceInstaller(t)
		dest := filepath.Join(t.TempDir(), "wiki-engine")
		if _, err := runWithOptions(server.URL, dest, Options{}); err == nil {
			t.Fatal("expected failure when no asset matches (no silent fallback)")
		}
		if *sourceCalled != "" {
			t.Errorf("expected no silent fallback, got %q", *sourceCalled)
		}
	})

	t.Run("latest tag error fails closed", func(t *testing.T) {
		server := httptest.NewServer(http.NewServeMux())
		defer server.Close()
		sourceCalled := stubSourceInstaller(t)
		dest := filepath.Join(t.TempDir(), "wiki-engine")
		if _, err := runWithOptions(server.URL, dest, Options{}); err == nil {
			t.Fatal("expected failure on latest tag 404")
		}
		if *sourceCalled != "" {
			t.Error("expected no silent fallback on tag lookup failure")
		}
	})
}

func TestExplicitFromSourcePinnedToTag(t *testing.T) {
	tag := "v2.1.0"
	server := newUpgradeServer(t, tag, "unused", nil, "", nil)
	defer server.Close()

	sourceCalled := stubSourceInstaller(t)
	dest := filepath.Join(t.TempDir(), "wiki-engine")

	report, err := runWithOptions(server.URL, dest, Options{FromSource: true})
	if err != nil {
		t.Fatalf("FromSource failed: %v", err)
	}
	if report.Mode != "from_source" || *sourceCalled != modulePrefix+"@"+tag {
		t.Errorf("FromSource = (%s, %q), want (from_source, %q)", report.Mode, *sourceCalled, modulePrefix+"@"+tag)
	}
}

func TestArchivePathSafetyAndSizeLimits(t *testing.T) {
	// 1. Unsafe tar traversal entry rejected.
	badTar := newTarGzWithEntries(t, []struct {
		name     string
		content  []byte
		typeflag byte
	}{
		{name: "../evil", content: []byte("bad")},
		{name: "wiki-engine", content: []byte("ok")},
	})
	tarPath := filepath.Join(t.TempDir(), "bad.tar.gz")
	_ = os.WriteFile(tarPath, badTar, 0o644)
	if err := extractTarGzFile(tarPath, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Errorf("expected unsafe traversal error, got: %v", err)
	}

	// 2. Symlink entry in tar rejected.
	symTar := newTarGzWithEntries(t, []struct {
		name     string
		content  []byte
		typeflag byte
	}{
		{name: "link", typeflag: tar.TypeSymlink},
	})
	symPath := filepath.Join(t.TempDir(), "sym.tar.gz")
	_ = os.WriteFile(symPath, symTar, 0o644)
	if err := extractTarGzFile(symPath, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Errorf("expected unsafe symlink error, got: %v", err)
	}

	// 3. Unsafe zip entry rejected.
	badZip := newZipAsset(t, "../../wiki-engine", []byte("bad"))
	zipPath := filepath.Join(t.TempDir(), "bad.zip")
	_ = os.WriteFile(zipPath, badZip, 0o644)
	if err := extractZipFile(zipPath, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Errorf("expected unsafe zip traversal error, got: %v", err)
	}

	// 4. Expanded extraction size cap enforced.
	oldMax := maxExtractedSize
	maxExtractedSize = 64
	defer func() { maxExtractedSize = oldMax }()

	bigZip := newZipAsset(t, "wiki-engine", bytes.Repeat([]byte("A"), 128))
	bigZipPath := filepath.Join(t.TempDir(), "big.zip")
	_ = os.WriteFile(bigZipPath, bigZip, 0o644)
	if err := extractZipFile(bigZipPath, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Errorf("expected expanded size limit error, got: %v", err)
	}

	// 5. Compressed download size cap enforced.
	oldDL := maxDownloadSize
	maxDownloadSize = 1024
	defer func() { maxDownloadSize = oldDL }()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 2048))
	}))
	defer srv.Close()
	if _, _, err := downloadToTempFile(srv.URL); err == nil {
		t.Error("downloadToTempFile should reject downloads exceeding maxDownloadSize")
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wiki-engine")
	payload := []byte("#!/bin/sh\necho new\n")
	if err := replaceExecutable(path, payload); err != nil {
		t.Fatalf("replaceExecutable failed: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(payload) {
		t.Errorf("replacement mismatch: %q, err=%v", got, err)
	}
	if err := replaceExecutable(filepath.Join(t.TempDir(), "no-such-dir", "wiki-engine"), payload); err == nil {
		t.Error("expected error for nonexistent target directory")
	}
}
