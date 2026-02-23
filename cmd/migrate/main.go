package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"quantlo/internal/config"
	"quantlo/internal/repository"
	"time"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		fmt.Println("Error: migration command is required")
		fmt.Println("Usage: go run cmd/migrate/main.go [command] [args]")
		fmt.Println("Commands: up, down, status, redo")
		os.Exit(1)
	}

	command := args[0]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	log.Printf("Starting migration: %s", command)

	if err := repository.RunMigrations(ctx, cfg.DSN(), command); err != nil {
		log.Fatalf("Migration error: %v", err)
	}

	fmt.Println("Migration finished successfully")
}