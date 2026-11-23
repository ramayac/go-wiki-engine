package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ramayac/go-wiki-engine/internal/config"
	"github.com/ramayac/go-wiki-engine/internal/model"
)

func setupLintWiki(t *testing.T, extra map[string]string) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"main.go": "package main\n",
		"wiki/README.md": `---
id: README
title: Wiki
type: concept
lifecycle: current
description: README
sources:
  - file: main.go
---
# Wiki

See [index](index.md).
`,
		"wiki/index.md": `---
id: index
title: Wiki Index
lifecycle: current
description: Index
---
# Index

- [README.md](README.md) | Overview
- [prologue/schema.md](prologue/schema.md) | Schema
- [prologue/log.md](prologue/log.md) | Log
- [prologue/repo-map.md](prologue/repo-map.md) | Repo Map
- [prologue/phases.md](prologue/phases.md) | Phases
- [operations/ingest.md](operations/ingest.md) | Ingest
- [operations/query.md](operations/query.md) | Query
- [operations/lint.md](operations/lint.md) | Lint
`,
		"wiki/prologue/log.md": `---
id: log
lifecycle: current
description: Log
---
# Log

## [2026-04-16] ingest | init

- Initialized.
`,
		"wiki/prologue/schema.md": `---
id: schema
type: concept
lifecycle: current
description: Schema
sources:
  - file: main.go
---
# Schema

See [index](../index.md).
`,
		"wiki/prologue/phases.md": `---
id: phases
lifecycle: current
description: Phases
---
# Phases

See [index](../index.md).
`,
		"wiki/prologue/repo-map.md": `---
id: repo-map
lifecycle: current
description: Repo Map
---
# Repo Map

See [index](../index.md).
`,
		"wiki/operations/ingest.md": `---
id: ingest
type: runbook
lifecycle: current
description: Ingest
sources:
  - file: main.go
---
# Ingest

See [index](../index.md).
`,
		"wiki/operations/query.md": `---
id: query
type: runbook
lifecycle: current
description: Query
sources:
  - file: main.go
---
# Query

See [index](../index.md).
`,
		"wiki/operations/lint.md": `---
id: lint
type: runbook
lifecycle: current
description: Lint
sources:
  - file: main.go
---
# Lint

See [index](../index.md).
`,
	}
	for k, v := range extra {
		files[k] = v
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSingleReadSnapshotLint(t *testing.T) {
	root := setupLintWiki(t, nil)
	cfg := config.DefaultConfig()
	snap, err := model.LoadSnapshot(root, cfg.WikiDir)
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}
	res := Run(snap, cfg, nil, nil)
	if !res.OK {
		t.Fatalf("expected clean lint, got: %v", res.Messages)
	}
	for path, count := range snap.ReadCounts {
		if count != 1 {
			t.Errorf("file %s read %d times during lint, want exactly 1", path, count)
		}
	}
}

func TestSelectorsAndGroups(t *testing.T) {
	if err := ValidateSelectors([]string{"schema", "links", "security"}, nil); err != nil {
		t.Errorf("valid group selectors failed: %v", err)
	}
	if err := ValidateSelectors(nil, []string{"all"}); err == nil {
		t.Error("--skip=all must be rejected")
	}
	if err := ValidateSelectors([]string{"nonexistent"}, nil); err == nil {
		t.Error("unknown checker must be rejected")
	}
}

func TestTypedModelAndRelationshipValidation(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/entities/storage.md": `---
id: storage
title: Storage
type: entity
lifecycle: current
description: Storage layer
---
# Storage

See [index](../index.md).
`,
		"wiki/entities/api.md": `---
id: api
title: API
type: entity
lifecycle: current
description: API layer
depends_on: [storage, schema]
related: [does-not-exist]
---
# API

See [index](../index.md).
`,
		"wiki/decisions/bad-decision.md": `---
id: wrong-id
type: decision
lifecycle: current
description: Decision missing workflow status
---
# Bad Decision

See [index](../index.md).
`,
		"wiki/incidents/bad-incident.md": `---
id: bad-incident
type: incident
lifecycle: current
description: Incident missing severity and status
---
# Bad Incident

See [index](../index.md).
`,
	})
	cfg := config.DefaultConfig()
	snap, err := model.LoadSnapshot(root, cfg.WikiDir)
	if err != nil {
		t.Fatal(err)
	}
	res := Run(snap, cfg, []string{"schema", "graph"}, nil)
	if res.OK {
		t.Fatal("expected lint failures for invalid types, IDs, and relationships")
	}
	joined := strings.Join(res.Messages, "\n")
	for _, want := range []string{
		"does not match filename stem",
		"decision page requires status",
		"incident page requires severity",
		"incident page requires status",
		`depends_on target "schema" must be an entity page`,
		"related page ID not found: does-not-exist",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("lint output missing %q:\n%s", want, joined)
		}
	}
}

func TestLintFixIdempotent(t *testing.T) {
	root := setupLintWiki(t, map[string]string{
		"wiki/index.md": `---
status: current
description: Index
---
# Index

## Docs

- [old.md](old.md) | Old doc
- [prologue/schema.md](prologue/schema.md) | Schema
- [README.md](README.md) | Overview
- [prologue/log.md](prologue/log.md) | Log
- [prologue/repo-map.md](prologue/repo-map.md) | Repo Map
- [prologue/phases.md](prologue/phases.md) | Phases
- [operations/ingest.md](operations/ingest.md) | Ingest
- [operations/query.md](operations/query.md) | Query
- [operations/lint.md](operations/lint.md) | Lint
`,
		"wiki/old.md": `---
status: legacy
description: Legacy doc
references: [source:main.go, external:https://example.com]
---
# Old Doc

See [[schema]] for details.
`,
	})

	first, err := Fix(root, "wiki")
	if err != nil {
		t.Fatalf("first Fix failed: %v", err)
	}
	if first.TotalFixes == 0 {
		t.Fatal("expected first Fix to apply mechanical fixes")
	}

	oldBytes, err := os.ReadFile(filepath.Join(root, "wiki", "old.md"))
	if err != nil {
		t.Fatal(err)
	}
	oldText := string(oldBytes)
	for _, want := range []string{
		"id: old",
		"lifecycle: legacy",
		"sources:",
		"file: main.go",
		"[Schema](prologue/schema.md)",
		"**Lifecycle: legacy**",
	} {
		if !strings.Contains(oldText, want) {
			t.Errorf("fixed old.md missing %q:\n%s", want, oldText)
		}
	}

	idxBytes, err := os.ReadFile(filepath.Join(root, "wiki", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	idxText := string(idxBytes)
	if !strings.Contains(idxText, "[legacy] Old doc") {
		t.Errorf("index.md missing lifecycle badge:\n%s", idxText)
	}
	// Current entries must appear before legacy entries in the reordered section.
	if strings.Index(idxText, "prologue/schema.md") > strings.Index(idxText, "old.md") {
		t.Errorf("index.md did not reorder current entries before legacy:\n%s", idxText)
	}

	// Second Fix run must be completely idempotent (0 changes).
	second, err := Fix(root, "wiki")
	if err != nil {
		t.Fatalf("second Fix failed: %v", err)
	}
	if second.TotalFixes != 0 {
		t.Errorf("second Fix was not idempotent: %+v", second)
	}
}
