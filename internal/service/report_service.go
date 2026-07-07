package service

import (
	"modern-pos/internal/repository"
	"sort"
	"time"
)

type ReportService struct {
	Repo *repository.ReportRepo
}

func NewReportService(repo *repository.ReportRepo) *ReportService {
	return &ReportService{Repo: repo}
}

// ReportResponse 返回结构
type ReportResponse struct {
	TotalRevenue float64   `json:"total_revenue"`
	TotalProfit  float64   `json:"total_profit"`
	OrderCount   int       `json:"order_count"`
	ChartLabels  []string  `json:"chart_labels"`
	ChartData    []float64 `json:"chart_data"`
	PieLabels    []string  `json:"pie_labels"`
	PieData      []float64 `json:"pie_data"`
}

// GenerateReport 生成经营分析报表
func (s *ReportService) GenerateReport(timeType string) (ReportResponse, error) {
	now := time.Now()
	var start, end time.Time

	switch timeType {
	case "day":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		end = start.Add(24 * time.Hour)
	case "week":
		offset := int(time.Monday - now.Weekday())
		if offset > 0 {
			offset = -6
		}
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).AddDate(0, 0, offset)
		end = start.AddDate(0, 0, 7)
	case "month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
		end = start.AddDate(0, 1, 0)
	case "year":
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.Local)
		end = start.AddDate(1, 0, 0)
	}

	records, err := s.Repo.GetSalesData(start, end)
	if err != nil {
		return ReportResponse{}, err
	}

	var resp ReportResponse
	uniqueOrderIDs := make(map[int]bool)
	timeGroup := make(map[string]float64)
	productGroup := make(map[string]float64)

	for _, r := range records {
		uniqueOrderIDs[r.OrderID] = true
		revenue := r.Price * float64(r.Qty)

		cost := r.CostPrice
		if r.Price == 0 {
			cost = 0
		}

		profit := (r.Price - cost) * float64(r.Qty)

		resp.TotalRevenue += revenue
		resp.TotalProfit += profit
		productGroup[r.ProductName] += float64(r.Qty)

		var key string
		switch timeType {
		case "day":
			key = r.CreatedAt.Format("15:00")
		case "week", "month":
			key = r.CreatedAt.Format("01-02")
		case "year":
			key = r.CreatedAt.Format("2006-01")
		}
		timeGroup[key] += revenue
	}

	resp.OrderCount = len(uniqueOrderIDs)

	var keys []string
	for k := range timeGroup {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		resp.ChartLabels = append(resp.ChartLabels, k)
		resp.ChartData = append(resp.ChartData, timeGroup[k])
	}

	// 饼图
	type kv struct {
		Key   string
		Value float64
	}
	var ss []kv
	for k, v := range productGroup {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].Value > ss[j].Value })

	for i, item := range ss {
		if i >= 5 {
			break
		}
		resp.PieLabels = append(resp.PieLabels, item.Key)
		resp.PieData = append(resp.PieData, item.Value)
	}

	return resp, nil
}
