package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckBranchCitationsAndReview(t *testing.T) {
	root := gitFixtureWiki(t)
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Add source files and a wiki page citing them on main before branching.
	runGit("checkout", "-q", "main")
	if err := os.MkdirAll(filepath.Join(root, "pkg", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "auth", "token.go"), []byte("package auth\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pkg", "legacy.go"), []byte("package pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "wiki", "auth.md"), []byte(`---
id: auth
type: entity
lifecycle: current
description: Auth service
sources:
  - file: pkg/auth/
  - file: pkg/legacy.go
  - file: remote/file.go
    repo: other-repo
---
# Auth
`), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".")
	runGit("commit", "-q", "-m", "add auth source and page")
	runGit("checkout", "-q", "-B", "feature")

	eng := newTestEngine(root)

	// 1. Modify a file inside the cited directory (uncommitted working-tree edit):
	// should produce a review recommendation with exit-0 semantics (no breakage).
	if err := os.WriteFile(filepath.Join(root, "pkg", "auth", "token.go"), []byte("package auth // updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := eng.Check(CheckOptions{Base: "main", IncludeWorkingTree: true})
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if res.HasBreakage() {
		t.Errorf("expected no breakage for modified source, got %+v", res.Broken)
	}
	if len(res.PagesToReview) != 1 || res.PagesToReview[0].Page != "wiki/auth.md" {
		t.Errorf("expected wiki/auth.md in PagesToReview, got %+v", res.PagesToReview)
	}

	// 2. Delete a cited file: must surface as Broken (exit 1 semantics).
	runGit("rm", "pkg/legacy.go")
	runGit("commit", "-q", "-m", "delete legacy.go")
	res, err = eng.Check(CheckOptions{Base: "main", IncludeWorkingTree: true})
	if err != nil {
		t.Fatalf("Check after delete failed: %v", err)
	}
	if !res.HasBreakage() {
		t.Fatalf("expected breakage after deleting cited file, got %+v", res)
	}
	foundDelete := false
	for _, b := range res.Broken {
		if b.Kind == "deleted_source" && b.Target == "pkg/legacy.go" {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Errorf("expected deleted_source finding for pkg/legacy.go, got %+v", res.Broken)
	}

	// 3. Outside git repo must return ErrCheckSetup (exit 2 semantics).
	nonGit := setupWiki(t)
	_, err = newTestEngine(nonGit).Check(CheckOptions{IncludeWorkingTree: true})
	if !errors.Is(err, ErrCheckSetup) {
		t.Errorf("Check outside git = %v, want ErrCheckSetup", err)
	}
}

func TestSearchRankedAndLifecycleFiltering(t *testing.T) {
	root := setupWiki(t)
	writeTestWiki(t, root, map[string]string{
		"wiki/current-auth.md": `---
id: current-auth
title: Authentication Guide
type: concept
lifecycle: current
tags: [auth, security]
description: Current auth
---
# Authentication Guide

Token validation details.
`,
		"wiki/legacy-auth.md": `---
id: legacy-auth
title: Authentication Guide Legacy
type: concept
lifecycle: legacy
tags: [auth]
description: Legacy auth
---
# Authentication Guide Legacy

Token validation old details.
`,
		"wiki/planned-auth.md": `---
id: planned-auth
title: Planned Auth
type: concept
lifecycle: planned
tags: [auth]
description: Future auth
---
# Planned Auth

Future token validation.
`,
	})
	eng := newTestEngine(root)

	// Default search excludes legacy and planned.
	res, err := eng.SearchRanked("token validation", SearchOptions{})
	if err != nil {
		t.Fatalf("SearchRanked failed: %v", err)
	}
	for _, r := range res {
		if r.Lifecycle == "legacy" || r.Lifecycle == "planned" {
			t.Errorf("default search must exclude %s page: %+v", r.Lifecycle, r)
		}
	}
	if len(res) == 0 || res[0].ID != "current-auth" {
		t.Errorf("expected current-auth as top match, got %+v", res)
	}

	// --include-history includes legacy and planned, with current ranked highest.
	resAll, err := eng.SearchRanked("token validation", SearchOptions{IncludeHistory: true})
	if err != nil {
		t.Fatalf("SearchRanked include-history failed: %v", err)
	}
	if len(resAll) < 3 {
		t.Errorf("expected >=3 results with IncludeHistory, got %d", len(resAll))
	}
	if resAll[0].ID != "current-auth" {
		t.Errorf("expected current-auth to outrank historical pages, got %s", resAll[0].ID)
	}

	// Filter by tag and type.
	resTag, err := eng.SearchRanked("token", SearchOptions{Tag: "security", Type: "concept"})
	if err != nil {
		t.Fatalf("SearchRanked tag filter failed: %v", err)
	}
	if len(resTag) != 1 || resTag[0].ID != "current-auth" {
		t.Errorf("tag+type filter = %+v, want only current-auth", resTag)
	}
}

func TestContextBoundedTruncation(t *testing.T) {
	root := setupWiki(t)
	eng := newTestEngine(root)

	cr, err := eng.ContextWithOptions(ContextOptions{
		Summarize: true,
		MaxPages:  2,
		MaxBytes:  65536,
	})
	if err != nil {
		t.Fatalf("ContextWithOptions failed: %v", err)
	}
	if !cr.Truncated || len(cr.Catalog) != 2 || cr.TotalCatalog <= 2 {
		t.Errorf("expected truncation at 2 pages, got truncated=%v len=%d total=%d", cr.Truncated, len(cr.Catalog), cr.TotalCatalog)
	}
	if !strings.Contains(cr.TruncatedReason, "max_pages=2") {
		t.Errorf("unexpected TruncatedReason: %q", cr.TruncatedReason)
	}
}

func TestGraphSourceNodesAndIDLookup(t *testing.T) {
	root := t.TempDir()
	writeTestWiki(t, root, map[string]string{
		"main.go": "package main\n",
		"wiki/index.md": `---
id: index
lifecycle: current
description: Index
---
# Index
- [storage](entities/storage.md) | Storage
- [api](entities/api.md) | API
`,
		"wiki/entities/storage.md": `---
id: storage
type: entity
lifecycle: current
description: Storage
sources:
  - file: main.go
---
# Storage
`,
		"wiki/entities/api.md": `---
id: api
type: entity
lifecycle: current
description: API
depends_on: [storage]
---
# API
`,
	})
	eng := newTestEngine(root)

	view, err := eng.BuildGraphViewWithOptions(GraphOptions{IncludeSources: true})
	if err != nil {
		t.Fatalf("BuildGraphViewWithOptions failed: %v", err)
	}
	if view.SchemaVersion != "2" {
		t.Errorf("SchemaVersion = %q, want 2", view.SchemaVersion)
	}
	hasSourceNode := false
	hasDepEdge := false
	for _, n := range view.Nodes {
		if n.Kind == "source" && n.File == "main.go" {
			hasSourceNode = true
		}
	}
	for _, e := range view.Edges {
		if e.From == "entities/api.md" && e.To == "entities/storage.md" && e.Kind == "depends_on" {
			hasDepEdge = true
		}
	}
	if !hasSourceNode {
		t.Errorf("expected source node main.go when IncludeSources=true")
	}
	if !hasDepEdge {
		t.Errorf("expected depends_on edge from api to storage")
	}

	// Lookup neighborhood by stable page ID.
	nv, err := Neighborhood(view.Nodes, view.Edges, "storage")
	if err != nil {
		t.Fatalf("Neighborhood by ID failed: %v", err)
	}
	if nv.Node.File != "entities/storage.md" {
		t.Errorf("Neighborhood(storage) resolved to %s", nv.Node.File)
	}
}

func TestExtractLinksContainment(t *testing.T) {
	root := t.TempDir()
	wikiDir := filepath.Join(root, "wiki")
	subDir := filepath.Join(wikiDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `
- [ok](../index.md)
- [escape](../../outside.md)
- [encoded](%2e%2e/%2e%2e/outside.md)
- [abs](/etc/passwd.md)
`
	links := ExtractLinks(content, subDir, wikiDir)
	if len(links) != 1 || links[0] != "index.md" {
		t.Errorf("ExtractLinks = %v, want only [index.md]", links)
	}
}

func BenchmarkSearchRanked100(b *testing.B)  { benchmarkSearchRanked(b, 100) }
func BenchmarkSearchRanked1000(b *testing.B) { benchmarkSearchRanked(b, 1000) }

func benchmarkSearchRanked(b *testing.B, count int) {
	root := b.TempDir()
	wikiDir := filepath.Join(root, "wiki")
	if err := os.MkdirAll(wikiDir, 0o755); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < count; i++ {
		content := fmt.Sprintf("---\nid: page-%d\ntitle: Service %d\ntype: concept\nlifecycle: current\ntags: [svc, topic-%d]\ndescription: Page %d\n---\n# Service %d\n\nDetailed architecture documentation about token validation and graph traversal for service %d.\n", i, i, i%10, i, i, i)
		if err := os.WriteFile(filepath.Join(wikiDir, fmt.Sprintf("page-%d.md", i)), []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	eng := newTestEngine(root)
	snap, err := eng.LoadSnapshot()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = SearchRankedFromSnapshot(snap, "token validation graph", SearchOptions{Limit: 10})
	}
}
