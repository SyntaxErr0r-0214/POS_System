package handler

import (
	"encoding/json"
	"net/http"
	"pos-demo/internal/repository"
)

type ProductHandler struct {
	Repo *repository.ProductRepo
}

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
