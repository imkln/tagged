package tags

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

type Tag struct {
	ID      string   `json:"id" dynamodbav:"id"`
	Name    string   `json:"name" dynamodbav:"name"`
	Sources []string `json:"-" dynamodbav:"sources"`
}

func Normalize(name string) string {
	return cases.Fold().String(norm.NFC.String(strings.Join(strings.Fields(name), " ")))
}

// ReplaceSource changes one producer's assignments without discarding other producers.
func ReplaceSource(existing []Tag, names []string, source string) []Tag {
	byID := make(map[string]Tag)
	for _, tag := range existing {
		tag.Sources = slices.DeleteFunc(slices.Clone(tag.Sources), func(value string) bool { return value == source })
		if len(tag.Sources) > 0 {
			byID[tag.ID] = tag
		}
	}

	for _, name := range names {
		name = strings.Join(strings.Fields(name), " ")
		key := Normalize(name)
		if key == "" {
			continue
		}

		digest := sha256.Sum256([]byte(key))
		id := hex.EncodeToString(digest[:])
		tag, found := byID[id]
		if !found {
			tag = Tag{ID: id, Name: name}
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
