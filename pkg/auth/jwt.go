package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const tokenIssuer = "blog-api"

type JWTManager struct{ secret []byte }

type tokenClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

func NewJWTManager(secret string) *JWTManager {
	return &JWTManager{secret: []byte(secret)}
}

func (m *JWTManager) GenerateToken(userID int64, expiresAt time.Time) (string, error) {
	now := time.Now().UTC()
	claims := tokenClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    tokenIssuer,
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (m *JWTManager) ValidateToken(value string) (int64, error) {
	claims := &tokenClaims{}
	token, err := jwt.ParseWithClaims(value, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(tokenIssuer), jwt.WithExpirationRequired())
	if err != nil || token == nil || !token.Valid || claims.UserID <= 0 {
		return 0, errors.New("invalid or expired token")
	}
	return claims.UserID, nil
}
