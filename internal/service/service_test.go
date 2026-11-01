package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"blog-api/internal/model"
	"blog-api/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type fakeUserRepository struct {
	created model.User
	user    model.User
}

func (r *fakeUserRepository) Create(_ context.Context, user model.User) (model.User, error) {
	r.created = user
	user.ID = 1
	return user, nil
}

func (r *fakeUserRepository) GetByEmail(_ context.Context, email string) (model.User, error) {
	if r.user.Email != email {
		return model.User{}, repository.ErrNotFound
	}
	return r.user, nil
}

func (r *fakeUserRepository) GetByID(_ context.Context, id int64) (model.User, error) {
	if r.user.ID != id {
		return model.User{}, repository.ErrNotFound
	}
	return r.user, nil
}

func TestUserServiceRegisterHashesPasswordAndNormalizesEmail(t *testing.T) {
	repo := &fakeUserRepository{}
	service := NewUserService(repo, "test-secret", time.Hour)

	user, err := service.Register(context.Background(), model.RegisterRequest{
		Username: " student ",
		Email:    "  Student@Example.COM  ",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Register returned an error: %v", err)
	}

	const normalizedEmail = "student@example.com"
	if repo.created.Email != normalizedEmail {
		t.Fatalf("email was not normalized: got %q, want %q", repo.created.Email, normalizedEmail)
	}
	if user.User.Email != normalizedEmail || user.User.Username != "student" {
		t.Fatalf("returned user was not normalized: %+v", user.User)
	}
	if id, err := service.ValidateToken(user.Token); err != nil || id != user.User.ID {
		t.Fatalf("registration must issue a valid JWT: %v", err)
	}
	claims := &jwt.RegisteredClaims{}
	_, err = jwt.ParseWithClaims(user.Token, claims, func(*jwt.Token) (any, error) { return []byte("test-secret"), nil })
	if err != nil || !claims.ExpiresAt.Time.Equal(user.ExpiresAt) {
		t.Fatal("expires_at must match JWT expiration")
	}
	encoded, err := json.Marshal(user)
	if err != nil || strings.Contains(string(encoded), "password") {
		t.Fatal("registration response must not expose password or hash")
	}
	if repo.created.PasswordHash == "password123" || repo.created.PasswordHash == "" {
		t.Fatal("password was not replaced with a hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(repo.created.PasswordHash), []byte("password123")); err != nil {
		t.Fatalf("stored password hash does not match the password: %v", err)
	}
}

func TestUserServiceLoginReturnsValidToken(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("prepare password hash: %v", err)
	}

	repo := &fakeUserRepository{user: model.User{
		ID:           42,
		Email:        "student@example.com",
		PasswordHash: string(hash),
	}}
	service := NewUserService(repo, "test-secret", time.Hour)

	response, err := service.Login(context.Background(), model.LoginRequest{
		Email:    "STUDENT@example.com",
		Password: "password123",
	})
	if err != nil {
		t.Fatalf("Login returned an error: %v", err)
	}
	if response.Token == "" {
		t.Fatal("Login returned an empty access token")
	}
	if !response.ExpiresAt.After(time.Now()) {
		t.Fatal("token expiry must be in the future")
	}

	userID, err := service.ValidateToken(response.Token)
	if err != nil {
		t.Fatalf("ValidateToken rejected a token returned by Login: %v", err)
	}
	if userID != repo.user.ID {
		t.Fatalf("unexpected user id in token: got %d, want %d", userID, repo.user.ID)
	}
}

func TestValidateTokenRejectsTokenWithoutExpiration(t *testing.T) {
	const secret = "test-secret"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":     "blog-api",
		"user_id": 42,
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token without expiration: %v", err)
	}

	service := NewUserService(&fakeUserRepository{}, secret, time.Hour)
	_, err = service.ValidateToken(signed)
	assertServiceErrorCode(t, err, CodeUnauthorized)
}

type fakePostRepository struct {
	posts       map[int64]model.Post
	updateCalls int
	deleteCalls int
}

func newFakePostRepository(post model.Post) *fakePostRepository {
	return &fakePostRepository{posts: map[int64]model.Post{post.ID: post}}
}

func (r *fakePostRepository) Create(_ context.Context, post model.Post) (model.Post, error) {
	post.ID = int64(len(r.posts) + 1)
	r.posts[post.ID] = post
	return post, nil
}

func (r *fakePostRepository) GetByID(_ context.Context, id int64) (model.Post, error) {
	post, ok := r.posts[id]
	if !ok {
		return model.Post{}, repository.ErrNotFound
	}
	return post, nil
}

func (r *fakePostRepository) GetAll(_ context.Context, _, _ int) ([]model.Post, int64, error) {
	posts := make([]model.Post, 0, len(r.posts))
	for _, post := range r.posts {
		posts = append(posts, post)
	}
	return posts, int64(len(posts)), nil
}

func (r *fakePostRepository) Update(_ context.Context, post model.Post) (model.Post, error) {
	if _, ok := r.posts[post.ID]; !ok {
		return model.Post{}, repository.ErrNotFound
	}
	r.updateCalls++
	r.posts[post.ID] = post
	return post, nil
}

func (r *fakePostRepository) Delete(_ context.Context, id int64) error {
	if _, ok := r.posts[id]; !ok {
		return repository.ErrNotFound
	}
	r.deleteCalls++
	delete(r.posts, id)
	return nil
}

