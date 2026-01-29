package repository

import (
	"database/sql"
	"time"
)

type ReportRepo struct {
	DB *sql.DB
}

func NewReportRepo(db *sql.DB) *ReportRepo {
	return &ReportRepo{DB: db}
}

// SaleRecord 销售记录结构体
type SaleRecord struct {
	CreatedAt   time.Time
	ProductName string
	Price       float64 // 售价
	CostPrice   float64 // 进价
	Qty         int     // 销售数量
}

// GetSalesData 获取指定时间范围内的所有销售明细
func (r *ReportRepo) GetSalesData(start, end time.Time) ([]SaleRecord, error) {
	// 关联查询：订单 -> 订单明细 -> 商品(为了拿进价)
	// 注意：这里我们用 products 表的 cost_price 近似计算历史成本
	sqlStr := `
		SELECT o.created_at, oi.product_name, oi.price, p.cost_price, oi.qty_picked
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.id
		LEFT JOIN products p ON oi.product_id = p.id
		WHERE o.status = 'Completed' AND o.created_at BETWEEN ? AND ?
		ORDER BY o.created_at ASC
	`
	rows, err := r.DB.Query(sqlStr, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SaleRecord
	for rows.Next() {
		var s SaleRecord
		var cost sql.NullFloat64 // 处理商品可能被删导致进价为空的情况

		err := rows.Scan(&s.CreatedAt, &s.ProductName, &s.Price, &cost, &s.Qty)
		if err != nil {
			continue
		}
		s.CostPrice = cost.Float64
		list = append(list, s)
	}
	return list, nil
}
