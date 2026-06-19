package config

import (
	"strings"
	"testing"
)

func TestValidateRejectsWeakJWTSecrets(t *testing.T) {
	tests := []string{
		"",
		"short-secret",
		"change-me",
		"change-me-in-production",
		"your-jwt-secret",
	}

	for _, secret := range tests {
		cfg := &Config{
			Server: ServerConfig{
				JWTSecret:      secret,
				JWTExpireHours: 24,
			},
		}
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected %q to be rejected", secret)
		}
	}
}

func TestValidateAcceptsStrongJWTSecret(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			JWTSecret:      strings.Repeat("a", 32),
			JWTExpireHours: 24,
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected strong config to pass validation: %v", err)
	}
}

func TestValidateRejectsNonPositiveJWTExpiry(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			JWTSecret:      strings.Repeat("a", 32),
			JWTExpireHours: 0,
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected non-positive jwt expiry to be rejected")
	}
}
