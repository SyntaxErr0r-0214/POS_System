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

// ReportResponse 返回给前端的数据结构
type ReportResponse struct {
	TotalRevenue float64   `json:"total_revenue"`
	TotalProfit  float64   `json:"total_profit"`
	OrderCount   int       `json:"order_count"`
	ChartLabels  []string  `json:"chart_labels"` // 图表X轴 (时间)
	ChartData    []float64 `json:"chart_data"`   // 图表Y轴 (金额)
	PieLabels    []string  `json:"pie_labels"`   // 饼图标签 (商品名)
	PieData      []float64 `json:"pie_data"`     // 饼图数据 (销量)
}

// GenerateReport 生成报表
// type: "day", "week", "month", "year"
func (s *ReportService) GenerateReport(timeType string) (ReportResponse, error) {
	now := time.Now()
	var start, end time.Time

	// 1. 确定时间范围
	switch timeType {
	case "day":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
		end = start.Add(24 * time.Hour)
	case "week":
		// 找本周一
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
	resp.OrderCount = len(records) // 注：这里简单用明细数近似，严谨应查order表

	// 临时Map用于聚合
	timeGroup := make(map[string]float64)
	productGroup := make(map[string]float64)

	for _, r := range records {
		revenue := r.Price * float64(r.Qty)
		profit := (r.Price - r.CostPrice) * float64(r.Qty)

		resp.TotalRevenue += revenue
		resp.TotalProfit += profit

		// 饼图：按商品名聚合销量
		productGroup[r.ProductName] += float64(r.Qty)

		// 条形图：按时间聚合
		var key string
		switch timeType {
		case "day":
			key = r.CreatedAt.Format("15:00") // 按小时
		case "week", "month":
			key = r.CreatedAt.Format("01-02") // 按日期
		case "year":
			key = r.CreatedAt.Format("2006-01") // 按月份
		}
		timeGroup[key] += revenue
	}

	// 3. 整理图表数据 (排序)
	// 条形图
	var keys []string
	for k := range timeGroup {
		keys = append(keys, k)
	}
	sort.Strings(keys) // 时间排序
	for _, k := range keys {
		resp.ChartLabels = append(resp.ChartLabels, k)
		resp.ChartData = append(resp.ChartData, timeGroup[k])
	}

	// 饼图 (取销量前5名)
	type kv struct {
		Key   string
		Value float64
	}
	var ss []kv
	for k, v := range productGroup {
		ss = append(ss, kv{k, v})
	}
	sort.Slice(ss, func(i, j int) bool { return ss[i].Value > ss[j].Value }) // 降序

	for i, item := range ss {
		if i >= 5 {
			break
		} // 只取前5
		resp.PieLabels = append(resp.PieLabels, item.Key)
		resp.PieData = append(resp.PieData, item.Value)
	}

	return resp, nil
}
