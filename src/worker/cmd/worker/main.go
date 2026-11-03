package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/imkln/tagged/src/worker"
)

func main() {
	maximum := int64(256 << 20)
	if raw := os.Getenv("MAX_UPLOAD_BYTES"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			slog.Error("MAX_UPLOAD_BYTES must be a positive number")
			os.Exit(1)
		}

		maximum = parsed
	}

	maximumPixels := worker.DefaultMaxImagePixels
	if raw := os.Getenv("MAX_IMAGE_PIXELS"); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			slog.Error("MAX_IMAGE_PIXELS must be a positive number")
			os.Exit(1)
		}

		maximumPixels = parsed
	}

	var places worker.PlaceResolver
	if apiKey := os.Getenv("GEOAPIFY_API_KEY"); apiKey != "" {
		places = worker.NewGeoapifyResolver(apiKey, nil)
	}

	handler, err := worker.NewAWSHandler(context.Background(), os.Getenv("PHOTO_BUCKET"), os.Getenv("PHOTO_TABLE"), maximum, maximumPixels, places)
	if err != nil {
		slog.Error("thumbnail worker startup failed", "error", err)
		os.Exit(1)
	}

	lambda.Start(handler)
}
