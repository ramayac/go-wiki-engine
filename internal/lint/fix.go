package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/model"
	"github.com/ramayac/go-wiki-engine/internal/schema"
)

// FileFixReport records mechanical fixes applied to a single wiki file.
type FileFixReport struct {
	File  string   `json:"file"`
	Count int      `json:"count"`
	Fixes []string `json:"fixes"`
}

// FixResult aggregates all mechanical fixes applied by `lint --fix`.
type FixResult struct {
	Files      []FileFixReport `json:"files"`
	TotalFixes int             `json:"total_fixes"`
}

// Fix applies idempotent mechanical fixes to the wiki in deterministic order:
// 1. Convert old frontmatter fields (status -> lifecycle, add id/title).
// 2. Convert resolvable [[wiki links]] to standard relative Markdown links.
// 3. Add lifecycle banners to non-current pages.
// 4. Add index badges to non-current entries in index.md.
// 5. Reorder index entries within each section by lifecycle priority.
// 6. Convert supported inline references to structured sources & references.
func Fix(repoRoot, wikiDir string) (*FixResult, error) {
	snap, err := model.LoadSnapshot(repoRoot, wikiDir)
	if err != nil {
		return nil, err
	}
	res := &FixResult{Files: []FileFixReport{}}

	// Build unambiguous lookup map by ID / stem / title for [[wiki links]].
	stemMatches := make(map[string][]*model.Page)
	for _, p := range snap.Pages {
		key := strings.ToLower(p.ID)
		stemMatches[key] = append(stemMatches[key], p)
		stem := strings.ToLower(model.ExpectedID(p.WikiRel))
		if stem != key {
			stemMatches[stem] = append(stemMatches[stem], p)
		}
	}

	// Pass 1, 2, 3, 6: per-page frontmatter, wiki links, lifecycle banners, structured references.
	for _, p := range snap.Pages {
		if p.FMErr != nil {
			continue // never rewrite malformed YAML automatically
		}
		var fixes []string
		meta := p.Metadata
		body := p.Body
		fmChanged := false
		bodyChanged := false

		// 1. Convert old frontmatter fields & add missing id/title.
		if p.FMState == model.FrontMatterPresent {
			if meta.Lifecycle == "" && meta.LegacyStatus {
				if mapped, ok := schema.MapLegacyStatus(meta.Status); ok {
					meta.Lifecycle = mapped
					meta.Status = ""
					meta.LegacyStatus = false
					fmChanged = true
					fixes = append(fixes, fmt.Sprintf("migrated legacy status to lifecycle: %s", mapped))
				}
			}
			expectedID := model.ExpectedID(p.WikiRel)
			if meta.ID == "" {
				meta.ID = expectedID
				fmChanged = true
				fixes = append(fixes, fmt.Sprintf("added id: %s", expectedID))
			}
			if meta.Title == "" {
				if t := p.EffectiveTitle(); t != "" && t != meta.ID {
					meta.Title = t
					fmChanged = true
					fixes = append(fixes, "added title from H1 heading")
				}
			}

			// 6. Convert supported inline references to structured sources & references.
			if meta.LegacyRefs && len(meta.References) > 0 {
				var remainingRefs []model.Reference
				existingSources := make(map[model.SourceReference]bool)
				for _, s := range meta.Sources {
					existingSources[s] = true
				}
				converted := 0
				for _, r := range meta.References {
					switch r.Type {
					case "source":
						sr := model.SourceReference{File: r.Value}
						if !existingSources[sr] {
							meta.Sources = append(meta.Sources, sr)
							existingSources[sr] = true
						}
						converted++
					case "external", "issue":
						remainingRefs = append(remainingRefs, r)
						converted++
					default:
						remainingRefs = append(remainingRefs, r)
					}
				}
				if converted > 0 {
					meta.References = remainingRefs
					meta.LegacyRefs = false
					fmChanged = true
					fixes = append(fixes, "converted inline references to structured sources/references")
				}
			}
		}

		// 2. Convert resolvable [[wiki links]] outside code blocks.
		newBody, convertedLinks := convertWikiLinks(body, p, stemMatches)
		if convertedLinks > 0 {
			body = newBody
			bodyChanged = true
			fixes = append(fixes, fmt.Sprintf("converted %d wiki link(s) to standard markdown", convertedLinks))
		}

		// 3. Add lifecycle banner to non-current pages (skipping index.md).
		lc := meta.EffectiveLifecycle()
		if p.WikiRel != "index.md" && lc != "" && lc != "current" {
			banner := schema.LifecycleBanner(lc, meta.SupersededBy)
			if banner != "" && !strings.Contains(body, "**Lifecycle:") {
				body = insertBannerAfterH1(body, banner)
				bodyChanged = true
				fixes = append(fixes, fmt.Sprintf("added lifecycle banner (%s)", lc))
			}
		}

		if fmChanged || bodyChanged {
			var finalContent string
			if p.FMState == model.FrontMatterPresent {
				finalContent = model.SerializeFrontMatter(meta) + body
			} else {
				finalContent = body
			}
			if finalContent != p.RawContent {
				if err := os.WriteFile(p.AbsPath, []byte(finalContent), 0o644); err != nil {
					return nil, fmt.Errorf("write %s: %w", p.Path, err)
				}
				res.Files = append(res.Files, FileFixReport{
					File:  p.Path,
					Count: len(fixes),
					Fixes: fixes,
				})
				res.TotalFixes += len(fixes)
			}
		}
	}

	// Reload snapshot so index.md badges and ordering reflect any lifecycle migrations.
	snap2, err := model.LoadSnapshot(repoRoot, wikiDir)
	if err != nil {
		return nil, err
	}

	// 4 & 5. Add index badges for non-current pages and reorder entries within each section by lifecycle.
	if snap2.Index != nil {
		updatedIndex, idxFixes := fixIndexPage(snap2)
		if len(idxFixes) > 0 && updatedIndex != snap2.Index.RawContent {
			if err := os.WriteFile(snap2.Index.AbsPath, []byte(updatedIndex), 0o644); err != nil {
				return nil, fmt.Errorf("write %s: %w", snap2.Index.Path, err)
			}
			merged := false
			for i := range res.Files {
				if res.Files[i].File == snap2.Index.Path {
					res.Files[i].Count += len(idxFixes)
					res.Files[i].Fixes = append(res.Files[i].Fixes, idxFixes...)
					merged = true
					break
				}
			}
			if !merged {
				res.Files = append(res.Files, FileFixReport{
					File:  snap2.Index.Path,
					Count: len(idxFixes),
					Fixes: idxFixes,
				})
			}
			res.TotalFixes += len(idxFixes)
		}
	}

	return res, nil
}

