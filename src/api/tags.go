package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/imkln/tagged/src/tags"
)

func matchesTags(photo photoRecord, selected []string) bool {
	for _, id := range selected {
		if !slices.ContainsFunc(photo.Tags, func(tag tags.Tag) bool { return tag.ID == id }) {
			return false
		}
	}

	return true
}

func (app *server) tagList(w http.ResponseWriter, r *http.Request) {
	if app.photos == nil {
		app.photoError(w, errors.New("photo storage is not configured"))
		return
	}

	photos, err := app.photos.repository.List(r.Context())
	if err != nil {
		app.photoError(w, err)
		return
	}

	query := tags.Normalize(r.URL.Query().Get("q"))
	unique := make(map[string]tags.Tag)
	counts := make(map[string]int)
	for _, photo := range photos {
		seen := make(map[string]bool)
		for _, tag := range photo.Tags {
			if !seen[tag.ID] {
				counts[tag.ID]++
				seen[tag.ID] = true
			}

			path := tag.Path
			if len(path) == 0 {
				path = []string{tag.Name}
			}

			if !strings.Contains(tags.Normalize(strings.Join(path, " ")), query) {
				continue
			}

			previous, exists := unique[tag.ID]
			if !exists || tag.Name < previous.Name {
				unique[tag.ID] = tag
			}
		}
	}

	type suggestion struct {
		tags.Tag
		Count int `json:"count"`
	}

	result := make([]suggestion, 0, len(unique))
	for _, tag := range unique {
		result = append(result, suggestion{Tag: tag, Count: counts[tag.ID]})
	}

	slices.SortFunc(result, func(a, b suggestion) int {
		if comparison := strings.Compare(tags.Normalize(a.Name), tags.Normalize(b.Name)); comparison != 0 {
			return comparison
		}

		return strings.Compare(a.ID, b.ID)
	})
	writeJSON(w, http.StatusOK, map[string]any{"tags": result})
}

type photoCursor struct {
	Offset int    `json:"offset"`
	Query  string `json:"query"`
}

func photoPage(photos []photoRecord, query url.Values) ([]photoRecord, string, error) {
	limit := 50
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 200 {
			return nil, "", errors.New("invalid page size")
		}

		limit = parsed
	}

	selected := slices.Clone(query["tag"])
	slices.Sort(selected)
	selected = slices.Compact(selected)
	encoded, _ := json.Marshal(selected)
	digest := sha256.Sum256(encoded)
	fingerprint := hex.EncodeToString(digest[:])
	cursor := photoCursor{Query: fingerprint}
	if value := query.Get("cursor"); value != "" {
		data, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || json.Unmarshal(data, &cursor) != nil || cursor.Offset < 0 || cursor.Query != fingerprint {
			return nil, "", errors.New("invalid cursor")
		}
	}

	start := min(cursor.Offset, len(photos))
	end := start + min(limit, len(photos)-start)
	next := ""
	if end < len(photos) {
		data, _ := json.Marshal(photoCursor{Offset: end, Query: fingerprint})
		next = base64.RawURLEncoding.EncodeToString(data)
	}

	return photos[start:end], next, nil
}
