package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"blog-api/internal/middleware"
	"blog-api/internal/service"
)

const maxRequestBody = 1 << 20

var (
	errInvalidPostID   = errors.New("invalid post id")
	errUnknownPostPath = errors.New("unknown post path")
)

type requestError struct {
	status  int
	message string
}

func (e *requestError) Error() string {
	return e.message
}

type DatabasePinger interface {
	PingContext(ctx context.Context) error
}

type Handler struct {
	users    *service.UserService
	posts    *service.PostService
	comments *service.CommentService
	auth     *middleware.Auth
	db       DatabasePinger
}

func New(
	users *service.UserService,
	posts *service.PostService,
	comments *service.CommentService,
	auth *middleware.Auth,
	db DatabasePinger,
) *Handler {
	return &Handler{users: users, posts: posts, comments: comments, auth: auth, db: db}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/register", h.Register)
	mux.HandleFunc("/login", h.Login)
	mux.HandleFunc("/posts", h.Posts)
	mux.HandleFunc("/posts/", h.Posts)
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/", h.NotFound)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject malformed paths with JSON instead of ServeMux's HTML redirect.
		clean := path.Clean(r.URL.Path)
		if clean != r.URL.Path && (clean == "/" || clean+"/" != r.URL.Path) {
			h.NotFound(w, r)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (h *Handler) NotFound(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "resource not found")
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return &requestError{status: http.StatusUnsupportedMediaType, message: "Content-Type must be application/json"}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return &requestError{status: http.StatusRequestEntityTooLarge, message: "request body is too large"}
		}
		if errors.Is(err, io.EOF) {
			return &requestError{status: http.StatusBadRequest, message: "request body must contain JSON"}
		}
		return &requestError{status: http.StatusBadRequest, message: "invalid JSON body"}
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return &requestError{status: http.StatusRequestEntityTooLarge, message: "request body is too large"}
		}
		return &requestError{status: http.StatusBadRequest, message: "request body must contain one JSON object"}
	}
	return nil
}

func parsePostPath(path string) (int64, bool, error) {
	remainder := strings.Trim(strings.TrimPrefix(path, "/posts/"), "/")
	parts := strings.Split(remainder, "/")
	if len(parts) < 1 || len(parts) > 2 || parts[0] == "" {
		return 0, false, errUnknownPostPath
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, false, errInvalidPostID
	}
	if len(parts) == 2 {
		if parts[1] != "comments" {
			return 0, false, errUnknownPostPath
		}
		return id, true, nil
	}
	return id, false, nil
}

func handleRequestError(w http.ResponseWriter, err error) {
	var inputError *requestError
	if errors.As(err, &inputError) {
		writeError(w, inputError.status, inputError.message)
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func pagination(r *http.Request) (int, int, error) {
	limit := 10
	offset := 0
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return 0, 0, errors.New("invalid query parameters")
	}

	if values, present := query["limit"]; present {
		if len(values) != 1 {
			return 0, 0, errors.New("limit must be specified once")
		}
		limit, err = strconv.Atoi(values[0])
		if err != nil || limit < 1 || limit > 100 {
			return 0, 0, errors.New("limit must be between 1 and 100")
		}
	}
	if values, present := query["offset"]; present {
		if len(values) != 1 {
			return 0, 0, errors.New("offset must be specified once")
		}
		offset, err = strconv.Atoi(values[0])
		if err != nil || offset < 0 {
			return 0, 0, errors.New("offset must be zero or greater")
		}
	}
	return limit, offset, nil
}

func handleServiceError(w http.ResponseWriter, err error) {
	var appError *service.Error
	if errors.As(err, &appError) {
		status := http.StatusInternalServerError
		switch appError.Code {
		case service.CodeInvalid:
			status = http.StatusBadRequest
		case service.CodeUnauthorized:
			status = http.StatusUnauthorized
		case service.CodeForbidden:
			status = http.StatusForbidden
		case service.CodeNotFound:
			status = http.StatusNotFound
		case service.CodeConflict:
			status = http.StatusConflict
		}
		writeError(w, status, appError.Message)
		return
	}

	log.Printf("internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func authenticatedUserID(r *http.Request) int64 {
	userID, _ := middleware.UserID(r.Context())
	return userID
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}
