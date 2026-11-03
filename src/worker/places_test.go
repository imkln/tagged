package worker

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestGeoapifyResolvesCountryAndCity(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
		country  string
		city     string
	}{
		{
			name:     "complete location",
			response: `{"results":[{"country":"Exampleland","city":"Garden","state":"North","street":"Example Street","datasource":{"attribution":"© OpenStreetMap contributors","url":"https://www.openstreetmap.org/copyright"}}]}`,
			country:  "Exampleland",
			city:     "Garden",
		},
		{name: "country only", response: `{"results":[{"country":"Exampleland","state":"North"}]}`, country: "Exampleland"},
		{name: "no result", response: `{"results":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: placeTransport(func(request *http.Request) (*http.Response, error) {
				calls += 1
				query := request.URL.Query()
				if request.Method != http.MethodGet || request.URL.Scheme != "https" || request.URL.Host != "api.geoapify.com" || request.URL.Path != "/v1/geocode/reverse" {
					t.Errorf("unexpected lookup endpoint: %s %s", request.Method, request.URL.Redacted())
				}

				for key, want := range map[string]string{
					"lat": "-12.5", "lon": "-45.25", "apiKey": "fictional-key",
					"lang": "en", "format": "json", "limit": "1",
				} {
					if query.Get(key) != want {
						t.Errorf("%s = %q, want %q", key, query.Get(key), want)
					}
				}

				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(test.response)), Header: make(http.Header)}, nil
			})}
			resolver := NewGeoapifyResolver("fictional-key", client)

			result, err := resolver.Resolve(t.Context(), -12.5, -45.25)

			if err != nil {
				t.Fatalf("resolve location: %v", err)
			}

			if result.Country != test.country || result.City != test.city || calls != 1 {
				t.Errorf("location = %+v; calls = %d, want %q / %q and one call", result, calls, test.country, test.city)
			}

			if test.country != "" {
				if !slices.ContainsFunc(result.Attributions, func(credit Attribution) bool {
					return credit.Name == "Geoapify" && credit.URL == "https://www.geoapify.com/"
				}) {
					t.Error("saved location lacks Geoapify attribution")
				}
			}

			if test.name == "complete location" && !slices.ContainsFunc(result.Attributions, func(credit Attribution) bool { return credit.Name == "© OpenStreetMap contributors" }) {
				t.Error("provider data-source attribution was discarded")
			}
		})
	}
}

func TestGeoapifyHandlesFailuresWithoutLeakingCredentials(t *testing.T) {
	for _, test := range []struct {
		name           string
		status         int
		body           string
		transportError bool
	}{
		{name: "rate limit", status: 429, body: `{"message":"fictional-key"}`},
		{name: "service unavailable", status: 503},
		{name: "malformed JSON", status: 200, body: `{`},
		{name: "unexpected response", status: 200, body: `{}`},
		{name: "transport failure", transportError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			client := &http.Client{Transport: placeTransport(func(request *http.Request) (*http.Response, error) {
				calls += 1
				if test.transportError {
					return nil, errors.New("lookup failed with fictional-key at " + request.URL.String())
				}

				return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body)), Header: make(http.Header)}, nil
			})}
			resolver := NewGeoapifyResolver("fictional-key", client)

			_, err := resolver.Resolve(t.Context(), -12.5, -45.25)

			if err == nil {
				t.Fatal("lookup failure was swallowed")
			}

			if strings.Contains(err.Error(), "fictional-key") || strings.Contains(err.Error(), "lat=") {
				t.Errorf("error exposed sensitive request details: %v", err)
			}

			if calls != 1 {
				t.Errorf("lookup attempted %d requests, want one without automatic retries", calls)
			}
		})
	}
}

func TestGeoapifySkipsInvalidCoordinatesAndHonorsCancellation(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: placeTransport(func(request *http.Request) (*http.Response, error) {
		calls += 1
		return nil, errors.New("unexpected transport call")
	})}
	resolver := NewGeoapifyResolver("fictional-key", client)

	for _, coordinates := range [][2]float64{{91, 0}, {0, 181}} {
		if _, err := resolver.Resolve(t.Context(), coordinates[0], coordinates[1]); err == nil {
			t.Errorf("accepted out-of-range coordinates %v", coordinates)
		}
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := resolver.Resolve(ctx, 0, 0); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled lookup error = %v, want context cancellation", err)
	}

	if calls != 0 {
		t.Errorf("invalid or canceled lookups made %d HTTP requests", calls)
	}
}

func TestWorkerUsesInjectedPlaceResolverForMissingPlaceNames(t *testing.T) {
	for _, test := range []struct {
		name        string
		data        func(*testing.T) []byte
		location    Place
		lookupError error
		wantCalls   int
		wantNames   []string
	}{
		{
			name: "GPS location",
			data: func(t *testing.T) []byte {
				return metadataJPEG(t, "2026:07:12 09:30:00", "")
			},
			location: Place{
				Country:      "Exampleland",
				City:         "Garden",
				Attributions: []Attribution{{Name: "Geoapify", URL: "https://www.geoapify.com/"}},
			},
			wantCalls: 1,
			wantNames: []string{"Exampleland", "Garden"},
		},
		{
			name: "country only",
			data: func(t *testing.T) []byte {
				return metadataJPEG(t, "2026:07:12 09:30:00", "")
			},
			location:  Place{Country: "Exampleland"},
			wantCalls: 1,
			wantNames: []string{"Exampleland"},
		},
		{
			name: "lookup fails",
			data: func(t *testing.T) []byte {
				return metadataJPEG(t, "2026:07:12 09:30:00", "")
			},
			lookupError: errors.New("fictional provider unavailable"),
			wantCalls:   1,
			wantNames:   []string{},
		},
		{
			name: "no GPS",
			data: func(t *testing.T) []byte {
				return fictionalJPEG(t, 80, 40)
			},
			wantNames: []string{},
		},
		{
			name: "embedded place names",
			data: func(t *testing.T) []byte {
				return placeMetadataJPEG(t, "Exampleland", "Garden")
			},
			wantNames: []string{"Exampleland", "Garden"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newWorkerFixture()
			resolver := &fakePlaceResolver{result: test.location, err: test.lookupError}
			fixture.worker.places = resolver
			photo := fixture.seed("pending")
			data := test.data(t)
			photo.Size = int64(len(data))
			fixture.repository.records[photo.ID] = photo
			fixture.objects.data[photo.OriginalKey] = data

			event := photoCreatedEvent(fixture, photo)
			for range 2 {
				if err := fixture.worker.HandleS3Event(t.Context(), event); err != nil {
					t.Fatalf("process photo: %v", err)
				}
			}

			saved, err := fixture.repository.Get(t.Context(), photo.ID)
			if err != nil {
				t.Fatalf("read processed photo: %v", err)
			}

			if saved.Status != "ready" || len(fixture.objects.writes) != 1 || len(resolver.coordinates) != test.wantCalls {
				t.Fatalf("status = %s, thumbnail writes = %d, lookups = %d", saved.Status, len(fixture.objects.writes), len(resolver.coordinates))
			}

			if !reflect.DeepEqual(saved.Attributions, test.location.Attributions) {
				t.Errorf("saved attribution = %v, want %v", saved.Attributions, test.location.Attributions)
			}

			names := make([]string, 0, len(saved.Tags))
			for _, tag := range saved.Tags {
				names = append(names, tag.Name)
			}

			slices.Sort(names)
			if !reflect.DeepEqual(names, test.wantNames) {
				t.Errorf("tags = %v, want %v", names, test.wantNames)
			}

			if test.wantCalls > 0 && resolver.coordinates[0] != [2]float64{-12.5, -45.25} {
				t.Errorf("lookup coordinates = %v, want extracted GPS", resolver.coordinates)
			}
		})
	}
}

type placeTransport func(*http.Request) (*http.Response, error)

func (transport placeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type fakePlaceResolver struct {
	result      Place
	err         error
	coordinates [][2]float64
}

func (resolver *fakePlaceResolver) Resolve(_ context.Context, latitude, longitude float64) (Place, error) {
	resolver.coordinates = append(resolver.coordinates, [2]float64{latitude, longitude})

	return resolver.result, resolver.err
}
