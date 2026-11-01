package handler

import (
	"log"
	"net/http"

	"blog-api/internal/model"
)

func (h *Handler) postComments(w http.ResponseWriter, r *http.Request, postID int64) {
	switch r.Method {
	case http.MethodGet:
		h.listComments(w, r, postID)
	case http.MethodPost:
		h.auth.Require(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.createComment(w, r, postID)
		})).ServeHTTP(w, r)
	default:
		methodNotAllowed(w, "GET, POST")
	}
}

func (h *Handler) createComment(w http.ResponseWriter, r *http.Request, postID int64) {
	var request model.CommentRequest
	if err := decodeJSON(w, r, &request); err != nil {
		handleRequestError(w, err)
		return
	}

	comment, err := h.comments.Create(r.Context(), postID, authenticatedUserID(r), request)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	log.Printf("user %d created comment %d on post %d", authenticatedUserID(r), comment.ID, postID)
	writeJSON(w, http.StatusCreated, comment)
}

func (h *Handler) listComments(w http.ResponseWriter, r *http.Request, postID int64) {
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	comments, total, err := h.comments.GetByPostID(r.Context(), postID, limit, offset)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.CommentListResponse{Comments: comments, Total: total, Limit: limit, Offset: offset})
}
