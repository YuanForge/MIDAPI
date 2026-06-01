package handler

import (
	"fanapi/internal/config"
	"fanapi/pkg/mailer"
)

type AuthHandler struct {
	cfg    *config.ServerConfig
	mailer *mailer.Mailer
}

func NewAuthHandler(cfg *config.ServerConfig, m *mailer.Mailer) *AuthHandler {
	return &AuthHandler{cfg: cfg, mailer: m}
}
