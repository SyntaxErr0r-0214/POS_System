package handler

import (
	"encoding/json"
	"net/http"
	"pos-demo/internal/model"
	"pos-demo/internal/repository"
	"pos-demo/internal/service"
	"strconv"
)

type ProductHandler struct {
	Repo      *repository.ProductRepo
	Inventory *service.InventoryService
}

// Scan 扫码
func (h *ProductHandler) Scan(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	p, err := h.Repo.FindByBarcode(code)
	if err != nil {
		http.Error(w, "未找到", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// ListInventory 获取库存列表
func (h *ProductHandler) ListInventory(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	list, err := h.Inventory.GetList(query)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// AddOrUpdate 新增或修改
func (h *ProductHandler) AddOrUpdate(w http.ResponseWriter, r *http.Request) {
	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "数据格式错误", 400)
		return
	}

	var err error
	if p.ID > 0 {
		err = h.Inventory.EditProduct(p)
	} else {
		err = h.Inventory.AddProduct(p)
	}

	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Write([]byte("操作成功"))
}

// DeleteProduct 删除商品接口 (新增)
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "无效ID", 400)
		return
	}

	if err := h.Inventory.DeleteProduct(id); err != nil {
		// 这里如果报错，通常是因为有外键关联（即该商品有历史订单）
		http.Error(w, "无法删除：该商品可能已有销售记录", 500)
		return
	}
	w.Write([]byte("ok"))
}

// SearchProduct 收银台联想搜索
func (h *ProductHandler) SearchProduct(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		w.Write([]byte("[]"))
		return
	}

	list, err := h.Repo.Search(q)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
