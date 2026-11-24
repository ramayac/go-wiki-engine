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

func stubAttestation(t *testing.T, status VerificationStatus, detail string, err error) {
	t.Helper()
	old := attestationVerifier
	attestationVerifier = func(baseURL, assetPath string) (VerificationStatus, string, error) {
		return status, detail, err
	}
	t.Cleanup(func() { attestationVerifier = old })
}

func stubVersionVerifier(t *testing.T, versionOut string, err error) {
	t.Helper()
	old := versionVerifier
	versionVerifier = func(executablePath string) (string, error) {
		return versionOut, err
	}
	t.Cleanup(func() { versionVerifier = old })
}

func newUpgradeServer(t *testing.T, tag, assetName string, assetData []byte, checksumHex string) *httptest.Server {
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
	return httptest.NewServer(mux)
}

func TestRunSuccessWithProvenanceAndVersionCheck(t *testing.T) {
	binary := []byte("#!/bin/sh\necho v2.0.0\n")
	assetData := newTarGzAsset(t, binary)
	tag := "v2.0.0"
	assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	sum := sha256Hex(assetData)

	stubAttestation(t, StatusVerified, "verified via gh attestation verify", nil)
	server := newUpgradeServer(t, tag, assetName, assetData, sum)
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
		server := newUpgradeServer(t, tag, assetName, assetData, strings.Repeat("0", 64))
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
		stubAttestation(t, StatusFailed, "bad signature", fmt.Errorf("gh attestation verify failed: bad signature"))
		server := newUpgradeServer(t, tag, assetName, assetData, sum)
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
		server := newUpgradeServer(t, tag, other, assetData, sha256Hex(assetData))
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
	server := newUpgradeServer(t, tag, "unused", nil, "")
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

func TestForgedIntotoFileIsNotProvenance(t *testing.T) {
	assetData := newTarGzAsset(t, []byte("new"))
	tag := "v2.0.0"
	assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	sum := sha256Hex(assetData)
	forged := []byte(fmt.Sprintf(`{"subject":[{"name":%q,"digest":{"sha256":%q}}]}`, assetName, sum))
	server := newUpgradeServer(t, tag, assetName, assetData, sum)
	// A forged provenance file served by the release itself.
	server.Config.Handler.(*http.ServeMux).HandleFunc("/releases/download/"+tag+"/"+assetName+".intoto.jsonl", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(forged)
	})
	defer server.Close()
	stubSourceInstaller(t)
	stubVersionVerifier(t, "v2.0.0", nil)

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	_ = os.WriteFile(dest, []byte("old"), 0o755)
	if _, err := runWithOptions(server.URL, dest, Options{RequireAttestation: true}); err == nil {
		t.Fatal("a release-served .intoto.jsonl must not satisfy --require-attestation")
	}

	report, err := runWithOptions(server.URL, dest, Options{})
	if err != nil {
		t.Fatalf("upgrade without --require-attestation should proceed on checksum: %v", err)
	}
	if report.AttestationStatus != StatusUnavailable {
		t.Errorf("AttestationStatus = %q, want unavailable", report.AttestationStatus)
	}
}

func TestAttestationStatusTable(t *testing.T) {
	const githubURL = "https://github.com/ramayac/go-wiki-engine"
	ghErr := fmt.Errorf("exit status 1")
	tests := []struct {
		name    string
		baseURL string
		output  string
		found   bool
		ghErr   error
		want    VerificationStatus
	}{
		{"gh succeeds", githubURL, "ok", true, nil, StatusVerified},
		{"gh missing", githubURL, "", false, nil, StatusUnavailable},
		{"gh not logged in", githubURL, "You are not logged into any GitHub hosts.", true, ghErr, StatusUnavailable},
		{"no attestations found, mixed case", githubURL, "\n  Error: No Attestations Found for subject", true, ghErr, StatusUnavailable},
		{"non-GitHub base URL never calls gh", "http://127.0.0.1:1234", "", true, nil, StatusUnavailable},
		{"unrecognised gh error", githubURL, "verification failed: signer workflow mismatch", true, ghErr, StatusFailed},
		{"unavailable words after first lines are ignored", githubURL, "bad\nsignature\nmismatch\nno attestations found", true, ghErr, StatusFailed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			old := ghAttestationVerify
			ghAttestationVerify = func(string) (string, bool, error) {
				called = true
				return tc.output, tc.found, tc.ghErr
			}
			t.Cleanup(func() { ghAttestationVerify = old })

			status, _, err := defaultVerifyAttestation(tc.baseURL, "asset")
			if status != tc.want {
				t.Fatalf("status = %q, want %q", status, tc.want)
			}
			if (status == StatusFailed) != (err != nil) {
				t.Errorf("status %q with err %v: error must be set exactly for failed", status, err)
			}
			if !strings.HasPrefix(tc.baseURL, "https://github.com/") && called {
				t.Error("gh must not run for a non-GitHub base URL")
			}
		})
	}
}

