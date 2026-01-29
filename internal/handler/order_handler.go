package handler

import (
	"encoding/json"
	"net/http"
	"pos-demo/internal/model"
	"pos-demo/internal/service"
)

type OrderHandler struct {
	Service *service.CheckoutService
}

// Checkout 结算
func (h *OrderHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	var req model.CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := h.Service.Checkout(req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("success"))
}

// Book 预订
func (h *OrderHandler) Book(w http.ResponseWriter, r *http.Request) {
	var req model.BookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := h.Service.Book(req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("预订成功"))
}

// Search 搜索订单 (核心修改)
func (h *OrderHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status") // 获取状态参数

	// 默认查 Pending
	if status == "" {
		status = "Pending"
	}

	orders, err := h.Service.OrderRepo.GetOrders(status, q)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// 顺便把每个订单的 items 查出来，前端展示方便
	for i := range orders {
		items, _ := h.Service.OrderRepo.GetItemsByOrderID(orders[i].ID)
		orders[i].Items = items
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

// Pickup 提货
func (h *OrderHandler) Pickup(w http.ResponseWriter, r *http.Request) {
	var req model.PickupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := h.Service.Pickup(req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("提货成功"))
}
