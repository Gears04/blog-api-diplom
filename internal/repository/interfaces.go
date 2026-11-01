package repository

import (
	"context"

	"blog-api/internal/model"
)

type UserRepository interface {
	Create(ctx context.Context, user model.User) (model.User, error)
	GetByEmail(ctx context.Context, email string) (model.User, error)
	GetByID(ctx context.Context, id int64) (model.User, error)
}

type PostRepository interface {
	Create(ctx context.Context, post model.Post) (model.Post, error)
	GetByID(ctx context.Context, id int64) (model.Post, error)
	GetAll(ctx context.Context, limit, offset int) ([]model.Post, int64, error)
	Update(ctx context.Context, post model.Post) (model.Post, error)
	Delete(ctx context.Context, id int64) error
}

type CommentRepository interface {
	Create(ctx context.Context, comment model.Comment) (model.Comment, error)
	GetByPostID(ctx context.Context, postID int64, limit, offset int) ([]model.Comment, int64, error)
}
