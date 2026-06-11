package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
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
	tw.Close()
	gw.Close()

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
	zw.Close()

	extracted, err := extractZip(buf.Bytes())
	if err != nil {
		t.Fatalf("unexpected error extracting zip: %v", err)
	}
	if string(extracted) != "zip-binary-content" {
		t.Errorf("unexpected extracted content: %s", string(extracted))
	}
}
