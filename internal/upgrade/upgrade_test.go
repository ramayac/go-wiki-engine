package upgrade

import (
	"testing"
)

func TestModuleConstant(t *testing.T) {
	if module == "" {
		t.Error("module constant is empty")
	}
	if module != "github.com/ramayac/go-wiki-engine/cmd/wiki-engine@latest" {
		t.Errorf("unexpected module: %s", module)
	}
}
