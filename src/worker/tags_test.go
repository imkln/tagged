package worker

import (
	"reflect"
	"slices"
	"testing"
)

func TestMetadataCreatesOnlyCountryAndCityTags(t *testing.T) {
	for _, test := range []struct {
		name    string
		country string
		city    string
		want    []string
	}{
		{name: "country and city", country: "Exampleland", city: "Garden", want: []string{"Exampleland", "Garden"}},
		{name: "country only", country: "Exampleland", want: []string{"Exampleland"}},
		{name: "no place names", want: []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			saved := processMetadataJPEG(t, placeMetadataJPEG(t, test.country, test.city))
			tags, _ := saved["tags"].([]any)
			got := make([]string, 0, len(tags))

			for _, value := range tags {
				tag, ok := value.(map[string]any)
				if !ok {
					t.Fatalf("tag = %v, want a tag record", value)
				}

				if tag["id"] == "" || tag["id"] == nil {
					t.Error("generated tag has no identity")
				}

				if !reflect.DeepEqual(tag["sources"], []any{"metadata"}) {
					t.Errorf("sources = %v, want metadata", tag["sources"])
				}

				got = append(got, tag["name"].(string))
			}

			slices.Sort(got)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("tags = %v, want only place tags %v", got, test.want)
			}
		})
	}
}

func TestMetadataTagIdentityIgnoresCaseAndWhitespace(t *testing.T) {
	first := processMetadataJPEG(t, placeMetadataJPEG(t, "Exampleland", "Garden"))
	second := processMetadataJPEG(t, placeMetadataJPEG(t, " exampleland ", " gARDEN "))
	ids := func(fields map[string]any) []string {
		tags, _ := fields["tags"].([]any)
		result := make([]string, 0, len(tags))
		for _, value := range tags {
			result = append(result, value.(map[string]any)["id"].(string))
		}

		slices.Sort(result)
		return result
	}
	firstIDs, secondIDs := ids(first), ids(second)

	if len(firstIDs) != 2 || !reflect.DeepEqual(firstIDs, secondIDs) {
		t.Errorf("tag identities = %v and %v, want the same two identities", firstIDs, secondIDs)
	}

	duplicate := processMetadataJPEG(t, placeMetadataJPEG(t, "Garden", " garden "))
	if got := ids(duplicate); len(got) != 1 {
		t.Errorf("same name from two metadata fields produced %v, want one tag", got)
	}
}

func placeMetadataJPEG(t *testing.T, country, city string) []byte {
	t.Helper()

	// Fixed fictional names only; this helper is not an XML serializer.
	packet := `http://ns.adobe.com/xap/1.0/` + "\x00" +
		`<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">` +
		`<rdf:Description xmlns:photoshop="http://ns.adobe.com/photoshop/1.0/" xmlns:tiff="http://ns.adobe.com/tiff/1.0/" ` +
		`photoshop:Country="` + country + `" photoshop:City="` + city + `" photoshop:State="North" tiff:Make="Example Camera"/>` +
		`</rdf:RDF></x:xmpmeta>`

	return insertMetadata(fictionalJPEG(t, 80, 40), []byte(packet))
}
