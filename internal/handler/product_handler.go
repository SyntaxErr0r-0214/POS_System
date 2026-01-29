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

// SaveResponse 保存接口的返回结构
type SaveResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	ConflictID int    `json:"conflict_id,omitempty"` // 如果重复，返回冲突ID
}

// AddOrUpdate 新增或修改
func (h *ProductHandler) AddOrUpdate(w http.ResponseWriter, r *http.Request) {
	var p model.Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "数据格式错误", 400)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	var resp SaveResponse

	if p.ID > 0 {
		// 编辑模式
		err := h.Inventory.EditProduct(p)
		if err != nil {
			resp = SaveResponse{Success: false, Message: err.Error()}
		} else {
			resp = SaveResponse{Success: true, Message: "修改成功"}
		}
	} else {
		// 新增模式 (包含查重逻辑)
		result, err := h.Inventory.AddProduct(p)
		if err != nil {
			resp = SaveResponse{Success: false, Message: err.Error()}
		} else if result.IsDuplicate {
			// 发现重复，返回特定的ID和消息
			resp = SaveResponse{Success: false, Message: result.Message, ConflictID: result.ExistingID}
		} else {
			resp = SaveResponse{Success: true, Message: "入库成功"}
		}
	}

	json.NewEncoder(w).Encode(resp)
}

// DeleteProduct 删除
func (h *ProductHandler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "无效ID", 400)
		return
	}
	if err := h.Inventory.DeleteProduct(id); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("ok"))
}

// SearchProduct 联想搜索
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

// Procure 批量采购接口
func (h *ProductHandler) Procure(w http.ResponseWriter, r *http.Request) {
	var items []map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	if err := h.Repo.BatchProcure(items); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte("success"))
}
