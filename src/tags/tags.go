package tags

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type Tag struct {
	ID      string   `json:"id" dynamodbav:"id"`
	Name    string   `json:"name" dynamodbav:"name"`
	Path    []string `json:"path,omitempty" dynamodbav:"path,omitempty"`
	Sources []string `json:"-" dynamodbav:"sources"`
}

func Normalize(name string) string {
	return cases.Fold().String(norm.NFC.String(strings.Join(strings.Fields(name), " ")))
}

// ReplaceSource changes one producer's assignments without discarding other producers.
func ReplaceSource(existing []Tag, names []string, source string) []Tag {
	paths := make([][]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, []string{name})
	}

	return ReplaceSourcePaths(existing, paths, source)
}

// ReplaceSourcePaths identifies nested tags by their full path, regardless of producer.
func ReplaceSourcePaths(existing []Tag, paths [][]string, source string) []Tag {
	byID := make(map[string]Tag)
	for _, tag := range existing {
		tag.Sources = slices.DeleteFunc(slices.Clone(tag.Sources), func(value string) bool { return value == source })
		if len(tag.Sources) > 0 {
			byID[tag.ID] = tag
		}
	}

	for _, path := range paths {
		display := make([]string, 0, len(path))
		normalized := make([]string, 0, len(path))
		for _, part := range path {
			part = strings.Join(strings.Fields(part), " ")
			if part != "" {
				display = append(display, part)
				normalized = append(normalized, Normalize(part))
			}
		}

		if len(display) == 0 {
			continue
		}

		key := normalized[0]
		if len(normalized) > 1 {
			encoded, _ := json.Marshal(normalized)
			key = string(encoded)
		}

		digest := sha256.Sum256([]byte(key))
		id := hex.EncodeToString(digest[:])
		tag, found := byID[id]
		if !found {
			tag = Tag{ID: id, Name: display[len(display)-1]}
			if len(display) > 1 {
				tag.Path = display
			}
		}

		if !slices.Contains(tag.Sources, source) {
			tag.Sources = append(tag.Sources, source)
			slices.Sort(tag.Sources)
		}

		byID[id] = tag
	}

	result := make([]Tag, 0, len(byID))
	for _, tag := range byID {
		result = append(result, tag)
	}

	slices.SortFunc(result, func(a, b Tag) int { return strings.Compare(a.ID, b.ID) })
	return result
}
