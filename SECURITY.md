# Security Policy

## Supported Versions

Only the latest release receives security fixes. Legacy pre-1.0 tags
(`0.1.0`, `v.0.5.0`) are unsupported.

## Reporting a Vulnerability

Please report vulnerabilities privately via the GitHub Security Advisory form:

[Report a vulnerability](https://github.com/ramayac/go-wiki-engine/security/advisories/new)

Do not open a public issue. You will get a response as soon as possible.

## Security & Supply-Chain Controls

- **Strict path containment (`internal/paths`):** Every command, `.wikirc` load,
  `init`, `sync-prompts`, `summary`, graph traversal, and source resolution
  enforces single-segment `wiki_dir` validation and symlink-aware boundary
  checks so no operation can read or traverse outside the repository or wiki.
- **Minimal audited dependencies:** Runtime uses the Go standard library plus
  `gopkg.in/yaml.v3` for standards-compliant YAML frontmatter parsing. CI runs
  pinned `govulncheck` (`v1.1.4`), `golangci-lint`, and GitHub Dependency Review.
- **Fail-closed self-upgrade (`internal/upgrade`):**
  - Downloads release archives over HTTPS, streaming directly to disk with both
    compressed (100 MiB) and expanded (200 MiB) size caps.
  - Checks the SHA-256 of the asset against `checksums.txt` from the same
    release. This proves integrity, not authenticity: whoever can replace the
    asset can replace `checksums.txt`.
  - Authenticity comes only from `gh attestation verify`, pinned to this
    repository and to the `release.yml` signer workflow. The upgrade trusts no
    provenance file that the release itself serves.
  - Without `gh` (or when `gh` is not logged in, or the release has no
    attestations), the upgrade proceeds on the checksum only.
    `--require-attestation` makes it fail instead. Any other `gh` failure
    always stops the upgrade.
  - Accepts only release tags of the form `vMAJOR.MINOR.PATCH[-pre]`, and
    refuses a tag lower than the running version (downgrade). A `dev` or
    otherwise unparsable running version may upgrade.
  - Rejects archive entries with unsafe paths (`..`, absolute paths, symlinks).
  - Never silently falls back to `go install` on download or verification
    failure; building from source requires explicit `--from-source` (pinned to
    the release tag).
  - Verifies the swapped binary executes `version` cleanly.
- **Immutable CI & release provenance:** Every external GitHub Action in
  `.github/workflows/` is pinned by 40-character commit SHA (enforced by
  `internal/audit`), workflows declare minimal `permissions`, and release assets
  publish per-platform checksums plus SLSA build-provenance attestations.
- **Read-only wiki content:** Wiki files are only ever parsed as plain Markdown
  and YAML metadata — nothing from the wiki is executed.
