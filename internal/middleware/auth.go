package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type TokenValidator interface {
	ValidateToken(token string) (int64, error)
}

type Auth struct {
	validator TokenValidator
}

type userIDKey struct{}

func NewAuth(validator TokenValidator) *Auth {
	return &Auth{validator: validator}
}

func (a *Auth) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeUnauthorized(w, "authorization token is required")
			return
		}

		userID, err := a.validator.ValidateToken(parts[1])
		if err != nil {
			writeUnauthorized(w, "invalid or expired token")
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserID(ctx context.Context) (int64, bool) {
	userID, ok := ctx.Value(userIDKey{}).(int64)
	return userID, ok && userID > 0
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
