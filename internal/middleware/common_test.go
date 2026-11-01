package middleware

import (
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"blog-api/internal/logger"
)

func TestRequestLoggingToFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	fileLogger, err := logger.New(path)
	if err != nil {
		t.Fatal(err)
	}
	previous := log.Writer()
	log.SetOutput(fileLogger)
	t.Cleanup(func() {
		log.SetOutput(previous)
		_ = fileLogger.Close()
	})
	h := Logging(Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/panic" {
			panic("test panic")
		}
		w.WriteHeader(http.StatusCreated)
	})))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/posts", nil))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("panic response code = %d", w.Code)
	}
	log.SetOutput(previous)
	if err := fileLogger.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"POST /posts 201", "GET /panic 500", "test panic"} {
		if !strings.Contains(string(data), expected) {
			t.Errorf("log does not contain %q", expected)
		}
	}
}
