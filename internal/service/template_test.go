package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"blog-api/internal/model"
)

func TestUsernameValidation(t *testing.T) {
	for _, name := range []string{"", "  ", "ab", strings.Repeat("я", 51), "abc\x00"} {
		repo := &fakeUserRepository{}
		s := NewUserService(repo, "test-secret", time.Hour)
		_, err := s.Register(context.Background(), model.RegisterRequest{Username: name, Email: "a@example.com", Password: "123456"})
		assertServiceErrorCode(t, err, CodeInvalid)
		if repo.created.Email != "" {
			t.Fatal("invalid username was saved")
		}
	}
	for _, name := range []string{"abc", strings.Repeat("я", 50)} {
		repo := &fakeUserRepository{}
		s := NewUserService(repo, "test-secret", time.Hour)
		if _, err := s.Register(context.Background(), model.RegisterRequest{Username: name, Email: "a@example.com", Password: "123456"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTemplateCommentLength(t *testing.T) {
	posts := newFakePostRepository(model.Post{ID: 1})
	comments := &fakeCommentRepository{}
	s := NewCommentService(comments, posts)
	_, err := s.Create(context.Background(), 1, 1, model.CommentRequest{Content: strings.Repeat("я", 1000)})
	if err != nil {
		t.Fatalf("1000 characters should be accepted: %v", err)
	}
	_, err = s.Create(context.Background(), 1, 1, model.CommentRequest{Content: strings.Repeat("я", 1001)})
	assertServiceErrorCode(t, err, CodeInvalid)
	if comments.createCalls != 1 {
		t.Fatal("too long comment was saved")
	}
}
