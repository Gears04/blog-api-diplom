package handler

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"blog-api/internal/logger"
	"blog-api/internal/middleware"
	"blog-api/internal/model"
	"blog-api/internal/repository"
	"blog-api/internal/service"
)

type readyDatabase struct{}

func (readyDatabase) PingContext(context.Context) error { return nil }

type diplomaComments struct {
	repository.CommentRepository
	user    *authTestRepository
	comment model.Comment
}

func (r *diplomaComments) Create(_ context.Context, c model.Comment) (model.Comment, error) {
	c.ID, c.CreatedAt = 2, time.Now().UTC()
	c.Author = model.AuthorResponse{ID: r.user.user.ID, Username: r.user.user.Username}
	r.comment = c
	return c, nil
}

func (r *diplomaComments) GetByPostID(_ context.Context, _ int64, _, _ int) ([]model.Comment, int64, error) {
	if r.comment.ID == 0 {
		return []model.Comment{}, 0, nil
	}
	return []model.Comment{r.comment}, 1, nil
}

func requireFields(t *testing.T, body []byte, fields ...string) {
	t.Helper()
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range fields {
		if _, ok := decoded[field]; !ok {
			t.Fatalf("missing %q in %s", field, body)
		}
	}
}

func TestDiplomaRoutesFieldsAndEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	fileLogger, err := logger.New(path)
	if err != nil {
		t.Fatal(err)
	}
	previous := log.Writer()
	log.SetOutput(fileLogger)
	t.Cleanup(func() { log.SetOutput(previous); _ = fileLogger.Close() })
	users := &authTestRepository{}
	posts := &contentTestRepository{user: users}
	comments := &diplomaComments{user: users}
	us := service.NewUserService(users, "test-secret", time.Hour)
	h := New(us, service.NewPostService(posts), service.NewCommentService(comments, posts), middleware.NewAuth(us), readyDatabase{}).Routes()
	request := func(method, path, body, token string, status int) []byte {
		t.Helper()
		w := jsonRequest(h, method, path, body, token)
		if w.Code != status || !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("%s %s: got %d %s", method, path, w.Code, w.Body)
		}
		if status >= 400 {
			requireFields(t, w.Body.Bytes(), "error")
		}
		return w.Body.Bytes()
	}
	request("GET", "/health", "", "", 200)
	request("POST", "/register", `{"username":"student","email":"student@example.com","password":"123456"}`, "", 201)
	body := request("POST", "/login", `{"email":"student@example.com","password":"123456"}`, "", 200)
	var loggedIn model.TokenResponse
	if err := json.Unmarshal(body, &loggedIn); err != nil {
		t.Fatal(err)
	}
	token := loggedIn.Token
	request("POST", "/posts", `{"title":"Post","content":"Body"}`, "", 401)
	request("POST", "/posts", `{"title":" ","content":"Body"}`, token, 400)
	body = request("POST", "/posts", `{"title":"Post","content":"Body"}`, token, 201)
	requireFields(t, body, "id", "author_id", "title", "content", "created_at")
	body = request("GET", "/posts/7", "", "", 200)
	requireFields(t, body, "id", "author_id", "title", "content", "created_at")
	request("GET", "/posts", "", "", 200)
	request("POST", "/posts/7/comments", `{"text":"Comment"}`, "", 401)
	request("POST", "/posts/7/comments", `{"text":" "}`, token, 400)
	request("POST", "/posts/7/comments", `{"content":"Unexpected field"}`, token, 400)
	body = request("POST", "/posts/7/comments", `{"text":"Comment"}`, token, 201)
	requireFields(t, body, "id", "post_id", "author_id", "text", "created_at")
	var c map[string]any
	_ = json.Unmarshal(body, &c)
	if c["text"] != "Comment" || c["author_id"] != float64(1) || c["post_id"] != float64(7) {
		t.Fatalf("wrong comment: %s", body)
	}
	request("GET", "/posts/7/comments", "", "", 200)
	for _, path := range []string{"/posts", "/posts/7", "/posts/7/comments"} {
		body := request("GET", path, "", "", 200)
		if strings.Contains(string(body), `"email"`) || strings.Contains(string(body), "student@example.com") {
			t.Fatalf("public response exposes email at %s", path)
		}
	}
	request("GET", "/posts/999", "", "", 404)
	request("GET", "/posts/7/comments?limit=0", "", "", 400)
	request("GET", "/register", "", "", 405)
	request("POST", "/health", "", "", 405)
	request("GET", "/missing", "", "", 404)
	request("GET", "/posts//7", "", "", 404)
	if err := fileLogger.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "created post") != 1 || strings.Count(text, "created comment") != 1 || !strings.Contains(text, "user 1 created post 7") || !strings.Contains(text, "user 1 created comment 2") {
		t.Fatalf("missing or duplicate events: %s", text)
	}
	if strings.Contains(text, token) || strings.Contains(text, "123456") {
		t.Fatal("secret in log")
	}
}
