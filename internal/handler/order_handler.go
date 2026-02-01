package handler

import (
	"encoding/json"
	"fmt"
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

// Search 搜索订单 (核心改动在这里！)
func (h *OrderHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	dateStr := r.URL.Query().Get("date") // [新增] 获取日期参数

	if status == "" {
		status = "Pending"
	}

	// [关键修改] 这里必须传 3 个参数，否则会报错！
	// 旧代码是: GetOrders(status, q)
	// 新代码是: GetOrders(status, q, dateStr)
	orders, err := h.Service.OrderRepo.GetOrders(status, q, dateStr)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// 填充 Items
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

// GetProcurement 获取采购清单
func (h *OrderHandler) GetProcurement(w http.ResponseWriter, r *http.Request) {
	list, err := h.Service.OrderRepo.GetProcurementList()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// Refund 全单退款接口
func (h *OrderHandler) Refund(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "缺少订单ID", 400)
		return
	}

	var orderID int
	fmt.Sscanf(idStr, "%d", &orderID)

	if err := h.Service.RefundOrder(orderID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("退款成功"))
}

// Reprint 补打接口
func (h *OrderHandler) Reprint(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	var orderID int
	fmt.Sscanf(idStr, "%d", &orderID)

	if orderID == 0 {
		http.Error(w, "无效ID", 400)
		return
	}

	if err := h.Service.ReprintTicket(orderID); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("补打指令已发送"))
}

// DoPartialRefund 部分退款接口
func (h *OrderHandler) DoPartialRefund(w http.ResponseWriter, r *http.Request) {
	var req service.PartialRefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "参数错误", 400)
		return
	}
	if err := h.Service.PartialRefund(req); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("退款成功"))
}
