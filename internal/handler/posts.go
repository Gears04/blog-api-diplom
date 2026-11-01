package handler

import (
	"errors"
	"log"
	"net/http"

	"blog-api/internal/model"
)

func (h *Handler) Posts(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/posts" || r.URL.Path == "/posts/" {
		h.postsCollection(w, r)
		return
	}

	postID, comments, err := parsePostPath(r.URL.Path)
	if err != nil {
		if errors.Is(err, errUnknownPostPath) {
			writeError(w, http.StatusNotFound, "resource not found")
		} else {
			writeError(w, http.StatusBadRequest, "invalid post id")
		}
		return
	}
	if comments {
		h.postComments(w, r, postID)
		return
	}
	h.postItem(w, r, postID)
}

func (h *Handler) postsCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPosts(w, r)
	case http.MethodPost:
		h.auth.Require(http.HandlerFunc(h.createPost)).ServeHTTP(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) postItem(w http.ResponseWriter, r *http.Request, postID int64) {
	switch r.Method {
	case http.MethodGet:
		h.getPost(w, r, postID)
	case http.MethodPut:
		h.auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.updatePost(w, r, postID)
		})).ServeHTTP(w, r)
	case http.MethodDelete:
		h.auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.deletePost(w, r, postID)
		})).ServeHTTP(w, r)
	default:
		methodNotAllowed(w, "GET, PUT, DELETE")
	}
}

func (h *Handler) listPosts(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	posts, total, err := h.posts.GetAll(r.Context(), limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.PostListResponse{Posts: posts, Total: total, Limit: limit, Offset: offset})
}

func (h *Handler) createPost(w http.ResponseWriter, r *http.Request) {
	var request model.PostRequest
	if err := decodeJSON(w, r, &request); err != nil {
		handleRequestError(w, err)
		return
	}

	post, err := h.posts.Create(r.Context(), authenticatedUserID(r), request)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	log.Printf("user %d created post %d", authenticatedUserID(r), post.ID)
	writeJSON(w, http.StatusCreated, post)
}

func (h *Handler) getPost(w http.ResponseWriter, r *http.Request, postID int64) {
	post, err := h.posts.GetByID(r.Context(), postID)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) updatePost(w http.ResponseWriter, r *http.Request, postID int64) {
	var request model.PostRequest
	if err := decodeJSON(w, r, &request); err != nil {
		handleRequestError(w, err)
		return
	}

	post, err := h.posts.Update(r.Context(), postID, authenticatedUserID(r), request)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, post)
}

func (h *Handler) deletePost(w http.ResponseWriter, r *http.Request, postID int64) {
	if err := h.posts.Delete(r.Context(), postID, authenticatedUserID(r)); err != nil {
		handleServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
