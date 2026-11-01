package handler

import (
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"

	"blog-api/internal/model"
	"blog-api/internal/repository"
	"blog-api/internal/service"
)

type paginationRepository struct {
	repository.PostRepository
	limit, offset, calls int
}

func (r *paginationRepository) GetAll(_ context.Context, limit, offset int) ([]model.Post, int64, error) {
	r.limit, r.offset = limit, offset
	r.calls++
	return []model.Post{}, 5, nil
}

func TestPostsPagination(t *testing.T) {
	cases := []struct {
		query                 string
		status, limit, offset int
	}{
		{"", 200, 10, 0},
		{"?limit=1&offset=2", 200, 1, 2},
		{"?limit=100&offset=0", 200, 100, 0},
		{"?limit=%ZZ", 400, 0, 0},
		{"?limit=0;offset=0", 400, 0, 0},
		{"?limit=", 400, 0, 0},
		{"?offset=", 400, 0, 0},
		{"?limit=1&limit=2", 400, 0, 0},
		{"?offset=0&offset=1", 400, 0, 0},
		{"?limit=0", 400, 0, 0},
		{"?limit=101", 400, 0, 0},
		{"?offset=-1", 400, 0, 0},
		{"?offset=text", 400, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			repo := &paginationRepository{}
			h := New(nil, service.NewPostService(repo), nil, nil, nil).Routes()
			r := httptest.NewRequest(http.MethodGet, "/posts"+tc.query, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("got %d, want %d: %s", w.Code, tc.status, w.Body.String())
			}
			mediaType, params, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
			if err != nil || mediaType != "application/json" || params["charset"] != "utf-8" || !json.Valid(w.Body.Bytes()) {
				t.Fatal("response must be JSON with the correct Content-Type")
			}
			if tc.status == http.StatusOK {
				var body model.PostListResponse
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body.Posts == nil || body.Total != 5 || body.Limit != tc.limit || body.Offset != tc.offset {
					t.Fatalf("invalid pagination response: %s", w.Body.String())
				}
				if repo.calls != 1 || repo.limit != tc.limit || repo.offset != tc.offset {
					t.Fatalf("wrong pagination sent to repository: %+v", repo)
				}
			} else {
				var body map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["error"] == "" {
					t.Fatal("expected a JSON error message")
				}
				if repo.calls != 0 {
					t.Fatal("invalid query reached repository")
				}
			}
		})
	}
}

func TestMalformedPathsReturnJSONWithoutRedirect(t *testing.T) {
	h := New(nil, nil, nil, nil, readyDatabase{}).Routes()
	for _, path := range []string{"//", "///", "//health", "/./", "/posts//7", "/posts/../health", "/posts/./7", "/missing"} {
		for _, method := range []string{"GET", "POST"} {
			t.Run(method+" "+path, func(t *testing.T) {
				w := jsonRequest(h, method, path, "", "")
				var body map[string]string
				if w.Code != http.StatusNotFound || json.Unmarshal(w.Body.Bytes(), &body) != nil || body["error"] == "" {
					t.Fatalf("expected JSON 404, got %d %s", w.Code, w.Body)
				}
				mediaType, _, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
				if err != nil || mediaType != "application/json" || w.Header().Get("Location") != "" {
					t.Fatal("expected JSON without redirect")
				}
			})
		}
	}
}

func TestAPIPrefixIsNotRegistered(t *testing.T) {
	h := New(nil, nil, nil, nil, nil).Routes()
	for _, tc := range []struct {
		method, path string
	}{
		{"POST", "/api/register"},
		{"POST", "/api/login"},
		{"GET", "/api/posts"},
		{"POST", "/api/posts"},
		{"GET", "/api/posts/7"},
		{"PUT", "/api/posts/7"},
		{"DELETE", "/api/posts/7"},
		{"GET", "/api/posts/7/comments"},
		{"POST", "/api/posts/7/comments"},
		{"GET", "/api/health"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := jsonRequest(h, tc.method, tc.path, "", "")
			var body map[string]string
			if w.Code != http.StatusNotFound || json.Unmarshal(w.Body.Bytes(), &body) != nil || body["error"] == "" {
				t.Fatalf("expected JSON 404, got %d %s", w.Code, w.Body)
			}
			mediaType, _, err := mime.ParseMediaType(w.Header().Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				t.Fatal("expected JSON Content-Type")
			}
		})
	}
}
