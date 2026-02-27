package http

import (
	"encoding/json"
	"net/http"
	"quantlo/internal/model"
	"quantlo/internal/service"
)

type Handler struct {
	svc service.LedgerService
}

func NewHandler(svc service.LedgerService) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /accounts", h.CreateAccount)
	mux.HandleFunc("DELETE /accounts", h.DeleteAccount)
	mux.HandleFunc("GET /balance", h.GetBalance)
	mux.HandleFunc("POST /recharge", h.Recharge)
	mux.HandleFunc("POST /spend", h.Spend)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"account_id"`
		Type   string `json:"resource_type"`
		Amount int64  `json:"initial_amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := h.svc.CreateAccount(r.Context(), req.ID, req.Type, req.Amount); err != nil {
		h.respondError(w, http.StatusConflict, err.Error())
		return
	}
	h.respondJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

func (h *Handler) Spend(w http.ResponseWriter, r *http.Request) {
	var req model.SpendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	res, err := h.svc.Spend(r.Context(), req)
	if err != nil {
		h.respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, res)
}

func (h *Handler) Recharge(w http.ResponseWriter, r *http.Request) {
	var req model.RechargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid_json")
		return
	}
	if err := h.svc.Recharge(r.Context(), req); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	accID := r.URL.Query().Get("account_id")
	resType := r.URL.Query().Get("resource_type")
	if accID == "" || resType == "" {
		h.respondError(w, http.StatusBadRequest, "missing_params")
		return
	}
	bal, err := h.svc.GetBalance(r.Context(), accID, resType)
	if err != nil {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]interface{}{"balance": bal})
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	accID := r.URL.Query().Get("account_id")
	resType := r.URL.Query().Get("resource_type")
	if accID == "" || resType == "" {
		h.respondError(w, http.StatusBadRequest, "missing_params")
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), accID, resType); err != nil {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	}
	h.respondJSON(w, http.StatusNoContent, nil)
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}
