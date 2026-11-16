package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"reddit-lab/internal/comments/api"
	"reddit-lab/internal/comments/store"
)

func main() {
	ctx := context.Background()
	prod := mustOpen(ctx, os.Getenv("DATABASE_URL"), os.Getenv("REDIS_URL"))
	sister := mustOpen(ctx, os.Getenv("SISTER_DATABASE_URL"), os.Getenv("SISTER_REDIS_URL"))

	srv := api.New(api.Config{
		Prod:   prod,
		Sister: sister,
		Legacy: os.Getenv("LEGACY_URL"),
	})

	log.Printf("comments listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", srv.Routes()))
}

func mustOpen(ctx context.Context, dbURL, redisURL string) *store.Store {
	st, err := store.Open(ctx, dbURL, redisURL)
	if err != nil {
		log.Fatalf("datastore: %v", err)
	}
	return st
}
