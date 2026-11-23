package model

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/ramayac/go-wiki-engine/internal/paths"
	"gopkg.in/yaml.v3"
)

// VocabularyFile is the name of the controlled tag vocabulary inside the wiki directory.
const VocabularyFile = "vocabulary.yaml"

// Vocabulary is the set of tags a wiki allows. An empty vocabulary means tags
// are free-form.
type Vocabulary struct {
	tags map[string]struct{}
}

// Empty reports whether the vocabulary lists no tags (free-form tags).
func (v Vocabulary) Empty() bool { return len(v.tags) == 0 }

// Has reports whether tag is listed.
func (v Vocabulary) Has(tag string) bool {
	_, ok := v.tags[tag]
	return ok
}

// Tags returns the listed tags in sorted order.
func (v Vocabulary) Tags() []string {
	out := make([]string, 0, len(v.tags))
	for t := range v.tags {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Suggest returns a listed tag that differs from tag only by case or by "-"
// versus "_", or "" when there is none.
func (v Vocabulary) Suggest(tag string) string {
	want := normalizeTag(tag)
	for _, t := range v.Tags() {
		if normalizeTag(t) == want {
			return t
		}
	}
	return ""
}

func normalizeTag(tag string) string {
	return strings.ReplaceAll(strings.ToLower(tag), "_", "-")
}

// LoadVocabulary reads vocabulary.yaml from wikiRoot (the absolute wiki
// directory). A missing, empty, or comments-only file gives an empty
// (free-form) vocabulary. Malformed YAML, a wrong structure, non-string values,
// or a file that resolves outside wikiRoot gives an error and an empty
// vocabulary.
//
// The file is a mapping of category names to lists of tags, or a bare list.
// Category names are for humans; the vocabulary is the union of all tags.
func LoadVocabulary(wikiRoot string) (Vocabulary, error) {
	abs, err := paths.ResolveWithin(wikiRoot, VocabularyFile)
	if err != nil {
		return Vocabulary{}, fmt.Errorf("%s: %w", VocabularyFile, err)
	}
	data, err := os.ReadFile(abs)
	if os.IsNotExist(err) {
		return Vocabulary{}, nil
	}
	if err != nil {
		return Vocabulary{}, fmt.Errorf("%s: %w", VocabularyFile, err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return Vocabulary{}, nil // empty or comments only
		}
		return Vocabulary{}, fmt.Errorf("%s: %w", VocabularyFile, err)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Vocabulary{}, fmt.Errorf("%s: only one YAML document is allowed", VocabularyFile)
	}

	root := &doc
	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		root = root.Content[0]
	}

	set := make(map[string]struct{})
	add := func(field string, n *yaml.Node) error {
		list, err := decodeStringList(field, n, n.Line)
		if err != nil {
			var fe *FrontMatterError
			if errors.As(err, &fe) {
				return fmt.Errorf("%s: line %d: %s", VocabularyFile, fe.Line, fe.Message)
			}
			return fmt.Errorf("%s: %w", VocabularyFile, err)
		}
		for _, t := range list {
			set[t] = struct{}{}
		}
		return nil
	}

	switch {
	case root.Tag == "!!null":
		// Document holds only null: free-form.
	case root.Kind == yaml.SequenceNode:
		if err := add("vocabulary", root); err != nil {
			return Vocabulary{}, err
		}
	case root.Kind == yaml.MappingNode:
		for i := 0; i+1 < len(root.Content); i += 2 {
			key, val := root.Content[i], root.Content[i+1]
			if val.Tag != "!!null" && val.Kind != yaml.SequenceNode {
				return Vocabulary{}, fmt.Errorf("%s: line %d: category %q must be a list of tags", VocabularyFile, key.Line, key.Value)
			}
			if err := add(key.Value, val); err != nil {
				return Vocabulary{}, err
			}
		}
	default:
		return Vocabulary{}, fmt.Errorf("%s: line %d: must be a list of tags or a mapping of category names to lists of tags", VocabularyFile, root.Line)
	}
	return Vocabulary{tags: set}, nil
}
