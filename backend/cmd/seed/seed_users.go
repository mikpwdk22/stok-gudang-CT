package main

import (
	"context"
	"fmt"
	"log"

	"ctstock/backend/internal/config"
	"ctstock/backend/internal/database"
	"ctstock/backend/internal/models"
	"ctstock/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type SeedUser struct {
	Username string
	Password string
	FullName string
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	ctx := context.Background()
	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Ensure migrations are up to date
	migrator := database.NewMigrationRunner(db.Pool, "migrations")
	if err := migrator.Up(ctx); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	userRepo := repository.NewUserRepository(db.Pool)

	// Pre-seed family member accounts (equal permissions for all)
	seedUsers := []SeedUser{
		{
			Username: "admin",
			Password: "password123",
			FullName: "Admin Store",
		},
		{
			Username: "ayah",
			Password: "password123",
			FullName: "Ayah",
		},
		{
			Username: "andi",
			Password: "password123",
			FullName: "Andi",
		},
		{
			Username: "budi",
			Password: "password123",
			FullName: "Budi",
		},
	}

	for _, su := range seedUsers {
		existing, err := userRepo.GetByUsername(ctx, su.Username)
		if err != nil {
			log.Printf("Error checking user %s: %v\n", su.Username, err)
			continue
		}

		if existing != nil {
			log.Printf("User '%s' already exists, skipping.\n", su.Username)
			continue
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(su.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("Failed to hash password for %s: %v\n", su.Username, err)
			continue
		}

		user := &models.User{
			Username:     su.Username,
			PasswordHash: string(hash),
			FullName:     su.FullName,
		}

		if err := userRepo.Create(ctx, user); err != nil {
			log.Printf("Failed to create user %s: %v\n", su.Username, err)
			continue
		}

		log.Printf("Successfully created user: %s (%s) [ID: %d]\n", user.Username, user.FullName, user.ID)
	}

	fmt.Println("\nAll family member accounts seeded successfully!")
	fmt.Println("Initial default password for all seeded accounts is: password123")
}

