package api

import (
	"net/http"
	"strings"
)

// withAuth wraps a handler with bearer token authentication.
func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// If no bearer token is configured, skip auth
		if s.cfg.BearerToken == "" {
			next(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			s.logger.Warn("missing authorization header", "remote_addr", r.RemoteAddr)
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		// Expect "Bearer <token>" format
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			s.logger.Warn("invalid authorization format", "remote_addr", r.RemoteAddr)
			http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if token != s.cfg.BearerToken {
			s.logger.Warn("invalid bearer token", "remote_addr", r.RemoteAddr)
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
