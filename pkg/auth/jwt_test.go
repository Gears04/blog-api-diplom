package auth

import (
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestJWTValidation(t *testing.T) {
	m := NewJWTManager("test-secret")
	token, err := m.GenerateToken(5, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if id, err := m.ValidateToken(token); err != nil || id != 5 {
		t.Fatalf("valid token rejected: %v", err)
	}
	if _, err := NewJWTManager("another-secret").ValidateToken(token); err == nil {
		t.Fatal("accepted wrong signature")
	}
	for _, tc := range []struct {
		name   string
		userID int64
		expiry time.Time
	}{
		{"expired", 5, time.Now().Add(-time.Hour)},
		{"zero user", 0, time.Now().Add(time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := m.GenerateToken(tc.userID, tc.expiry)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.ValidateToken(token); err == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	wrongMethod, err := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"iss": tokenIssuer, "user_id": 5, "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ValidateToken(wrongMethod); err == nil {
		t.Fatal("accepted a different algorithm")
	}
}