func convertWikiLinks(body string, page *model.Page, stemMatches map[string][]*model.Page) (string, int) {
	lines := strings.Split(body, "\n")
	inCode := false
	count := 0
	pageDir := filepath.Dir(page.WikiRel)

	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		newLine := wikiLinkRe.ReplaceAllStringFunc(line, func(match string) string {
			inner := strings.TrimSpace(match[2 : len(match)-2])
			// Support [[target|label]] syntax.
			targetName, label := inner, inner
			if parts := strings.SplitN(inner, "|", 2); len(parts) == 2 {
				targetName = strings.TrimSpace(parts[0])
				label = strings.TrimSpace(parts[1])
			}
			key := strings.ToLower(strings.TrimSuffix(targetName, ".md"))
			cands := stemMatches[key]
			if len(cands) != 1 {
				return match // ambiguous or unresolved — leave untouched
			}
			target := cands[0]
			rel, err := filepath.Rel(pageDir, target.WikiRel)
			if err != nil {
				return match
			}
			rel = filepath.ToSlash(rel)
			if label == targetName {
				label = target.EffectiveTitle()
			}
			count++
			return fmt.Sprintf("[%s](%s)", label, rel)
		})
		lines[i] = newLine
	}
	return strings.Join(lines, "\n"), count
}

