package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds the application configuration loaded from environment variables
type Config struct {
	Port        string
	Env         string
	JWTSecret   string
	DatabaseURL string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBSSLMode   string
}

// Load loads environment variables from a .env file (if present) and system environment
func Load() (*Config, error) {
	_ = godotenv.Load(".env", "../.env", "../../.env")

	port := getEnv("PORT", "8080")
	env := getEnv("ENV", "development")
	jwtSecret := getEnv("JWT_SECRET", "ctstock_family_inventory_super_secret_jwt_key_2026")

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgrespassword")
	dbName := getEnv("DB_NAME", "ctstock")
	dbSSLMode := getEnv("DB_SSLMODE", "disable")

	defaultDBURL := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode)

	databaseURL := getEnv("DATABASE_URL", defaultDBURL)

	return &Config{
		Port:        port,
		Env:         env,
		JWTSecret:   jwtSecret,
		DatabaseURL: databaseURL,
		DBHost:      dbHost,
		DBPort:      dbPort,
		DBUser:      dbUser,
		DBPassword:  dbPassword,
		DBName:      dbName,
		DBSSLMode:   dbSSLMode,
	}, nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
