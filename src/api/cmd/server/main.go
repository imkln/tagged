package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/awslabs/aws-lambda-go-api-proxy/httpadapter"
	"github.com/imkln/tagged/src/api"
)

func main() {
	assets := flag.String("assets", "src/web/dist", "frontend build directory")
	flag.Parse()
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		*assets = "web"
	}
	config := api.Config{
		Origin:       os.Getenv("APP_ORIGIN"),
		Issuer:       os.Getenv("OIDC_ISSUER"),
		ClientID:     os.Getenv("OIDC_CLIENT_ID"),
		AuthDomain:   os.Getenv("AUTH_DOMAIN"),
		OwnerEmail:   os.Getenv("OWNER_EMAIL"),
		OwnerSubject: os.Getenv("OWNER_SUBJECT"),
		CookieKey:    os.Getenv("COOKIE_KEY"),
	}
	handler, err := api.New(context.Background(), config, os.DirFS(*assets))
	if err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
	if os.Getenv("AWS_LAMBDA_RUNTIME_API") != "" {
		lambda.Start(httpadapter.NewV2(handler).ProxyWithContext)
		return
	}
	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Minute,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			slog.Error("shutdown failed", "error", err)
		}
	}()
	slog.Info("listening", "url", "http://127.0.0.1:8080")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
