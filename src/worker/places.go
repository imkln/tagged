package worker

import "context"

type Attribution struct {
	Name string `json:"name" dynamodbav:"name"`
	URL  string `json:"url" dynamodbav:"url"`
}

type Place struct {
	Country      string
	City         string
	Attributions []Attribution
}

// PlaceResolver supplies place names without exposing a provider's HTTP API to processing.
type PlaceResolver interface {
	Resolve(context.Context, float64, float64) (Place, error)
}
