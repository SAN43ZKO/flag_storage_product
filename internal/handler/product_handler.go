package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"storage/internal/model"
	"storage/internal/service"
	"strconv"
	"time"
)

type ProductHandler struct {
	svc       *service.ProductService
	uploadDir string
}

func NewProductHundler(svc *service.ProductService, uploadDir string) *ProductHandler {
	return &ProductHandler{svc: svc, uploadDir: uploadDir}
}

func (h *ProductHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stats", h.GetStats)
	mux.HandleFunc("GET /api/products/low-stock", h.GetLowStock)
	mux.HandleFunc("GET /api/categories", h.GetCategories)
	mux.HandleFunc("GET /api/units", h.GetUnits)
	mux.HandleFunc("GET /products", h.List)
	mux.HandleFunc("POST /products", h.Create)
	mux.HandleFunc("GET /products/{id}", h.GetByID)
	mux.HandleFunc("PUT /products/{id}", h.Update)
	mux.HandleFunc("DELETE /products/{id}", h.Delete)
	mux.HandleFunc("POST /products/{id}/image", h.UploadImage)
	mux.HandleFunc("GET /products/images/{filename}", h.ServeImage)
}

func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	product, err := h.svc.Create(r.Context(), req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJson(w, http.StatusCreated, product)
}

func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, " invalid product id")
		return
	}
	product, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJson(w, http.StatusOK, product)
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")

	product, err := h.svc.List(r.Context(), search)
	if err != nil {
		log.Printf("List error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	writeJson(w, http.StatusOK, product)
}

func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req model.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	product, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJson(w, http.StatusOK, product)
}

func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ProductHandler) UploadImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	log.Printf("UploadImage called: productID=%d", id)

	if _, err = h.svc.GetByID(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, "product not found")
		return
	}

	r.ParseMultipartForm(10 << 20)
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read image")
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("product_%d_%d%s", id, time.Now().UnixNano(), ext)
	filePath := filepath.Join(h.uploadDir, filename)
	log.Printf("Saving file: %s", filename)

	dst, err := os.Create(filePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save image")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		os.Remove(filePath)
		writeError(w, http.StatusInternalServerError, "failed to copy image")
		return
	}

	if err := h.svc.UpdateImage(r.Context(), id, filename); err != nil {
		os.Remove(filePath)
		writeError(w, http.StatusInternalServerError, "failed to update product")
		return
	}
	log.Printf("Calling UpdateImage: productID=%d, filename=%s", id, filename)
	if err := h.svc.UpdateImage(r.Context(), id, filename); err != nil {
		log.Printf("UpdateImage failed: %v", err)
		os.Remove(filePath)
		writeError(w, http.StatusInternalServerError, "failed to update product")
		return
	}
	log.Printf("Image uploaded successfully: productID=%d", id)
	writeJson(w, http.StatusOK, map[string]string{"image_path": filename})
}

func (h *ProductHandler) ServeImage(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	http.ServeFile(w, r, filepath.Join(h.uploadDir, filename))
}

func (h *ProductHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJson(w, http.StatusOK, stats)
}

func (h *ProductHandler) GetLowStock(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.GetLowStock(r.Context(), 10)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJson(w, http.StatusOK, products)
}

func (h *ProductHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.svc.GetCategories(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJson(w, http.StatusOK, categories)
}

func (h *ProductHandler) GetUnits(w http.ResponseWriter, r *http.Request) {
	units, err := h.svc.GetUnits(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJson(w, http.StatusOK, units)
}

// Utils
func writeJson(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJson(w, status, map[string]string{"error": msg})
}

func handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrProductNotFound):
		writeError(w, http.StatusNotFound, "product not found")
	case errors.Is(err, service.ErrNameRequired):
		writeError(w, http.StatusBadRequest, "name is required")
	case errors.Is(err, service.ErrSKURequired):
		writeError(w, http.StatusBadRequest, "sku is required")
	case errors.Is(err, service.ErrQuantityNegative):
		writeError(w, http.StatusBadRequest, "quantity cannot be negative")
	default:
		log.Printf("unhandled error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
