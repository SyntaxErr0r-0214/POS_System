package handler

import (
	"encoding/json"
	"net/http"
	"pos-demo/internal/service"
)

type ReportHandler struct {
	Service *service.ReportService
}

func (h *ReportHandler) GetReport(w http.ResponseWriter, r *http.Request) {
	timeType := r.URL.Query().Get("type")
	if timeType == "" {
		timeType = "day"
	}

	data, err := h.Service.GenerateReport(timeType)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
