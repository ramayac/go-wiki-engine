package engine

import (
	"fmt"

	"github.com/ramayac/go-wiki-engine/internal/lint"
	"github.com/ramayac/go-wiki-engine/internal/model"
)

// Re-export lint severity and issue types so engine callers remain source-compatible.
type Severity = lint.Severity

const (
	SevInfo  = lint.SevInfo
	SevWarn  = lint.SevWarn
	SevError = lint.SevError
)

// Issue aliases lint.Issue.
type Issue = lint.Issue

// LintResult aliases lint.Result.
type LintResult = lint.Result

// FixResult aliases lint.FixResult.
type FixResult = lint.FixResult

// KnownCheckerNames returns all registered individual checker names.
func KnownCheckerNames() []string {
	return lint.KnownCheckerNames()
}

// KnownGroupNames returns all registered checker group names.
func KnownGroupNames() []string {
	return append([]string(nil), lint.GroupNames...)
}

// ValidateLintSelectors validates --check and --skip selectors (accepting both
// individual checker names and group names, and rejecting --skip=all).
func ValidateLintSelectors(check, skip []string) error {
	return lint.ValidateSelectors(check, skip)
}

// Lint runs all registered checkers against a single WikiSnapshot.
func (e *Engine) Lint() LintResult {
	return e.LintWithOptions(nil, nil)
}

// LintWithOptions runs selected checkers against ONE consistent WikiSnapshot
// where every wiki file is read exactly once.
func (e *Engine) LintWithOptions(check []string, skip []string) LintResult {
	snap, err := e.LoadSnapshot()
	if err != nil {
		msg := fmt.Sprintf("wiki directory not found: %s (run wiki-engine init)", e.Cfg.WikiDir)
		if _, pathErr := e.ResolveWikiPath(); pathErr != nil {
			msg = fmt.Sprintf("invalid wiki directory %q: %v", e.Cfg.WikiDir, pathErr)
		}
		return LintResult{
			OK:       false,
			Messages: []string{fmt.Sprintf("%s: [lint] %s", e.Cfg.WikiDir, msg)},
			Issues: []Issue{{
				Severity: SevError,
				Check:    "lint",
				File:     e.Cfg.WikiDir,
				Message:  msg,
			}},
			Reason: msg,
		}
	}
	return e.lintSnapshot(snap, check, skip)
}

func (e *Engine) lintSnapshot(snap *model.WikiSnapshot, check, skip []string) LintResult {
	return lint.Run(snap, e.Cfg, check, skip)
}

// LintFix applies deterministic, idempotent mechanical fixes to the wiki.
func (e *Engine) LintFix() (*FixResult, error) {
	if _, err := e.ResolveWikiPath(); err != nil {
		return nil, err
	}
	return lint.Fix(e.RootDir, e.Cfg.WikiDir)
}
