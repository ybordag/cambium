// Package main is the Cambium API gateway entry point.
//
//	@title			Cambium — Gardening Agent API
//	@version		1.0
//	@description	HTTP API gateway for the Gardening Agent system. Sits between Verdant (React frontend) and Rhizome (Python LangGraph agent). Handles authentication, encrypted provider key storage, and request routing.
//	@contact.name	Gardening Agent
//	@license.name	Apache 2.0
//
//	@host		localhost:8080
//	@BasePath	/
//	@schemes	http https
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT access token — format: "Bearer <token>"
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/ybordag/cambium/docs" // generated swagger docs
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
