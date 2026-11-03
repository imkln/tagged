package worker

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type geoapifyResolver struct {
	apiKey string
	client *http.Client
}

func NewGeoapifyResolver(apiKey string, client *http.Client) PlaceResolver {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}

	return &geoapifyResolver{apiKey: strings.TrimSpace(apiKey), client: client}
}

func (resolver *geoapifyResolver) Resolve(ctx context.Context, latitude, longitude float64) (Place, error) {
	if err := ctx.Err(); err != nil {
		return Place{}, err
	}

	if resolver.apiKey == "" || math.IsNaN(latitude) || math.IsNaN(longitude) || math.Abs(latitude) > 90 || math.Abs(longitude) > 180 {
		return Place{}, errors.New("invalid location lookup configuration or coordinates")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := url.Values{
		"lat":    {strconv.FormatFloat(latitude, 'f', -1, 64)},
		"lon":    {strconv.FormatFloat(longitude, 'f', -1, 64)},
		"apiKey": {resolver.apiKey},
		"lang":   {"en"},
		"format": {"json"},
		"limit":  {"1"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.geoapify.com/v1/geocode/reverse?"+query.Encode(), nil)
	if err != nil {
		return Place{}, errors.New("unable to prepare location lookup")
	}

	response, err := resolver.client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return Place{}, ctx.Err()
		}

		// HTTP errors can contain the complete request URL, including coordinates and API key.
		return Place{}, errors.New("location lookup request failed")
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return Place{}, errors.New("location lookup service unavailable")
	}

	var body struct {
		Results []struct {
			Country    string `json:"country"`
			City       string `json:"city"`
			Datasource struct {
				Attribution string `json:"attribution"`
				URL         string `json:"url"`
			} `json:"datasource"`
		} `json:"results"`
	}

	data, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 || json.Unmarshal(data, &body) != nil || body.Results == nil {
		return Place{}, errors.New("invalid location lookup response")
	}

	if len(body.Results) == 0 {
		return Place{}, nil
	}

	first := body.Results[0]
	place := Place{Country: strings.TrimSpace(first.Country), City: strings.TrimSpace(first.City)}
	if place.Country == "" && place.City == "" {
		return place, nil
	}

	place.Attributions = []Attribution{
		{Name: "Geoapify", URL: "https://www.geoapify.com/"},
		{Name: "© OpenStreetMap contributors", URL: "https://www.openstreetmap.org/copyright"},
	}
	if first.Datasource.Attribution != "" && first.Datasource.Attribution != "© OpenStreetMap contributors" {
		link, err := url.Parse(first.Datasource.URL)
		if err == nil && link.Scheme == "https" && link.Host != "" {
			place.Attributions = append(place.Attributions, Attribution{Name: first.Datasource.Attribution, URL: link.String()})
		} else {
			place.Attributions = append(place.Attributions, Attribution{Name: first.Datasource.Attribution})
		}
	}

	return place, nil
}
