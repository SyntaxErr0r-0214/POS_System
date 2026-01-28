package handler

import (
	"encoding/json"
	"net/http"
	"pos-demo/internal/model"
	"pos-demo/internal/repository"
	"pos-demo/internal/service" // 引入 service 包
)

type ProductHandler struct {
	Repo      *repository.ProductRepo
	Inventory *service.InventoryService // 注入 InventoryService
}

// Scan 方法保持不变
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

// ListInventory 获取库存列表 API
func (h *ProductHandler) ListInventory(w http.ResponseWriter, r *http.Request) {
	list, err := h.Inventory.GetList()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// AddOrUpdate 新增或修改 API
func (h *ProductHandler) AddOrUpdate(w http.ResponseWriter, r *http.Request) {
	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "数据格式错误", 400)
		return
	}

	var err error
	if p.ID > 0 {
		// 有ID是修改
		err = h.Inventory.EditProduct(p)
	} else {
		// 没ID是新增
		err = h.Inventory.AddProduct(p)
	}

	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Write([]byte("操作成功"))
}

// SearchProduct 联想搜索 API
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
