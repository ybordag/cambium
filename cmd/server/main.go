package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/ybordag/cambium/internal/api"
	"github.com/ybordag/cambium/internal/db"
)

func main() {
	ctx := context.Background()

	pool, err := db.Connect(ctx)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("cambium listening on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, api.NewRouter(pool)))
}
