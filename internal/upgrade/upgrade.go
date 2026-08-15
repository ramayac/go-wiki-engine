// Package upgrade implements self-upgrade with SHA-256 checksum validation.
package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// upgradeHTTPClient is shared by all upgrade network calls. Redirects are
// followed manually via ErrUseLastResponse so getLatestTag can read the
// Location header of GitHub's /releases/latest redirect.
var upgradeHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

const repoURL = "https://github.com/ramayac/go-wiki-engine"
const module = "github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest"

// Run executes the self-upgrade. It attempts to download the latest precompiled
// binary from GitHub, validates its SHA-256 checksum, and replaces the running binary.
// If it fails to find or download a release binary (e.g., in offline environments or if
// Go toolchain is preferred), it falls back to `go install`.
func Run() error {
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to locate running executable: %w", err)
	}

	fmt.Fprintf(os.Stderr, "checking for latest release at %s...\n", repoURL)
	tag, err := getLatestTag()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to get latest release tag: %v\n", err)
		return fallbackGoInstall()
	}
	fmt.Fprintf(os.Stderr, "latest release version is %s\n", tag)

	// Fetch checksums.txt
	checksumsURL := fmt.Sprintf("%s/releases/download/%s/checksums.txt", repoURL, tag)
	fmt.Fprintf(os.Stderr, "fetching checksums from %s...\n", checksumsURL)
	checksumsData, err := downloadBytes(checksumsURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to download checksums: %v\n", err)
		return fallbackGoInstall()
	}

	// Parse checksums to match our OS and Arch
	assetName, expectedHash, err := matchAssetInChecksums(string(checksumsData), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
		return fallbackGoInstall()
	}
	fmt.Fprintf(os.Stderr, "matched release asset: %s (expected hash: %s)\n", assetName, expectedHash)

	// Download asset
	assetURL := fmt.Sprintf("%s/releases/download/%s/%s", repoURL, tag, assetName)
	fmt.Fprintf(os.Stderr, "downloading asset from %s...\n", assetURL)
	assetData, err := downloadBytes(assetURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to download asset: %v\n", err)
		return fallbackGoInstall()
	}

	// Verify SHA-256 checksum
	hash := sha256.Sum256(assetData)
	computedHash := hex.EncodeToString(hash[:])
	if computedHash != expectedHash {
		return fmt.Errorf("checksum validation failed for %s: computed %s, expected %s", assetName, computedHash, expectedHash)
	}
	fmt.Fprintln(os.Stderr, "checksum validation succeeded")

	// Extract binary
	var binaryBytes []byte
	if strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz") {
		binaryBytes, err = extractTarGz(assetData)
	} else if strings.HasSuffix(assetName, ".zip") {
		binaryBytes, err = extractZip(assetData)
	} else {
		// Raw binary
		binaryBytes = assetData
	}
	if err != nil {
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Replace the running binary
	err = replaceExecutable(executablePath, binaryBytes)
	if err != nil {
		return fmt.Errorf("failed to replace executable: %w", err)
	}

	fmt.Fprintln(os.Stderr, "upgrade complete")
	fmt.Fprintln(os.Stderr, "run `wiki-engine sync-prompts` in each repo to update prompts and instructions for all supported AI tools")
	return nil
}

func getLatestTag() (string, error) {
	resp, err := upgradeHTTPClient.Get(repoURL + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("unexpected status fetching latest redirect: %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("missing Location header in redirect")
	}

	// Location is e.g. "https://github.com/ramayac/go-wiki-engine/releases/tag/v0.2.0"
	parts := strings.Split(loc, "/tag/")
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected redirect location format: %s", loc)
	}
	return parts[1], nil
}

func downloadBytes(url string) ([]byte, error) {
	resp, err := upgradeHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func matchAssetInChecksums(checksumsContent, goos, goarch string) (string, string, error) {
	lines := strings.Split(checksumsContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		filename := parts[1]

		if matchAsset(filename, goos, goarch) {
			return filename, hash, nil
		}
	}
	return "", "", fmt.Errorf("no matching asset found in checksums.txt for %s/%s", goos, goarch)
}

func matchAsset(filename, goos, goarch string) bool {
	if !strings.Contains(filename, "wiki-engine") {
		return false
	}
	osPattern1 := "_" + goos + "_"
	osPattern2 := "_" + goos + "."
	if !strings.Contains(filename, osPattern1) && !strings.Contains(filename, osPattern2) {
		return false
	}
	archPattern1 := "_" + goarch + "_"
	archPattern2 := "_" + goarch + "."
	if !strings.Contains(filename, archPattern1) && !strings.Contains(filename, archPattern2) {
		return false
	}
	return true
}

func extractTarGz(gzipData []byte) ([]byte, error) {
	gr, err := gzip.NewReader(bytes.NewReader(gzipData))
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		name := filepath.Base(header.Name)
		if name == "wiki-engine" || name == "wiki-engine.exe" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("binary not found in tar.gz archive")
}

func extractZip(zipData []byte) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, err
	}

	for _, f := range r.File {
		name := filepath.Base(f.Name)
		if name == "wiki-engine" || name == "wiki-engine.exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("binary not found in zip archive")
}

func replaceExecutable(executablePath string, newBytes []byte) error {
	dir := filepath.Dir(executablePath)
	tmpFile, err := os.CreateTemp(dir, "wiki-engine-upgrade-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(newBytes); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		oldPath := executablePath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(executablePath, oldPath); err != nil {
			return fmt.Errorf("failed to move running executable on Windows: %w", err)
		}
		defer os.Remove(oldPath)
	}

	if err := os.Rename(tmpPath, executablePath); err != nil {
		return err
	}

	return nil
}

func fallbackGoInstall() error {
	fmt.Fprintln(os.Stderr, "falling back to `go install`...")
	gobin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go not found in PATH; install Go or download a release binary from GitHub")
	}

	cmd := exec.Command(gobin, "install", module)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	fmt.Fprintf(os.Stderr, "running: go install %s\n", module)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upgrade fallback failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "upgrade fallback complete")
	return nil
}
