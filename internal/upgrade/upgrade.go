// Package upgrade implements self-upgrade via `go install`.
package upgrade

import (
	"fmt"
	"os"
	"os/exec"
)

const module = "github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest"

// Run executes `go install` to upgrade wiki-engine to the latest version.
func Run() error {
	gobin, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go not found in PATH; install Go or download a release binary from GitHub")
	}

	cmd := exec.Command(gobin, "install", module)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	fmt.Fprintf(os.Stderr, "running: go install %s\n", module)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "upgrade complete")
	fmt.Fprintln(os.Stderr, "run `wiki-engine sync-prompts` in each repo to update prompts and instructions")
	return nil
}
