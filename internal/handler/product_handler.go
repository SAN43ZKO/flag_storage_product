package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"storage/internal/model"
	"storage/internal/service"

	"github.com/signintech/gopdf"
)

type ProductHandler struct {
	svc       *service.ProductService
	uploadDir string
}

type ReceiptItem struct {
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
	Category string `json:"category,omitempty"`
	Unit     string `json:"unit,omitempty"`
}

type ReceiptRequest struct {
	Items []ReceiptItem `json:"items"`
}

type ReceiptResult struct {
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Errors  []string `json:"errors,omitempty"`
}

func NewProductHandler(svc *service.ProductService, uploadDir string) *ProductHandler {
	return &ProductHandler{svc: svc, uploadDir: uploadDir}
}

func (h *ProductHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/stats", h.GetStats)
	mux.HandleFunc("GET /api/products/low-stock", h.GetLowStock)
	mux.HandleFunc("GET /api/categories", h.GetCategories)
	mux.HandleFunc("GET /api/units", h.GetUnits)
	mux.HandleFunc("POST /products/receipt", h.ProcessReceipt)
	mux.HandleFunc("POST /products/receipt/pdf", h.GenerateReceiptPDF)
	mux.HandleFunc("GET /products/autocomplete", h.Autocomplete)
	mux.HandleFunc("GET /products/images/{filename}", h.ServeImage)
	mux.HandleFunc("GET /products", h.List)
	mux.HandleFunc("POST /products", h.Create)
	mux.HandleFunc("GET /products/{id}", h.GetByID)
	mux.HandleFunc("PUT /products/{id}", h.Update)
	mux.HandleFunc("DELETE /products/{id}", h.Delete)
	mux.HandleFunc("POST /products/{id}/image", h.UploadImage)
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
	category := r.URL.Query().Get("category")
	sortByQty := r.URL.Query().Get("sort")

	product, err := h.svc.List(r.Context(), search, category, sortByQty)
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

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid form")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read image")
		return
	}
	defer file.Close()

	ext := filepath.Ext(header.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".gif" && ext != ".webp" {
		writeError(w, http.StatusBadRequest, "unsupported image format")
		return
	}
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
	log.Printf("Image uploaded successfully: productID=%d", id)
	writeJson(w, http.StatusOK, map[string]string{"image_path": filename})
}

func (h *ProductHandler) ServeImage(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	// Reject any path traversal attempt before touching the filesystem.
	if filename == "" || filename != filepath.Base(filename) || strings.Contains(filename, "..") {
		writeError(w, http.StatusBadRequest, "invalid filename")
		return
	}
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

func (h *ProductHandler) ProcessReceipt(w http.ResponseWriter, r *http.Request) {
	var req ReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}

	var result ReceiptResult
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			result.Errors = append(result.Errors, "Пропущена позиция: количество должно быть положительным")
			continue
		}

		var existing *model.Product

		// Ищем по SKU, если он указан
		if item.SKU != "" {
			p, err := h.svc.FindBySKU(r.Context(), item.SKU)
			if err == nil {
				existing = &p
			}
		}

		// Если не нашли по SKU, ищем по названию
		if existing == nil && item.Name != "" {
			p, err := h.svc.FindByName(r.Context(), item.Name)
			if err == nil {
				existing = &p
			}
		}

		if existing != nil {
			newQty := existing.Quantity + item.Quantity
			req := model.CreateProductRequest{
				Name:     existing.Name,
				SKU:      existing.SKU,
				Quantity: newQty,
				Reserved: existing.Reserved,
			}
			// Приоритет новым значениям из формы
			if item.Category != "" {
				req.Category = &item.Category
			} else if existing.Category != nil {
				req.Category = existing.Category
			}
			if item.Unit != "" {
				req.Unit = &item.Unit
			} else if existing.Unit != nil {
				req.Unit = existing.Unit
			}
			if existing.ImagePath != nil {
				req.ImagePath = existing.ImagePath
			}
			if _, err := h.svc.Update(r.Context(), existing.ID, req); err != nil {
				result.Errors = append(result.Errors, "Ошибка обновления товара: "+err.Error())
				continue
			}
			result.Updated++
		} else {
			// Создание нового товара
			if item.SKU == "" && item.Name == "" {
				result.Errors = append(result.Errors, "Не удалось создать товар: укажите артикул или название")
				continue
			}
			createReq := model.CreateProductRequest{
				Name:     &item.Name,
				SKU:      &item.SKU,
				Quantity: item.Quantity,
			}
			if item.Category != "" {
				createReq.Category = &item.Category
			}
			if item.Unit != "" {
				createReq.Unit = &item.Unit
			}
			if _, err := h.svc.Create(r.Context(), createReq); err != nil {
				result.Errors = append(result.Errors, "Ошибка создания товара: "+err.Error())
				continue
			}
			result.Created++
		}
	}

	writeJson(w, http.StatusOK, result)
}

