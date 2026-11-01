package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"blog-api/internal/model"
	"blog-api/internal/repository"
)

type CommentService struct {
	comments repository.CommentRepository
	posts    repository.PostRepository
}

func NewCommentService(comments repository.CommentRepository, posts repository.PostRepository) *CommentService {
	return &CommentService{comments: comments, posts: posts}
}

func (s *CommentService) Create(ctx context.Context, postID, authorID int64, request model.CommentRequest) (model.Comment, error) {
	if authorID <= 0 {
		return model.Comment{}, newError(CodeUnauthorized, "authentication required")
	}
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return model.Comment{}, newError(CodeInvalid, "text is required")
	}
	if strings.ContainsRune(content, '\x00') {
		return model.Comment{}, newError(CodeInvalid, "text must not contain a null character")
	}
	if utf8.RuneCountInString(content) > 1000 {
		return model.Comment{}, newError(CodeInvalid, "comment must not exceed 1000 characters")
	}

	if _, err := s.posts.GetByID(ctx, postID); errors.Is(err, repository.ErrNotFound) {
		return model.Comment{}, newError(CodeNotFound, "post not found")
	} else if err != nil {
		return model.Comment{}, fmt.Errorf("get post: %w", err)
	}

	comment, err := s.comments.Create(ctx, model.Comment{PostID: postID, AuthorID: authorID, Content: content})
	if errors.Is(err, repository.ErrNotFound) {
		return model.Comment{}, newError(CodeNotFound, "post not found")
	}
	if err != nil {
		return model.Comment{}, fmt.Errorf("create comment: %w", err)
	}
	return comment, nil
}

func (s *CommentService) GetByPostID(ctx context.Context, postID int64, limit, offset int) ([]model.Comment, int64, error) {
	if err := validatePagination(limit, offset); err != nil {
		return nil, 0, err
	}
	if _, err := s.posts.GetByID(ctx, postID); errors.Is(err, repository.ErrNotFound) {
		return nil, 0, newError(CodeNotFound, "post not found")
	} else if err != nil {
		return nil, 0, fmt.Errorf("get post: %w", err)
	}

	comments, total, err := s.comments.GetByPostID(ctx, postID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get comments: %w", err)
	}
	return comments, total, nil
}
