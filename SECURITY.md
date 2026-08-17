# Security Policy

## Supported Versions

Only the latest release receives security fixes. Legacy pre-1.0 tags
(`0.1.0`, `v.0.5.0`) are unsupported.

## Reporting a Vulnerability

Please report vulnerabilities privately via the GitHub Security Advisory form:

[Report a vulnerability](https://github.com/ramayac/go-wiki-engine/security/advisories/new)

Do not open a public issue. You will get a response as soon as possible.

## Design Notes

- Zero third-party runtime dependencies — Go standard library only.
- `wiki-engine upgrade` downloads over HTTPS and verifies the SHA-256
  checksum from `checksums.txt` before replacing the binary.
- Wiki content is only ever read as plain Markdown — nothing from the wiki
  is executed.
- Engine read paths (e.g. `summary`) guard against path traversal outside
  `wiki_dir`.
