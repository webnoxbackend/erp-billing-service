package http

import (
	"encoding/json"
	"net/http"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"

	"github.com/google/uuid"
)

type SubscriptionHandler struct {
	subService *application.SubscriptionService
}

func NewSubscriptionHandler(subService *application.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subService: subService}
}

func (h *SubscriptionHandler) CreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.subService.CreateSubscription(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *SubscriptionHandler) UpgradeSubscription(w http.ResponseWriter, r *http.Request) {
	var req dto.UpgradeSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.subService.UpgradeSubscription(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *SubscriptionHandler) DowngradeSubscription(w http.ResponseWriter, r *http.Request) {
	var req dto.DowngradeSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.subService.DowngradeSubscription(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *SubscriptionHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.URL.Query().Get("organization_id")
	if orgIDStr == "" {
		http.Error(w, "organization_id query parameter is required", http.StatusBadRequest)
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "Invalid organization_id", http.StatusBadRequest)
		return
	}

	resp, err := h.subService.GetStatus(r.Context(), orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *SubscriptionHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.URL.Query().Get("organization_id")
	if orgIDStr == "" {
		http.Error(w, "organization_id query parameter is required", http.StatusBadRequest)
		return
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "Invalid organization_id", http.StatusBadRequest)
		return
	}

	resp, err := h.subService.GetHistory(r.Context(), orgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
