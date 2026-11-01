package handler

import (
	"net/http"

	"blog-api/internal/model"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}

	var request model.RegisterRequest
	if err := decodeJSON(w, r, &request); err != nil {
		handleRequestError(w, err)
		return
	}

	user, err := h.users.Register(r.Context(), request)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}

	var request model.LoginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		handleRequestError(w, err)
		return
	}

	response, err := h.users.Login(r.Context(), request)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}
