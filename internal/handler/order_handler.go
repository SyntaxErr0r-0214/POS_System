package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"pos-demo/internal/model"
	"pos-demo/internal/service"
)

type OrderHandler struct {
	Service *service.CheckoutService
}

func (h *OrderHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req model.CheckoutRequest
	json.Unmarshal(body, &req)

	if err := h.Service.Checkout(req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Write([]byte("ok"))
}

func (h *OrderHandler) Book(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req model.BookingRequest
	json.Unmarshal(body, &req)

	id, err := h.Service.Book(req)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte(fmt.Sprintf("预订成功 #%d", id)))
}

func (h *OrderHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	orders, _ := h.Service.SearchOrders(q)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func (h *OrderHandler) Pickup(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req model.PickupRequest
	json.Unmarshal(body, &req)

	if err := h.Service.Pickup(req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Write([]byte("提货成功"))
}