func TestGhAttestationUnavailableRespectsRequireAttestation(t *testing.T) {
	assetData := newTarGzAsset(t, []byte("new"))
	tag := "v2.0.0"
	assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	server := newUpgradeServer(t, tag, assetName, assetData, sha256Hex(assetData))
	defer server.Close()
	stubSourceInstaller(t)
	stubVersionVerifier(t, "v2.0.0", nil)

	for _, tc := range []struct {
		name    string
		status  VerificationStatus
		err     error
		require bool
		wantErr bool
	}{
		{"unavailable passes without flag", StatusUnavailable, nil, false, false},
		{"unavailable fails with flag", StatusUnavailable, nil, true, true},
		{"failed fails without flag", StatusFailed, fmt.Errorf("boom"), false, true},
		{"failed fails with flag", StatusFailed, fmt.Errorf("boom"), true, true},
		{"verified passes with flag", StatusVerified, nil, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubAttestation(t, tc.status, "detail", tc.err)
			dest := filepath.Join(t.TempDir(), "wiki-engine")
			_ = os.WriteFile(dest, []byte("old"), 0o755)
			_, err := runWithOptions(server.URL, dest, Options{RequireAttestation: tc.require})
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %t", err, tc.wantErr)
			}
		})
	}
}

func TestValidateTag(t *testing.T) {
	for _, tag := range []string{"1.5.1", "v1.5.1", "v1.5.1-rc.1", "v1.5.1+build.5"} {
		if err := validateTag(tag); err != nil {
			t.Errorf("validateTag(%q) = %v, want nil", tag, err)
		}
	}
	for _, tag := range []string{"../x", "-x", "v1;rm", "a/b", "", strings.Repeat("1", 200), "v1.5", "v1.5.1 ", "v99999999999999999999.0.0"} {
		if err := validateTag(tag); err == nil {
			t.Errorf("validateTag(%q) = nil, want error", tag)
		}
	}
}

func TestRunRejectsInvalidTagBeforeAnyDownload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/releases/tag/v1;rm", http.StatusFound)
	})
	downloads := 0
	mux.HandleFunc("/releases/download/", func(w http.ResponseWriter, r *http.Request) { downloads++ })
	server := httptest.NewServer(mux)
	defer server.Close()
	sourceCalled := stubSourceInstaller(t)

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	for _, opts := range []Options{{}, {FromSource: true}} {
		if _, err := runWithOptions(server.URL, dest, opts); err == nil || !strings.Contains(err.Error(), "invalid release tag") {
			t.Errorf("opts %+v: expected invalid release tag error, got %v", opts, err)
		}
	}
	if downloads != 0 || *sourceCalled != "" {
		t.Errorf("no download or go install may run for an invalid tag (downloads=%d, source=%q)", downloads, *sourceCalled)
	}
}

func TestDowngradeGuard(t *testing.T) {
	tests := []struct {
		name    string
		current string
		tag     string
		refuse  bool
	}{
		{"lower tag refused", "v1.5.1", "v1.4.9", true},
		{"lower minor refused", "v1.5.1", "v1.4.99", true},
		{"equal allowed", "v1.5.1", "v1.5.1", false},
		{"higher allowed", "v1.5.1", "v1.6.0", false},
		{"numeric not lexical", "v1.9.0", "v1.10.0", false},
		{"dev build allowed", "dev", "v0.0.1", false},
		{"empty version allowed", "", "v0.0.1", false},
		{"dirty build allowed", "v1.5.1-dirty-2026-11-23T10:00:00Z", "v1.0.0", false},
		{"git describe build allowed", "v1.5.1-3-gabc1234-2026-11-23T10:00:00Z", "v1.0.0", false},
		{"v prefix on tag only", "1.5.1", "v1.4.0", true},
		{"v prefix on running only", "v1.5.1", "1.4.0", true},
		{"release build string refused", "v1.5.1-abc1234-2026-11-23T10:00:00Z", "v1.5.0", true},
		{"release build string equal allowed", "v1.5.1-abc1234-2026-11-23T10:00:00Z", "v1.5.1", false},
		{"prerelease tag lower than running release", "v1.5.1", "v1.5.1-rc.1", true},
		{"release tag higher than running prerelease", "v1.5.1-rc.1", "v1.5.1", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assetData := newTarGzAsset(t, []byte("new"))
			assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tc.tag, runtime.GOOS, runtime.GOARCH)
			server := newUpgradeServer(t, tc.tag, assetName, assetData, sha256Hex(assetData))
			defer server.Close()
			sourceCalled := stubSourceInstaller(t)
			stubAttestation(t, StatusUnavailable, "none", nil)
			stubVersionVerifier(t, tc.tag, nil)

			dest := filepath.Join(t.TempDir(), "wiki-engine")
			_ = os.WriteFile(dest, []byte("old"), 0o755)
			_, err := runWithOptions(server.URL, dest, Options{CurrentVersion: tc.current})
			if tc.refuse {
				if err == nil || !strings.Contains(err.Error(), "downgrade") || !strings.Contains(err.Error(), tc.tag) || !strings.Contains(err.Error(), tc.current) {
					t.Fatalf("expected downgrade error naming both versions, got: %v", err)
				}
				if got, _ := os.ReadFile(dest); string(got) != "old" {
					t.Error("binary must not be replaced on downgrade")
				}
				return
			}
			if err != nil {
				t.Fatalf("expected upgrade to proceed, got: %v", err)
			}
			if *sourceCalled != "" {
				t.Error("unexpected go install")
			}
		})
	}

	t.Run("from-source also guarded", func(t *testing.T) {
		server := newUpgradeServer(t, "v1.0.0", "unused", nil, "")
		defer server.Close()
		sourceCalled := stubSourceInstaller(t)
		_, err := runWithOptions(server.URL, filepath.Join(t.TempDir(), "wiki-engine"), Options{FromSource: true, CurrentVersion: "v1.5.1"})
		if err == nil || *sourceCalled != "" {
			t.Fatalf("expected downgrade refusal before go install, err=%v source=%q", err, *sourceCalled)
		}
	})
}
