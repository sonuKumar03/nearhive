package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sonukumar/nearhive/internal/auth"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(mgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				JSONError(w, http.StatusUnauthorized, "missing authorization header", "UNAUTHORIZED", nil)
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := mgr.ValidateToken(tokenStr)
			if err != nil {
				JSONError(w, http.StatusUnauthorized, "invalid or expired token", "UNAUTHORIZED", nil)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuthMiddleware(mgr *auth.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
				if claims, err := mgr.ValidateToken(tokenStr); err == nil {
					ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}


func GetUserIDFromContext(ctx context.Context) uuid.UUID {
	if ctx == nil {
		return uuid.Nil
	}
	val, ok := ctx.Value(UserIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil
	}
	return val
}
