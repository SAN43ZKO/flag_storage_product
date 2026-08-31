package handler

import (
	"net/http"

	"storage/internal/service"
)

type ProductChangeHandler struct {
	changeSvc *service.ProductChangeService
}

func NewProductChangeHandler(changeSvc *service.ProductChangeService) *ProductChangeHandler {
	return &ProductChangeHandler{changeSvc: changeSvc}
}

func (h *ProductChangeHandler) List(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	changes, err := h.changeSvc.List(r.Context(), search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJson(w, http.StatusOK, changes)
}
