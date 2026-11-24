package config_test

import (
	"os"
	"testing"

	"ctstock/backend/internal/config"
)

func TestConfigDefaults(t *testing.T) {
	os.Clearenv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.DBHost != "localhost" {
		t.Errorf("expected default DBHost localhost, got %s", cfg.DBHost)
	}
	if cfg.DBPort != "5432" {
		t.Errorf("expected default DBPort 5432, got %s", cfg.DBPort)
	}
	if cfg.DBName != "ctstock" {
		t.Errorf("expected default DBName ctstock, got %s", cfg.DBName)
	}
}

func TestConfigCustomEnv(t *testing.T) {
	os.Setenv("PORT", "9090")
	os.Setenv("DB_NAME", "ctstock_test")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
	if cfg.DBName != "ctstock_test" {
		t.Errorf("expected DBName ctstock_test, got %s", cfg.DBName)
	}
}

