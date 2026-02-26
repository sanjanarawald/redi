package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"redi/config"
)

type contextKeyType string

const configKey contextKeyType = "config"

func WithConfig(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), configKey, cfg)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