func insertBannerAfterH1(body, banner string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "# ") {
			out := make([]string, 0, len(lines)+3)
			out = append(out, lines[:i+1]...)
			out = append(out, "", banner)
			rest := lines[i+1:]
			if len(rest) > 0 && strings.TrimSpace(rest[0]) != "" {
				out = append(out, "")
			}
			out = append(out, rest...)
			return strings.Join(out, "\n")
		}
	}
	return banner + "\n\n" + body
}

var (
	indexEntryRe  = regexp.MustCompile(`^(\s*[-*]\s+\[([^\]]+)\]\(([^)]+)\))(\s*\|\s*(.*))?$`)
	badgePrefixRe = regexp.MustCompile(`^\[(planned|wip|implemented|legacy|superseded)\]\s*`)
)

func fixIndexPage(snap *model.WikiSnapshot) (string, []string) {
	lines := strings.Split(snap.Index.RawContent, "\n")
	var fixes []string
	badgesAdded := 0

	// Step 4: Add lifecycle badges to non-current entries.
	for i, line := range lines {
		m := indexEntryRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		target := strings.TrimSpace(m[3])
		if idx := strings.Index(target, "#"); idx != -1 {
			target = target[:idx]
		}
		targetClean := filepath.ToSlash(filepath.Clean(target))
		p, ok := snap.ByPath[targetClean]
		if !ok {
			continue
		}
		lc := p.EffectiveLifecycle()
		if lc == "" || lc == "current" {
			continue
		}
		badge := "[" + lc + "]"
		desc := strings.TrimSpace(m[5])
		if strings.HasPrefix(desc, badge) {
			continue
		}
		// Replace an outdated badge if present, else prepend badge.
		desc = badgePrefixRe.ReplaceAllString(desc, "")
		if desc == "" {
			lines[i] = fmt.Sprintf("%s | %s", m[1], badge)
		} else {
			lines[i] = fmt.Sprintf("%s | %s %s", m[1], badge, desc)
		}
		badgesAdded++
	}
	if badgesAdded > 0 {
		fixes = append(fixes, fmt.Sprintf("added/updated %d lifecycle badge(s) in index.md", badgesAdded))
	}

	// Step 5: Reorder contiguous bullet-entry blocks within each section by lifecycle priority.
	reordered := 0
	for i := 0; i < len(lines); {
		if !indexEntryRe.MatchString(lines[i]) {
			i++
			continue
		}
		start := i
		for i < len(lines) && indexEntryRe.MatchString(lines[i]) {
			i++
		}
		end := i
		if end-start <= 1 {
			continue
		}
		block := append([]string(nil), lines[start:end]...)
		sorted := append([]string(nil), block...)
		sort.SliceStable(sorted, func(a, b int) bool {
			return indexLinePriority(snap, sorted[a]) < indexLinePriority(snap, sorted[b])
		})
		changed := false
		for k := range block {
			if block[k] != sorted[k] {
				changed = true
				break
			}
		}
		if changed {
			copy(lines[start:end], sorted)
			reordered++
		}
	}
	if reordered > 0 {
		fixes = append(fixes, fmt.Sprintf("reordered %d index section(s) by lifecycle", reordered))
	}

	return strings.Join(lines, "\n"), fixes
}

func indexLinePriority(snap *model.WikiSnapshot, line string) int {
	m := indexEntryRe.FindStringSubmatch(line)
	if m == nil {
		return 99
	}
	target := strings.TrimSpace(m[3])
	if idx := strings.Index(target, "#"); idx != -1 {
		target = target[:idx]
	}
	if p, ok := snap.ByPath[filepath.ToSlash(filepath.Clean(target))]; ok {
		return schema.LifecyclePriority(p.EffectiveLifecycle())
	}
	return 99
}
