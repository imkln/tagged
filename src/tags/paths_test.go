package tags

import (
	"reflect"
	"testing"
)

func TestShowsNestedTagPaths(t *testing.T) {
	first := ReplaceSourcePaths(nil, [][]string{{"Animals"}, {"Animals", "Cats"}}, "manual")
	second := ReplaceSourcePaths(nil, [][]string{{"Pets", "Cats"}}, "manual")
	var child Tag

	for _, tag := range first {
		if tag.Name == "Cats" {
			child = tag
		}
	}

	if !reflect.DeepEqual(child.Path, []string{"Animals", "Cats"}) {
		t.Fatalf("tag path = %v, want parent and child", child.Path)
	}

	if child.ID == second[0].ID {
		t.Error("tags under different parents share an identity")
	}

	repeated := ReplaceSourcePaths(first, [][]string{{" animals ", " cATS "}}, "manual")
	if len(repeated) != 1 || repeated[0].ID != child.ID {
		t.Errorf("normalized nested identity changed: %v", repeated)
	}
}
