package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ctstock/backend/internal/middleware"
	"ctstock/backend/internal/models"
	"ctstock/backend/internal/services"
)

type StockHandler struct {
	stockService *services.StockService
}

func NewStockHandler(stockService *services.StockService) *StockHandler {
	return &StockHandler{stockService: stockService}
}

// StockIn handles POST /api/stock/in
func (h *StockHandler) StockIn(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
		return
	}

	claims, ok := r.Context().Value(middleware.UserContextKey).(*services.JWTClaims)
	if !ok || claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var req models.StockMovementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON request body"})
		return
	}

	tx, err := h.stockService.StockIn(r.Context(), claims.UserID, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(tx)
}

// StockOut handles POST /api/stock/out
func (h *StockHandler) StockOut(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
		return
	}

	claims, ok := r.Context().Value(middleware.UserContextKey).(*services.JWTClaims)
	if !ok || claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
		return
	}

	var req models.StockMovementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON request body"})
		return
	}

	tx, err := h.stockService.StockOut(r.Context(), claims.UserID, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tx)
}

// ListTransactions handles GET /api/transactions
func (h *StockHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
		return
	}

	query := r.URL.Query()
	filter := models.TransactionFilter{}

	if typeStr := query.Get("type"); typeStr != "" {
		tt := models.TransactionType(typeStr)
		filter.TransactionType = &tt
	}

	if userStr := query.Get("user_id"); userStr != "" {
		if uid, err := strconv.Atoi(userStr); err == nil && uid > 0 {
			filter.UserID = &uid
		}
	}

	if prodStr := query.Get("product_id"); prodStr != "" {
		if pid, err := strconv.Atoi(prodStr); err == nil && pid > 0 {
			filter.ProductID = &pid
		}
	}

	if fromStr := query.Get("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.FromDate = &t
		}
	}

	if toStr := query.Get("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			// Include full end of day
			t = t.Add(24*time.Hour - time.Nanosecond)
			filter.ToDate = &t
		}
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	resp, err := h.stockService.ListTransactions(r.Context(), filter)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetTransactionByID handles GET /api/transactions/:id
func (h *StockHandler) GetTransactionByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Method Not Allowed"})
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Missing transaction ID"})
		return
	}

	id, err := strconv.Atoi(parts[2])
	if err != nil || id <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid transaction ID"})
		return
	}

	tx, err := h.stockService.GetTransactionByID(r.Context(), id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if tx == nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Transaction not found"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tx)
}

