package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"blog-api/internal/model"
	"blog-api/internal/repository"

	"blog-api/pkg/auth"
)

const (
	dummyPasswordHash = "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2uheWG/igi."
)

type UserService struct {
	repo     repository.UserRepository
	tokens   *auth.JWTManager
	tokenTTL time.Duration
}

func NewUserService(repo repository.UserRepository, jwtSecret string, tokenTTL time.Duration) *UserService {
	return &UserService{
		repo:     repo,
		tokens:   auth.NewJWTManager(jwtSecret),
		tokenTTL: tokenTTL,
	}
}

func (s *UserService) Register(ctx context.Context, request model.RegisterRequest) (model.TokenResponse, error) {
	username := strings.TrimSpace(request.Username)
	if size := utf8.RuneCountInString(username); size < 3 || size > 50 || strings.ContainsRune(username, '\x00') {
		return model.TokenResponse{}, newError(CodeInvalid, "username must contain 3 to 50 characters and no null character")
	}
	email, err := validateEmail(request.Email)
	if err != nil {
		return model.TokenResponse{}, err
	}
	if err := validatePassword(request.Password); err != nil {
		return model.TokenResponse{}, err
	}

	hash, err := auth.HashPassword(request.Password)
	if err != nil {
		return model.TokenResponse{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.repo.Create(ctx, model.User{Username: username, Email: email, PasswordHash: hash})
	if errors.Is(err, repository.ErrConflict) {
		return model.TokenResponse{}, newError(CodeConflict, "user with this email or username already exists")
	}
	if err != nil {
		return model.TokenResponse{}, fmt.Errorf("create user: %w", err)
	}

	return s.tokenResponse(user)
}

func (s *UserService) Login(ctx context.Context, request model.LoginRequest) (model.TokenResponse, error) {
	email, err := validateEmail(request.Email)
	if err != nil {
		return model.TokenResponse{}, newError(CodeUnauthorized, "invalid email or password")
	}
	if request.Password == "" || len(request.Password) > 72 {
		return model.TokenResponse{}, newError(CodeUnauthorized, "invalid email or password")
	}

	user, err := s.repo.GetByEmail(ctx, email)
	if errors.Is(err, repository.ErrNotFound) {
		_ = auth.CheckPassword(dummyPasswordHash, request.Password)
		return model.TokenResponse{}, newError(CodeUnauthorized, "invalid email or password")
	}
	if err != nil {
		return model.TokenResponse{}, fmt.Errorf("get user: %w", err)
	}

	if err := auth.CheckPassword(user.PasswordHash, request.Password); err != nil {
		return model.TokenResponse{}, newError(CodeUnauthorized, "invalid email or password")
	}

	return s.tokenResponse(user)
}

func (s *UserService) tokenResponse(user model.User) (model.TokenResponse, error) {
	expiresAt := time.Now().UTC().Add(s.tokenTTL).Truncate(time.Second)
	token, err := s.tokens.GenerateToken(user.ID, expiresAt)
	if err != nil {
		return model.TokenResponse{}, err
	}

	return model.TokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      user.ToResponse(),
	}, nil
}

func (s *UserService) ValidateToken(token string) (int64, error) {
	id, err := s.tokens.ValidateToken(token)
	if err != nil {
		return 0, newError(CodeUnauthorized, "invalid or expired token")
	}
	return id, nil
}

func validateEmail(value string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 255 {
		return "", newError(CodeInvalid, "valid email is required")
	}
	return email, nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < 6 {
		return newError(CodeInvalid, "password must contain at least 6 characters")
	}
	if len([]byte(password)) > 72 {
		return newError(CodeInvalid, "password must not exceed 72 bytes")
	}
	return nil
}
