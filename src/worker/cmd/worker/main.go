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

	handler, err := worker.NewAWSHandler(context.Background(), os.Getenv("PHOTO_BUCKET"), os.Getenv("PHOTO_TABLE"), maximum)
	if err != nil {
		slog.Error("thumbnail worker startup failed", "error", err)
		os.Exit(1)
	}

	lambda.Start(handler)
}
