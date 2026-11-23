package engine

// WatchResult holds the output of one watch cycle.
type WatchResult struct {
	Changed    []string `json:"changed"`
	Candidates []string `json:"candidates"`
	LintOK     bool     `json:"lint_ok"`
	LintIssues []Issue  `json:"lint_issues,omitempty"`
}

// WatchOnce runs a single watch cycle using the unified Git change pipeline and
// a single-snapshot lint pass.
func (e *Engine) WatchOnce() (*WatchResult, error) {
	cs, err := e.DiscoverChanges(ChangeOptions{
		DiffRange:          e.Cfg.DefaultDiff,
		IncludeWorkingTree: true,
	})
	if err != nil {
		return nil, err
	}

	changed := cs.NonWikiChanged(e.Cfg.WikiDir)
	var candidates []string
	for _, f := range changed {
		if !e.isIgnored(f) {
			candidates = append(candidates, f)
		}
	}

	lintRes := e.Lint()
	return &WatchResult{
		Changed:    changed,
		Candidates: candidates,
		LintOK:     lintRes.OK,
		LintIssues: lintRes.Issues,
	}, nil
}