func (h *ProductHandler) Autocomplete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if len(query) < 2 {
		writeJson(w, http.StatusOK, []model.ProductAutocomplete{})
		return
	}
	items, err := h.svc.Autocomplete(r.Context(), query)
	if err != nil {
		log.Printf("autocomplete error: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJson(w, http.StatusOK, items)
}

func findFont(candidates []string) string {
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func (h *ProductHandler) GenerateReceiptPDF(w http.ResponseWriter, r *http.Request) {
	var req ReceiptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "неверный формат запроса")
		return
	}
	if len(req.Items) == 0 {
		writeError(w, http.StatusBadRequest, "нет позиций для печати")
		return
	}

	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.AddPage()

	regularFont := findFont([]string{
		"/usr/share/fonts/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/DejaVuSans.ttf",
		"/usr/share/fonts/TTF/dejavu/DejaVuSans.ttf",
	})
	boldFont := findFont([]string{
		"/usr/share/fonts/dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/TTF/DejaVuSans-Bold.ttf",
		"/usr/share/fonts/TTF/dejavu/DejaVuSans-Bold.ttf",
	})

	if regularFont == "" || boldFont == "" {
		log.Printf("GenerateReceiptPDF: DejaVu fonts not found")
		writeError(w, http.StatusInternalServerError, "fonts not found")
		return
	}

	if err := pdf.AddTTFFont("dejavu", regularFont); err != nil {
		log.Printf("GenerateReceiptPDF: font regular: %v", err)
		writeError(w, http.StatusInternalServerError, "font error")
		return
	}
	if err := pdf.AddTTFFont("dejavu-bold", boldFont); err != nil {
		log.Printf("GenerateReceiptPDF: font bold: %v", err)
		writeError(w, http.StatusInternalServerError, "font error")
		return
	}

	// Заголовок
	pdf.SetFont("dejavu-bold", "", 16)
	pdf.SetXY(40, 40)
	pdf.CellWithOption(&gopdf.Rect{W: 515, H: 25},
		"Перемещение на склад",
		gopdf.CellOption{Align: gopdf.Left | gopdf.Middle})

	// Дата
	pdf.SetFont("dejavu", "", 10)
	pdf.SetXY(40, 70)
	pdf.CellWithOption(&gopdf.Rect{W: 515, H: 15},
		"Дата: "+time.Now().Format("02.01.2006 15:04"),
		gopdf.CellOption{Align: gopdf.Left | gopdf.Middle})

	// Таблица
	const (
		tableX = 40.0
		tableW = 420.0
		rowH   = 18.0
	)
	colWidths := []float64{50, 190, 90, 50, 40}
	headers := []string{"Артикул", "Наименование", "Категория", "Кол-во", "Ед."}

	startY := 100.0
	pdf.SetFont("dejavu-bold", "", 10)
	x := tableX
	for i, h := range headers {
		pdf.SetXY(x, startY)
		pdf.CellWithOption(&gopdf.Rect{W: colWidths[i], H: rowH}, h,
			gopdf.CellOption{Align: gopdf.Left | gopdf.Middle})
		x += colWidths[i]
	}

	y := startY + rowH
	pdf.Line(tableX, y, tableX+tableW, y)

	pdf.SetFont("dejavu", "", 9)
	for _, item := range req.Items {
		y += rowH
		if y > 750 {
			pdf.AddPage()
			y = 40
		}

		displayName := item.Name
		if displayName == "" {
			displayName = item.SKU
		}
		if len(displayName) > 40 {
			displayName = displayName[:40] + "…"
		}

		row := []string{
			item.SKU,
			displayName,
			item.Category,
			strconv.Itoa(item.Quantity),
			item.Unit,
		}

		x = tableX
		for i, val := range row {
			pdf.SetXY(x, y)
			pdf.CellWithOption(&gopdf.Rect{W: colWidths[i], H: rowH}, val,
				gopdf.CellOption{Align: gopdf.Left | gopdf.Middle})
			x += colWidths[i]
		}
		pdf.Line(tableX, y+rowH, tableX+tableW, y+rowH)
	}

	// Итоги
	total := 0
	for _, item := range req.Items {
		total += item.Quantity
	}
	y += rowH + 20

	pdf.SetFont("dejavu", "", 11)
	pdf.SetXY(40, y)
	pdf.CellWithOption(&gopdf.Rect{W: 515, H: 20},
		fmt.Sprintf("Всего позиций: %d, единиц: %d", len(req.Items), total),
		gopdf.CellOption{Align: gopdf.Left | gopdf.Middle})

	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		log.Printf("GenerateReceiptPDF: write: %v", err)
		writeError(w, http.StatusInternalServerError, "pdf write error")
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		`attachment; filename="warehouse-transfer-`+time.Now().Format("20060102-150405")+`.pdf"`)
	w.Write(buf.Bytes())
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
