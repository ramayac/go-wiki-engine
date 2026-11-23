// Package upgrade implements fail-closed self-upgrade with SHA-256 checksum
// verification, optional provenance attestation checks, streamed size-bounded
// archive extraction, and explicit --from-source installation.
package upgrade

import (
	"archive/tar"
	"archive/zip"
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

// upgradeHTTPClient follows asset redirects to signed storage URLs.
var upgradeHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
}

// latestTagHTTPClient stops at the first response so getLatestTag reads the
// Location header of GitHub's /releases/latest redirect.
var latestTagHTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

const modulePrefix = "github.com/ramayac/go-wiki-engine/cmd/wiki-engine"

// Size caps for both compressed download and expanded binary extraction.
// Variables so unit tests can lower them.
var (
	maxDownloadSize  = int64(100 << 20) // 100 MiB compressed
	maxExtractedSize = int64(200 << 20) // 200 MiB expanded
)

// repoURL is injectable for httptest-backed tests.
var repoURL = "https://github.com/ramayac/go-wiki-engine"

// versionVerifier executes `<executablePath> version` after replacement.
// Injectable so unit tests can verify both success and failure handling.
var versionVerifier = func(executablePath string) (string, error) {
	out, err := exec.Command(executablePath, "version").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// sourceInstaller runs `go install <modulePath>` ONLY when explicitly requested
// via `--from-source`.
var sourceInstaller = func(modulePath string) error {
	gobin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go not found in PATH; install Go or download a release binary from GitHub")
	}

	cmd := exec.Command(gobin, "install", modulePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	fmt.Fprintf(os.Stderr, "running: go install %s\n", modulePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("from-source install failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "from-source install complete")
	return nil
}

// Options configures upgrade behavior.
type Options struct {
	FromSource         bool // explicitly install from source via `go install` pinned to release tag
	RequireAttestation bool // fail if build provenance attestation is unavailable
}

// Run executes the fail-closed default self-upgrade.
func Run() error {
	_, err := RunWithOptions(Options{})
	return err
}

// RunWithOptions executes the upgrade and returns a detailed VerificationReport.
func RunWithOptions(opts Options) (*VerificationReport, error) {
	executablePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to locate running executable: %w", err)
	}
	return runWithOptions(repoURL, executablePath, opts)
}

// runWithOptions implements the fail-closed upgrade flow:
//  1. Discover release tag.
//  2. If --from-source, run pinned `go install <module>@<tag>` and stop.
//  3. Download checksums.txt.
//  4. Match platform asset.
//  5. Stream release asset to disk (size-capped) while computing SHA-256.
//  6. Verify checksum — fail closed on mismatch or download failure.
//  7. Verify build provenance attestation when available (fail closed on failure,
//     or when --require-attestation is set and unavailable).
//  8. Extract binary from disk archive with path-safety and expanded-size caps.
//  9. Stage and atomically replace the executable.
//  10. Execute `version` on the replacement and fail if it does not succeed.
func runWithOptions(baseURL, executablePath string, opts Options) (*VerificationReport, error) {
	report := &VerificationReport{
		Mode:              "release_binary",
		ChecksumStatus:    StatusSkipped,
		AttestationStatus: StatusSkipped,
		VersionCheck:      StatusSkipped,
	}

	fmt.Fprintf(os.Stderr, "checking for latest release at %s...\n", baseURL)
	tag, err := getLatestTag(baseURL)
	if err != nil {
		return report, fmt.Errorf("upgrade failed: could not resolve latest release tag: %w (no silent fallback; use --from-source to build with go install)", err)
	}
	report.Tag = tag
	fmt.Fprintf(os.Stderr, "latest release version is %s\n", tag)

	// Explicit --from-source path: pinned strictly to the resolved release tag.
	if opts.FromSource {
		report.Mode = "from_source"
		target := modulePrefix + "@" + tag
		if err := sourceInstaller(target); err != nil {
			return report, err
		}
		return report, nil
	}

	// 1. Download checksums.txt (fail closed).
	checksumsURL := fmt.Sprintf("%s/releases/download/%s/checksums.txt", baseURL, tag)
	fmt.Fprintf(os.Stderr, "fetching checksums from %s...\n", checksumsURL)
	checksumsData, err := downloadBytes(checksumsURL)
	if err != nil {
		return report, fmt.Errorf("upgrade failed: could not download checksums.txt: %w (use --from-source to build from tag %s)", err, tag)
	}

	// 2. Match OS/Arch asset.
	assetName, expectedHash, err := matchAssetInChecksums(string(checksumsData), runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return report, fmt.Errorf("upgrade failed: %w (use --from-source to build from tag %s)", err, tag)
	}
	report.Asset = assetName
	report.ChecksumExpected = expectedHash
	fmt.Fprintf(os.Stderr, "matched release asset: %s (expected sha256: %s)\n", assetName, expectedHash)

	// 3. Stream asset directly to a temporary file on disk with size cap + incremental SHA-256.
	assetURL := fmt.Sprintf("%s/releases/download/%s/%s", baseURL, tag, assetName)
	fmt.Fprintf(os.Stderr, "downloading asset from %s...\n", assetURL)
	tmpAsset, computedHash, err := downloadToTempFile(assetURL)
	if err != nil {
		return report, fmt.Errorf("upgrade failed: could not download release asset %s: %w", assetName, err)
	}
	defer func() { _ = os.Remove(tmpAsset) }()
	report.ChecksumComputed = computedHash

	// 4. Verify SHA-256 checksum (fail closed).
	if !strings.EqualFold(computedHash, expectedHash) {
		report.ChecksumStatus = StatusFailed
		return report, fmt.Errorf("checksum validation failed for %s: computed %s, expected %s", assetName, computedHash, expectedHash)
	}
	report.ChecksumStatus = StatusVerified
	fmt.Fprintln(os.Stderr, "checksum validation succeeded")

	// 5. Verify build provenance attestation when available (fail closed on failure).
	attStatus, attDetail, attErr := attestationVerifier(baseURL, tag, assetName, tmpAsset, computedHash)
	report.AttestationStatus = attStatus
	report.AttestationDetail = attDetail
	if attErr != nil || attStatus == StatusFailed {
		return report, fmt.Errorf("provenance attestation verification failed for %s: %v", assetName, attErr)
	}
	if opts.RequireAttestation && attStatus != StatusVerified {
		return report, fmt.Errorf("provenance attestation required but unavailable for %s: %s", assetName, attDetail)
	}
	fmt.Fprintf(os.Stderr, "provenance attestation: %s (%s)\n", attStatus, attDetail)

	// 6. Stage extracted binary to a temp file next to executablePath (size & path-safety checked).
	stagePath, err := extractBinaryToStage(tmpAsset, assetName, filepath.Dir(executablePath))
	if err != nil {
		return report, fmt.Errorf("failed to extract binary: %w", err)
	}
	defer func() { _ = os.Remove(stagePath) }()

	// 7. Atomically swap staged binary into executablePath.
	if err := swapExecutable(stagePath, executablePath); err != nil {
		return report, fmt.Errorf("failed to replace executable: %w", err)
	}

	// 8. Verify replacement binary executes `version` cleanly (fail closed).
	verOut, verErr := versionVerifier(executablePath)
	if verErr != nil {
		report.VersionCheck = StatusFailed
		return report, fmt.Errorf("replacement binary failed version check: %w", verErr)
	}
	report.VersionCheck = StatusVerified
	report.InstalledVersion = verOut
	fmt.Fprintf(os.Stderr, "upgraded to %s\n", verOut)
	fmt.Fprintln(os.Stderr, "run `wiki-engine init` or `wiki-engine sync-prompts` in each repo to reconcile prompts and instructions")
	return report, nil
}

func getLatestTag(baseURL string) (string, error) {
	resp, err := latestTagHTTPClient.Get(baseURL + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusMovedPermanently {
		return "", fmt.Errorf("unexpected status fetching latest redirect: %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("missing Location header in redirect")
	}
	parts := strings.Split(loc, "/tag/")
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", fmt.Errorf("unexpected redirect location format: %s", loc)
	}
	return strings.TrimSpace(parts[1]), nil
}

func downloadBytes(url string) ([]byte, error) {
	resp, err := upgradeHTTPClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxDownloadSize {
		return nil, fmt.Errorf("download exceeds %d bytes", maxDownloadSize)
	}
	return data, nil
}

// downloadToTempFile streams an HTTP response body to a temp file on disk,
// enforcing maxDownloadSize and computing SHA-256 in a single pass.
func downloadToTempFile(url string) (tmpPath string, sha256Hex string, err error) {
	resp, err := upgradeHTTPClient.Get(url)
	if err != nil {
		return "", "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("status: %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "wiki-engine-asset-*")
	if err != nil {
		return "", "", err
	}
	tmpPath = tmp.Name()
	cleanup := true
	defer func() {
		_ = tmp.Close()
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	h := sha256.New()
	limited := io.LimitReader(resp.Body, maxDownloadSize+1)
	n, err := io.Copy(io.MultiWriter(tmp, h), limited)
	if err != nil {
		return "", "", err
	}
	if n > maxDownloadSize {
		return "", "", fmt.Errorf("download exceeds %d bytes", maxDownloadSize)
	}
	if err := tmp.Sync(); err != nil {
		return "", "", err
	}
	cleanup = false
	return tmpPath, hex.EncodeToString(h.Sum(nil)), nil
}

func matchAssetInChecksums(checksumsContent, goos, goarch string) (string, string, error) {
	for _, line := range strings.Split(checksumsContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		hash := parts[0]
		filename := strings.TrimPrefix(parts[1], "*")
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

// validateArchiveEntryPath rejects archive paths containing traversal, absolute
// paths, drive prefixes, or NUL bytes.
func validateArchiveEntryPath(entryName string) error {
	if entryName == "" {
		return fmt.Errorf("empty archive entry name")
	}
	if strings.ContainsRune(entryName, '\x00') {
		return fmt.Errorf("unsafe archive entry path (NUL byte): %q", entryName)
	}
	norm := strings.ReplaceAll(entryName, `\`, "/")
	if strings.HasPrefix(norm, "/") || filepath.IsAbs(entryName) {
		return fmt.Errorf("unsafe absolute archive entry path: %q", entryName)
	}
	if len(norm) >= 2 && norm[1] == ':' {
		return fmt.Errorf("unsafe drive-prefixed archive entry path: %q", entryName)
	}
	for _, seg := range strings.Split(norm, "/") {
		if seg == ".." {
			return fmt.Errorf("unsafe traversal in archive entry path: %q", entryName)
		}
	}
	return nil
}

func extractBinaryToStage(assetPath, assetName, stageDir string) (string, error) {
	tmpStage, err := os.CreateTemp(stageDir, "wiki-engine-stage-*")
	if err != nil {
		return "", err
	}
	stagePath := tmpStage.Name()
	cleanup := true
	defer func() {
		_ = tmpStage.Close()
		if cleanup {
			_ = os.Remove(stagePath)
		}
	}()

	var extractErr error
	switch {
	case strings.HasSuffix(assetName, ".tar.gz") || strings.HasSuffix(assetName, ".tgz"):
		extractErr = extractTarGzFile(assetPath, tmpStage)
	case strings.HasSuffix(assetName, ".zip"):
		extractErr = extractZipFile(assetPath, tmpStage)
	default:
		f, err := os.Open(assetPath)
		if err != nil {
			return "", err
		}
		_, extractErr = copyBounded(tmpStage, f, maxExtractedSize)
		_ = f.Close()
	}
	if extractErr != nil {
		return "", extractErr
	}
	if err := tmpStage.Sync(); err != nil {
		return "", err
	}
	if err := tmpStage.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(stagePath, 0o755); err != nil {
		return "", err
	}
	cleanup = false
	return stagePath, nil
}

func extractTarGzFile(archivePath string, dst io.Writer) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	found := false
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := validateArchiveEntryPath(header.Name); err != nil {
			return err
		}
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			return fmt.Errorf("unsafe link entry in archive: %q", header.Name)
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != 0 {
			continue
		}
		base := filepath.Base(strings.ReplaceAll(header.Name, `\`, "/"))
		if base == "wiki-engine" || base == "wiki-engine.exe" {
			if _, err := copyBounded(dst, tr, maxExtractedSize); err != nil {
				return err
			}
			found = true
			// Continue scanning remaining headers so any malicious later entry is still caught.
		}
	}
	if !found {
		return fmt.Errorf("binary not found in tar.gz archive")
	}
	return nil
}

func extractZipFile(archivePath string, dst io.Writer) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()

	found := false
	for _, zf := range zr.File {
		if err := validateArchiveEntryPath(zf.Name); err != nil {
			return err
		}
		if zf.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe symlink entry in zip archive: %q", zf.Name)
		}
		if zf.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(strings.ReplaceAll(zf.Name, `\`, "/"))
		if (base == "wiki-engine" || base == "wiki-engine.exe") && !found {
			rc, err := zf.Open()
			if err != nil {
				return err
			}
			_, cerr := copyBounded(dst, rc, maxExtractedSize)
			_ = rc.Close()
			if cerr != nil {
				return cerr
			}
			found = true
		}
	}
	if !found {
		return fmt.Errorf("binary not found in zip archive")
	}
	return nil
}

func copyBounded(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("extracted binary exceeds maximum size (%d bytes)", limit)
	}
	return n, nil
}

func swapExecutable(stagePath, executablePath string) error {
	if runtime.GOOS == "windows" {
		oldPath := executablePath + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(executablePath, oldPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to move running executable on Windows: %w", err)
		}
		defer func() { _ = os.Remove(oldPath) }()
	}
	return os.Rename(stagePath, executablePath)
}

// replaceExecutable stages newBytes and swaps atomically into executablePath.
func replaceExecutable(executablePath string, newBytes []byte) error {
	dir := filepath.Dir(executablePath)
	tmpFile, err := os.CreateTemp(dir, "wiki-engine-upgrade-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write(newBytes); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return err
	}
	return swapExecutable(tmpPath, executablePath)
}
