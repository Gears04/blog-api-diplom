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

type PostService struct {
	repo repository.PostRepository
}

func NewPostService(repo repository.PostRepository) *PostService {
	return &PostService{repo: repo}
}

func (s *PostService) Create(ctx context.Context, authorID int64, request model.PostRequest) (model.Post, error) {
	if authorID <= 0 {
		return model.Post{}, newError(CodeUnauthorized, "authentication required")
	}

	title, content, err := validatePost(request)
	if err != nil {
		return model.Post{}, err
	}

	post, err := s.repo.Create(ctx, model.Post{Title: title, Content: content, AuthorID: authorID})
	if errors.Is(err, repository.ErrNotFound) {
		return model.Post{}, newError(CodeUnauthorized, "user not found")
	}
	if err != nil {
		return model.Post{}, fmt.Errorf("create post: %w", err)
	}
	return post, nil
}

func (s *PostService) GetByID(ctx context.Context, id int64) (model.Post, error) {
	post, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return model.Post{}, newError(CodeNotFound, "post not found")
	}
	if err != nil {
		return model.Post{}, fmt.Errorf("get post: %w", err)
	}
	return post, nil
}

func (s *PostService) GetAll(ctx context.Context, limit, offset int) ([]model.Post, int64, error) {
	if err := validatePagination(limit, offset); err != nil {
		return nil, 0, err
	}
	posts, total, err := s.repo.GetAll(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get posts: %w", err)
	}
	return posts, total, nil
}

func validatePagination(limit, offset int) error {
	if limit < 1 || limit > 100 || offset < 0 {
		return newError(CodeInvalid, "limit must be 1 to 100 and offset must be zero or greater")
	}
	return nil
}

func (s *PostService) Update(ctx context.Context, id, authorID int64, request model.PostRequest) (model.Post, error) {
	if authorID <= 0 {
		return model.Post{}, newError(CodeUnauthorized, "authentication required")
	}

	title, content, err := validatePost(request)
	if err != nil {
		return model.Post{}, err
	}

	current, err := s.GetByID(ctx, id)
	if err != nil {
		return model.Post{}, err
	}
	if current.AuthorID != authorID {
		return model.Post{}, newError(CodeForbidden, "only the author can update this post")
	}

	current.Title = title
	current.Content = content
	updated, err := s.repo.Update(ctx, current)
	if errors.Is(err, repository.ErrNotFound) {
		return model.Post{}, newError(CodeNotFound, "post not found")
	}
	if err != nil {
		return model.Post{}, fmt.Errorf("update post: %w", err)
	}
	return updated, nil
}

func (s *PostService) Delete(ctx context.Context, id, authorID int64) error {
	if authorID <= 0 {
		return newError(CodeUnauthorized, "authentication required")
	}

	current, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if current.AuthorID != authorID {
		return newError(CodeForbidden, "only the author can delete this post")
	}

	if err := s.repo.Delete(ctx, id); errors.Is(err, repository.ErrNotFound) {
		return newError(CodeNotFound, "post not found")
	} else if err != nil {
		return fmt.Errorf("delete post: %w", err)
	}
	return nil
}

func validatePost(request model.PostRequest) (string, string, error) {
	title := strings.TrimSpace(request.Title)
	content := strings.TrimSpace(request.Content)
	if title == "" {
		return "", "", newError(CodeInvalid, "title is required")
	}
	if utf8.RuneCountInString(title) > 200 {
		return "", "", newError(CodeInvalid, "title must not exceed 200 characters")
	}
	if content == "" {
		return "", "", newError(CodeInvalid, "content is required")
	}
	if strings.ContainsRune(title, '\x00') || strings.ContainsRune(content, '\x00') {
		return "", "", newError(CodeInvalid, "title and content must not contain a null character")
	}
	if utf8.RuneCountInString(content) > 50000 {
		return "", "", newError(CodeInvalid, "content must not exceed 50000 characters")
	}
	return title, content, nil
}
