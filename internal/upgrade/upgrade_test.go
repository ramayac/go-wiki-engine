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

func TestModuleConstant(t *testing.T) {
	if module == "" {
		t.Error("module constant is empty")
	}
	if module != "github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest" {
		t.Errorf("unexpected module: %s", module)
	}
}

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
		res := matchAsset(tc.filename, tc.goos, tc.goarch)
		if res != tc.expected {
			t.Errorf("matchAsset(%q, %q, %q) = %t; expected %t", tc.filename, tc.goos, tc.goarch, res, tc.expected)
		}
	}
}

func TestMatchAssetInChecksums(t *testing.T) {
	checksums := `
1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef  wiki-engine_0.2.0_linux_amd64.tar.gz
abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890  wiki-engine_0.2.0_darwin_arm64.tar.gz
`
	filename, hash, err := matchAssetInChecksums(checksums, "linux", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filename != "wiki-engine_0.2.0_linux_amd64.tar.gz" {
		t.Errorf("unexpected filename: %s", filename)
	}
	if hash != "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef" {
		t.Errorf("unexpected hash: %s", hash)
	}

	_, _, err = matchAssetInChecksums(checksums, "windows", "amd64")
	if err == nil {
		t.Error("expected error for unmatched asset, got nil")
	}
}

func TestExtractTarGz(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	fileContent := []byte("binary-content")
	hdr := &tar.Header{
		Name: "wiki-engine",
		Mode: 0755,
		Size: int64(len(fileContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(fileContent); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	extracted, err := extractTarGz(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error extracting tar.gz: %v", err)
	}
	if string(extracted) != "binary-content" {
		t.Errorf("unexpected extracted content: %s", string(extracted))
	}
}

func TestExtractZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	f, err := zw.Create("wiki-engine")
	if err != nil {
		t.Fatal(err)
	}
	fileContent := []byte("zip-binary-content")
	if _, err := f.Write(fileContent); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	extracted, err := extractZip(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error extracting zip: %v", err)
	}
	if string(extracted) != "zip-binary-content" {
		t.Errorf("unexpected extracted content: %s", string(extracted))
	}
}

// --- download-path tests (todo #54) ---

// newTarGzAsset packages content as a tar.gz archive containing the binary.
func newTarGzAsset(t *testing.T, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{
		Name: "wiki-engine",
		Mode: 0755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// newUpgradeServer serves a GitHub-releases-shaped API: a /releases/latest
// redirect and the checksums + asset downloads for one tag. checksumHex may
// be empty to serve no checksums endpoint.
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

// stubFallback replaces fallbackInstaller for the duration of a test and
// returns a flag recording whether it was called.
func stubFallback(t *testing.T) *bool {
	t.Helper()
	called := false
	old := fallbackInstaller
	fallbackInstaller = func() error {
		called = true
		return nil
	}
	t.Cleanup(func() { fallbackInstaller = old })
	return &called
}

func TestRunSuccess(t *testing.T) {
	binary := []byte("v1.0.0-binary-content")
	assetData := newTarGzAsset(t, binary)
	tag := "v1.0.0"
	assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	server := newUpgradeServer(t, tag, assetName, assetData, sha256Hex(assetData))
	defer server.Close()

	fallbackCalled := stubFallback(t)

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := run(server.URL, dest); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if *fallbackCalled {
		t.Error("fallback invoked unexpectedly on the success path")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(binary) {
		t.Errorf("binary not replaced: got %q, want %q", string(got), string(binary))
	}
}

func TestRunChecksumMismatch(t *testing.T) {
	binary := []byte("v1.0.0-binary-content")
	assetData := newTarGzAsset(t, binary)
	tag := "v1.0.0"
	assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	wrongHash := strings.Repeat("0", 64)
	server := newUpgradeServer(t, tag, assetName, assetData, wrongHash)
	defer server.Close()

	fallbackCalled := stubFallback(t)

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := run(server.URL, dest)
	if err == nil || !strings.Contains(err.Error(), "checksum validation failed") {
		t.Fatalf("expected checksum validation error, got: %v", err)
	}
	if *fallbackCalled {
		t.Error("fallback should not run after a checksum failure")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Errorf("binary modified on failure: got %q", string(got))
	}
}

func TestRunFallsBackWhenNoMatchingAsset(t *testing.T) {
	assetData := newTarGzAsset(t, []byte("other-platform"))
	tag := "v1.0.0"
	otherAsset := "wiki-engine_" + tag + "_darwin_amd64.tar.gz"
	if runtime.GOOS == "darwin" {
		otherAsset = "wiki-engine_" + tag + "_linux_amd64.tar.gz"
	}
	server := newUpgradeServer(t, tag, otherAsset, assetData, sha256Hex(assetData))
	defer server.Close()

	fallbackCalled := stubFallback(t)

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := run(server.URL, dest); err != nil {
		t.Fatalf("run should fall back cleanly, got error: %v", err)
	}
	if !*fallbackCalled {
		t.Error("expected fallback to be invoked when no asset matches")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "old-binary" {
		t.Errorf("binary modified despite fallback: got %q", string(got))
	}
}

func TestRunFallsBackOnLatestTagError(t *testing.T) {
	// No /releases/latest route — the server returns 404.
	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()

	fallbackCalled := stubFallback(t)

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := run(server.URL, dest); err != nil {
		t.Fatalf("run should fall back cleanly, got error: %v", err)
	}
	if !*fallbackCalled {
		t.Error("expected fallback to be invoked when the latest-tag lookup fails")
	}
}

func TestRunExtractFailure(t *testing.T) {
	// Checksum is valid for the bytes served, but they are not a tar.gz
	// archive — extraction must fail without touching the fallback.
	assetData := []byte("not a real archive")
	tag := "v1.0.0"
	assetName := fmt.Sprintf("wiki-engine_%s_%s_%s.tar.gz", tag, runtime.GOOS, runtime.GOARCH)
	server := newUpgradeServer(t, tag, assetName, assetData, sha256Hex(assetData))
	defer server.Close()

	fallbackCalled := stubFallback(t)

	dest := filepath.Join(t.TempDir(), "wiki-engine")
	if err := os.WriteFile(dest, []byte("old-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := run(server.URL, dest)
	if err == nil || !strings.Contains(err.Error(), "failed to extract binary") {
		t.Fatalf("expected extraction error, got: %v", err)
	}
	if *fallbackCalled {
		t.Error("fallback should not run after checksum-verified extraction failure")
	}
}
