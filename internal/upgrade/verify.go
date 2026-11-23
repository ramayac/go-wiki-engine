package upgrade

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

// VerificationStatus captures the outcome of an individual verification step.
type VerificationStatus string

const (
	StatusVerified    VerificationStatus = "verified"
	StatusUnavailable VerificationStatus = "unavailable"
	StatusFailed      VerificationStatus = "failed"
	StatusSkipped     VerificationStatus = "skipped"
)

// VerificationReport explains every security check performed during upgrade.
type VerificationReport struct {
	Tag               string             `json:"tag"`
	Asset             string             `json:"asset,omitempty"`
	Mode              string             `json:"mode"` // "release_binary" or "from_source"
	ChecksumStatus    VerificationStatus `json:"checksum_status"`
	ChecksumExpected  string             `json:"checksum_expected,omitempty"`
	ChecksumComputed  string             `json:"checksum_computed,omitempty"`
	AttestationStatus VerificationStatus `json:"attestation_status"`
	AttestationDetail string             `json:"attestation_detail,omitempty"`
	VersionCheck      VerificationStatus `json:"version_check"`
	InstalledVersion  string             `json:"installed_version,omitempty"`
}

// attestationVerifier verifies build provenance for a downloaded release asset
// file on disk. Injectable so tests can exercise both verified and tampered
// provenance paths without requiring network/GitHub CLI auth.
var attestationVerifier = defaultVerifyAttestation

// defaultVerifyAttestation verifies provenance when available:
//  1. If the release endpoint serves an `<asset>.intoto.jsonl` / provenance
//     bundle, validate its JSON structure and subject digest against computedHash.
//  2. Otherwise, if `gh` CLI is installed and baseURL is GitHub, invoke
//     `gh attestation verify <assetPath> --repo ramayac/go-wiki-engine`.
//
// Returns StatusVerified, StatusUnavailable (if neither source is present), or
// StatusFailed with a non-nil error if verification fails.
func defaultVerifyAttestation(baseURL, tag, assetName, assetPath, computedHash string) (VerificationStatus, string, error) {
	// 1. Check for a release provenance bundle alongside the asset.
	bundleURL := fmt.Sprintf("%s/releases/download/%s/%s.intoto.jsonl", baseURL, tag, assetName)
	resp, err := upgradeHTTPClient.Get(bundleURL)
	if err == nil {
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode == http.StatusOK {
			data, rerr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
			if rerr != nil {
				return StatusFailed, "failed reading provenance bundle", rerr
			}
			if verr := verifyIntotoDigest(data, assetName, computedHash); verr != nil {
				return StatusFailed, verr.Error(), verr
			}
			return StatusVerified, "verified in-toto provenance digest bundle", nil
		}
	}

	// 2. Fall back to `gh attestation verify` only against real GitHub URLs.
	if strings.HasPrefix(baseURL, "https://github.com/") {
		if ghPath, lerr := exec.LookPath("gh"); lerr == nil {
			cmd := exec.Command(ghPath, "attestation", "verify", assetPath, "--repo", "ramayac/go-wiki-engine")
			out, cerr := cmd.CombinedOutput()
			if cerr != nil {
				msg := strings.TrimSpace(string(out))
				if strings.Contains(strings.ToLower(msg), "no attestations found") || strings.Contains(strings.ToLower(msg), "not logged into") {
					return StatusUnavailable, msg, nil
				}
				return StatusFailed, msg, fmt.Errorf("gh attestation verify failed: %s", msg)
			}
			return StatusVerified, "verified via gh attestation verify", nil
		}
	}

	return StatusUnavailable, "no provenance attestation available for this release", nil
}

// verifyIntotoDigest validates an in-toto statement (or Sigstore bundle wrapping
// a subject digest) against the expected asset name and SHA-256 digest.
func verifyIntotoDigest(data []byte, assetName, computedHash string) error {
	var doc struct {
		Subject []struct {
			Name   string            `json:"name"`
			Digest map[string]string `json:"digest"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid provenance JSON: %w", err)
	}
	if len(doc.Subject) == 0 {
		return fmt.Errorf("provenance statement has no subjects")
	}
	for _, s := range doc.Subject {
		if s.Digest["sha256"] == computedHash && (s.Name == "" || s.Name == assetName) {
			return nil
		}
	}
	return fmt.Errorf("provenance subject digest mismatch for %s (expected sha256 %s)", assetName, computedHash)
}
