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
		log.Fatalf("Failed to load config: %v", err)
	}

	command := flag.String("cmd", "", "command for goose (up, down)")
	flag.Parse()

	if *command == "" {
		fmt.Println("Error: command flag is required")
		flag.Usage()
		os.Exit(1)
	}

	dsn := cfg.DSN()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	extraArgs := flag.Args()

	log.Printf("Starting migration: %s...", *command)
	if err := repository.RunMigrations(ctx, dsn, *command, extraArgs...); err != nil {
		log.Fatalf("Error running migrations: %v", err)
	}

	fmt.Println("Migrations completed successfully")
}