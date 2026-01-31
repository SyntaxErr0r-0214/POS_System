package service

import (
	"pos-demo/internal/repository"
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

// GenerateReport 生成报表
func (s *ReportService) GenerateReport(timeType string) (ReportResponse, error) {
	now := time.Now()
	var start, end time.Time

	// 1. 确定时间范围 (逻辑保持不变)
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

	// 2. 聚合数据
	var resp ReportResponse

	// [新增] 用于订单去重的 Map
	uniqueOrderIDs := make(map[int]bool)

	timeGroup := make(map[string]float64)
	productGroup := make(map[string]float64)

	for _, r := range records {
		// 记录订单ID以便统计总单数
		uniqueOrderIDs[r.OrderID] = true

		revenue := r.Price * float64(r.Qty)

		// [优化] 利润计算逻辑：
		// 如果售价为0 (赠品/临时商品)，强制忽略成本，避免出现负利润
		// 如果你希望真实反映亏损，可以把这个 if 去掉
		cost := r.CostPrice
		if r.Price == 0 {
			cost = 0
		}

		profit := (r.Price - cost) * float64(r.Qty)

		resp.TotalRevenue += revenue
		resp.TotalProfit += profit

		// 饼图数据
		productGroup[r.ProductName] += float64(r.Qty)

		// 条形图数据
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

	// [修正] 真正的订单数 = 去重后的ID数量
	resp.OrderCount = len(uniqueOrderIDs)

	// 3. 整理图表数据 (保持原逻辑)
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