func TestPostServiceChecksAuthorForUpdateAndDelete(t *testing.T) {
	const (
		postID   int64 = 10
		authorID int64 = 7
		otherID  int64 = 8
	)
	existing := model.Post{ID: postID, AuthorID: authorID, Title: "Old title", Content: "Old content"}
	update := model.PostRequest{Title: "New title", Content: "New content"}

	t.Run("another user cannot update", func(t *testing.T) {
		repo := newFakePostRepository(existing)
		_, err := NewPostService(repo).Update(context.Background(), postID, otherID, update)
		assertServiceErrorCode(t, err, CodeForbidden)
		if repo.updateCalls != 0 {
			t.Fatal("repository Update was called for another user")
		}
	})

	t.Run("author can update", func(t *testing.T) {
		repo := newFakePostRepository(existing)
		updated, err := NewPostService(repo).Update(context.Background(), postID, authorID, update)
		if err != nil {
			t.Fatalf("Update returned an error for the author: %v", err)
		}
		if repo.updateCalls != 1 || updated.Title != update.Title || updated.Content != update.Content {
			t.Fatalf("post was not updated: %+v", updated)
		}
	})

	t.Run("another user cannot delete", func(t *testing.T) {
		repo := newFakePostRepository(existing)
		err := NewPostService(repo).Delete(context.Background(), postID, otherID)
		assertServiceErrorCode(t, err, CodeForbidden)
		if repo.deleteCalls != 0 {
			t.Fatal("repository Delete was called for another user")
		}
	})

	t.Run("author can delete", func(t *testing.T) {
		repo := newFakePostRepository(existing)
		err := NewPostService(repo).Delete(context.Background(), postID, authorID)
		if err != nil {
			t.Fatalf("Delete returned an error for the author: %v", err)
		}
		if repo.deleteCalls != 1 {
			t.Fatalf("unexpected Delete call count: got %d, want 1", repo.deleteCalls)
		}
	})
}

type fakeCommentRepository struct {
	createCalls int
}

func (r *fakeCommentRepository) Create(_ context.Context, comment model.Comment) (model.Comment, error) {
	r.createCalls++
	return comment, nil
}

func (r *fakeCommentRepository) GetByPostID(_ context.Context, _ int64, _, _ int) ([]model.Comment, int64, error) {
	return []model.Comment{}, 0, nil
}

func TestCommentServiceRejectsMissingPost(t *testing.T) {
	posts := newFakePostRepository(model.Post{ID: 1, AuthorID: 1})
	comments := &fakeCommentRepository{}
	service := NewCommentService(comments, posts)

	_, err := service.Create(context.Background(), 999, 1, model.CommentRequest{Content: "Comment"})
	assertServiceErrorCode(t, err, CodeNotFound)
	if comments.createCalls != 0 {
		t.Fatal("comment was sent to the repository for a missing post")
	}
}

func TestLoginChecksPasswordByteLimit(t *testing.T) {
	for name, password := range map[string]string{
		"ASCII":   strings.Repeat("a", 72),
		"Unicode": strings.Repeat("я", 36),
	} {
		t.Run(name, func(t *testing.T) {
			hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
			if err != nil {
				t.Fatal(err)
			}
			repo := &fakeUserRepository{user: model.User{
				ID: 1, Email: "student@example.com", PasswordHash: string(hash),
			}}
			s := NewUserService(repo, "test-secret", time.Hour)
			response, err := s.Login(context.Background(), model.LoginRequest{
				Email: repo.user.Email, Password: password,
			})
			if err != nil || response.Token == "" {
				t.Fatalf("correct 72-byte password rejected: %v", err)
			}
			response, err = s.Login(context.Background(), model.LoginRequest{
				Email: repo.user.Email, Password: password + "DIFFERENT",
			})
			assertServiceErrorCode(t, err, CodeUnauthorized)
			if response.Token != "" {
				t.Fatal("token issued for a different password")
			}
		})
	}
}

func TestPostServiceRejectsNullCharacters(t *testing.T) {
	for name, request := range map[string]model.PostRequest{
		"title":   {Title: "Title\x00", Content: "Text"},
		"content": {Title: "Title", Content: "Text\x00"},
	} {
		t.Run(name, func(t *testing.T) {
			existing := model.Post{ID: 1, AuthorID: 1, Title: "Old", Content: "Old text"}
			repo := newFakePostRepository(existing)
			s := NewPostService(repo)
			_, err := s.Create(context.Background(), 1, request)
			assertServiceErrorCode(t, err, CodeInvalid)
			if len(repo.posts) != 1 {
				t.Fatal("invalid post was saved")
			}
			_, err = s.Update(context.Background(), 1, 1, request)
			assertServiceErrorCode(t, err, CodeInvalid)
			if repo.updateCalls != 0 || repo.posts[1] != existing {
				t.Fatal("invalid update changed the existing post")
			}
		})
	}
}

func TestCommentServiceRejectsNullCharacter(t *testing.T) {
	posts := newFakePostRepository(model.Post{ID: 1, AuthorID: 1})
	comments := &fakeCommentRepository{}
	s := NewCommentService(comments, posts)
	_, err := s.Create(context.Background(), 1, 1, model.CommentRequest{Content: "Comment\x00"})
	assertServiceErrorCode(t, err, CodeInvalid)
	if comments.createCalls != 0 {
		t.Fatal("invalid comment was sent to the repository")
	}
}

func assertServiceErrorCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected service error %q, got nil", want)
	}
	var serviceErr *Error
	if !errors.As(err, &serviceErr) {
		t.Fatalf("expected *service.Error, got %T: %v", err, err)
	}
	if serviceErr.Code != want {
		t.Fatalf("unexpected error code: got %q, want %q", serviceErr.Code, want)
	}
}
