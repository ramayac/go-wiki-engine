package upgrade

import (
	"fmt"
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
// file on disk. Injectable so tests can exercise verified, unavailable, and
// failed provenance paths without requiring network/GitHub CLI auth.
var attestationVerifier = defaultVerifyAttestation

// ghAttestationVerify runs `gh attestation verify` for assetPath. found is false
// when the gh CLI is not installed. Injectable so tests never execute gh.
var ghAttestationVerify = func(assetPath string) (output string, found bool, err error) {
	ghPath, lerr := exec.LookPath("gh")
	if lerr != nil {
		return "", false, nil
	}
	out, cerr := exec.Command(ghPath, "attestation", "verify", assetPath,
		"--repo", attestationRepo, "--signer-workflow", signerWorkflow).CombinedOutput()
	return string(out), true, cerr
}

// defaultVerifyAttestation is the only source of StatusVerified: a successful
// `gh attestation verify`, pinned to the repository and to the release workflow
// that signs the assets. Files served by the release itself (for example an
// `.intoto.jsonl` next to the asset) are never trusted, because whoever can
// replace the asset can replace those files too.
//
// Outcomes:
//   - gh missing, gh not logged in, "no attestations found", or a base URL that
//     is not github.com: StatusUnavailable. The upgrade proceeds on the checksum
//     only, unless --require-attestation is set.
//   - Any other gh failure: StatusFailed with an error. The upgrade always stops.
//   - gh exits 0: StatusVerified.
func defaultVerifyAttestation(baseURL, assetPath string) (VerificationStatus, string, error) {
	if !strings.HasPrefix(baseURL, "https://github.com/") {
		return StatusUnavailable, "no provenance attestation available for this release", nil
	}
	out, found, err := ghAttestationVerify(assetPath)
	if !found {
		return StatusUnavailable, "gh CLI not found; cannot verify build provenance", nil
	}
	if err != nil {
		msg := strings.TrimSpace(out)
		if ghOutputMeansUnavailable(msg) {
			return StatusUnavailable, msg, nil
		}
		return StatusFailed, msg, fmt.Errorf("gh attestation verify failed: %s", msg)
	}
	return StatusVerified, "verified via gh attestation verify", nil
}

// ghOutputMeansUnavailable reports whether gh output shows that no attestation
// could be checked (no attestations published, or gh is not logged in). It
// compares only the first lines of the trimmed output, ignoring case. Every
// other message counts as a real verification failure.
func ghOutputMeansUnavailable(msg string) bool {
	lines := strings.Split(strings.TrimSpace(msg), "\n")
	if len(lines) > 3 {
		lines = lines[:3]
	}
	for _, line := range lines {
		line = strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(line, "no attestations found") || strings.Contains(line, "not logged into") {
			return true
		}
	}
	return false
}
