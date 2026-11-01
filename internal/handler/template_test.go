package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"blog-api/internal/middleware"
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"blog-api/internal/service"
)

type authTestRepository struct {
	repository.UserRepository
	user model.User
}

func (r *authTestRepository) Create(_ context.Context, user model.User) (model.User, error) {
	if r.user.ID != 0 && (r.user.Email == user.Email || r.user.Username == user.Username) {
		return model.User{}, repository.ErrConflict
	}
	user.ID, user.CreatedAt = 1, time.Now().UTC()
	r.user = user
	return user, nil
}

func (r *authTestRepository) GetByEmail(_ context.Context, email string) (model.User, error) {
	if r.user.ID == 0 || r.user.Email != email {
		return model.User{}, repository.ErrNotFound
	}
	return r.user, nil
}

type contentTestRepository struct {
	repository.PostRepository
	user *authTestRepository
	post model.Post
}

func (r *contentTestRepository) Create(_ context.Context, post model.Post) (model.Post, error) {
	post.ID = 7
	post.Author = model.AuthorResponse{ID: r.user.user.ID, Username: r.user.user.Username}
	r.post = post
	return post, nil
}

func (r *contentTestRepository) GetByID(_ context.Context, id int64) (model.Post, error) {
	if r.post.ID != id {
		return model.Post{}, repository.ErrNotFound
	}
	return r.post, nil
}

func (r *contentTestRepository) GetAll(_ context.Context, _, _ int) ([]model.Post, int64, error) {
	if r.post.ID == 0 {
		return []model.Post{}, 0, nil
	}
	return []model.Post{r.post}, 1, nil
}

type commentPaginationRepository struct {
	repository.CommentRepository
	limit, offset, calls int
}

func (r *commentPaginationRepository) GetByPostID(_ context.Context, _ int64, limit, offset int) ([]model.Comment, int64, error) {
	r.limit, r.offset = limit, offset
	r.calls++
	return []model.Comment{}, 4, nil
}

func jsonRequest(h http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestRegistrationAndLoginTokenResponse(t *testing.T) {
	users := &authTestRepository{}
	posts := &contentTestRepository{user: users}
	userService := service.NewUserService(users, "test-secret", time.Hour)
	h := New(userService, service.NewPostService(posts), nil, middleware.NewAuth(userService), nil).Routes()
	w := jsonRequest(h, "POST", "/register", `{"username":"student","email":"student@example.com","password":"123456"}`, "")
	if w.Code != http.StatusCreated {
		t.Fatalf("register: %d %s", w.Code, w.Body)
	}
	var registered model.TokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &registered); err != nil {
		t.Fatal(err)
	}
	if registered.Token == "" || registered.User.Username != "student" || registered.User.Email != "student@example.com" || !registered.ExpiresAt.After(time.Now()) {
		t.Fatalf("invalid registration response: %s", w.Body)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("registration leaked a password")
	}
	w = jsonRequest(h, "POST", "/posts", `{"title":"Test","content":"Text"}`, registered.Token)
	if w.Code != http.StatusCreated {
		t.Fatalf("registration token rejected: %d %s", w.Code, w.Body)
	}
	var post model.Post
	if err := json.Unmarshal(w.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	if post.Author.ID != registered.User.ID || post.Author.Username != "student" {
		t.Fatalf("missing author object: %s", w.Body)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("post leaked a password")
	}
	w = jsonRequest(h, "POST", "/login", `{"email":"student@example.com","password":"123456"}`, "")
	var login model.TokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusOK || login.Token == "" || login.User != registered.User {
		t.Fatalf("login response mismatch: %d %s", w.Code, w.Body)
	}
	for _, body := range []string{
		`{"username":"student","email":"different@example.com","password":"123456"}`,
		`{"username":"different","email":"STUDENT@example.com","password":"123456"}`,
	} {
		w = jsonRequest(h, "POST", "/register", body, "")
		if w.Code != http.StatusConflict {
			t.Fatalf("duplicate must return 409: %d %s", w.Code, w.Body)
		}
	}
}

func TestCommentsPagination(t *testing.T) {
	for _, tc := range []struct {
		query                 string
		status, limit, offset int
	}{
		{"", 200, 10, 0}, {"?limit=1&offset=100", 200, 1, 100},
		{"?limit=0", 400, 0, 0}, {"?offset=-1", 400, 0, 0},
		{"?limit=1&limit=2", 400, 0, 0}, {"?offset=", 400, 0, 0},
	} {
		t.Run(tc.query, func(t *testing.T) {
			posts := &contentTestRepository{post: model.Post{ID: 7}}
			comments := &commentPaginationRepository{}
			h := New(nil, nil, service.NewCommentService(comments, posts), nil, nil).Routes()
			w := jsonRequest(h, "GET", "/posts/7/comments"+tc.query, "", "")
			if w.Code != tc.status {
				t.Fatalf("unexpected status: %d %s", w.Code, w.Body)
			}
			if tc.status != 200 {
				if comments.calls != 0 {
					t.Fatal("invalid pagination reached repository")
				}
				return
			}
			var body model.CommentListResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Comments == nil || body.Total != 4 || body.Limit != tc.limit || body.Offset != tc.offset || comments.limit != tc.limit || comments.offset != tc.offset {
				t.Fatalf("wrong pagination: %s", w.Body)
			}
		})
	}
}
