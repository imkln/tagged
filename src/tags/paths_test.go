package tags

import (
	"reflect"
	"testing"
)

func TestShowsNestedTagPaths(t *testing.T) {
	first := ReplaceSourcePaths(nil, [][]string{{"Exampleland"}, {"Exampleland", "Garden"}}, "metadata")
	second := ReplaceSourcePaths(nil, [][]string{{"Otherland", "Garden"}}, "metadata")
	var city Tag

	for _, tag := range first {
		if tag.Name == "Garden" {
			city = tag
		}
	}

	if !reflect.DeepEqual(city.Path, []string{"Exampleland", "Garden"}) {
		t.Fatalf("city path = %v, want country and city", city.Path)
	}

	if city.ID == second[0].ID {
		t.Error("cities under different parents share an identity")
	}

	repeated := ReplaceSourcePaths(first, [][]string{{" exampleland ", " gARDEN "}}, "metadata")
	if len(repeated) != 1 || repeated[0].ID != city.ID {
		t.Errorf("normalized nested identity changed: %v", repeated)
	}
}
