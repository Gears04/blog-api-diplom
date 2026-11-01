package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"blog-api/internal/model"
	"blog-api/internal/repository"
)

const (
	uniqueViolationSQLState     = "23505"
	foreignKeyViolationSQLState = "23503"
)

type sqlStateError interface {
	SQLState() string
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, user model.User) (model.User, error) {
	const query = `
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, username, email, password_hash, created_at`

	var created model.User
	err := r.db.QueryRowContext(ctx, query, user.Username, user.Email, user.PasswordHash).Scan(
		&created.ID,
		&created.Username,
		&created.Email,
		&created.PasswordHash,
		&created.CreatedAt,
	)
	if err != nil {
		return model.User{}, translateError(err, "create user")
	}

	return created, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	const query = `
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE email = $1`

	return r.get(ctx, query, email)
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (model.User, error) {
	const query = `
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE id = $1`

	return r.get(ctx, query, id)
}

func (r *UserRepository) get(ctx context.Context, query string, arg any) (model.User, error) {
	var user model.User
	err := r.db.QueryRowContext(ctx, query, arg).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		return model.User{}, translateError(err, "get user")
	}

	return user, nil
}

func translateError(err error, operation string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return repository.ErrNotFound
	}

	var postgresErr sqlStateError
	if errors.As(err, &postgresErr) {
		switch postgresErr.SQLState() {
		case uniqueViolationSQLState:
			return repository.ErrConflict
		case foreignKeyViolationSQLState:
			return repository.ErrNotFound
		}
	}

	return fmt.Errorf("%s: %w", operation, err)
}

var _ repository.UserRepository = (*UserRepository)(nil)
