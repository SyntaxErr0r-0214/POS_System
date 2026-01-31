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
	OrderID     int
	CreatedAt   time.Time
	ProductName string
	Price       float64
	CostPrice   float64
	Qty         int
	// 如果你之前在 model 里加了 QtyRefunded 这里可以加，
	// 但为了简单，我们在下面函数里直接处理，这里不需要改结构体
}

// GetSalesData 获取指定时间范围内的所有销售明细
func (r *ReportRepo) GetSalesData(start, end time.Time) ([]SaleRecord, error) {
	// 查询包含已完成(Completed)和部分退款(Partial)的订单
	// 同时查出 qty_refunded 用于计算净销量
	sqlStr := `
		SELECT o.id, o.created_at, oi.product_name, oi.price, p.cost_price, oi.qty_picked, COALESCE(oi.qty_refunded, 0)
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.id
		LEFT JOIN products p ON oi.product_id = p.id
		WHERE o.status IN ('Completed', 'Partial') AND o.created_at BETWEEN ? AND ?
		ORDER BY o.created_at ASC
	`
	rows, err := r.DB.Query(sqlStr, start, end)
	// [修复] 这里必须检查 err
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SaleRecord
	for rows.Next() {
		var s SaleRecord
		var cost sql.NullFloat64
		var qtyPicked, qtyRefunded int

		// Scan 必须对应 SQL 里的字段顺序
		err := rows.Scan(&s.OrderID, &s.CreatedAt, &s.ProductName, &s.Price, &cost, &qtyPicked, &qtyRefunded)
		if err != nil {
			continue
		}

		s.CostPrice = cost.Float64

		// [核心逻辑] 有效销量 = 拿走的 - 退掉的
		s.Qty = qtyPicked - qtyRefunded

		// 只有当有效销量 > 0 时才计入报表
		if s.Qty > 0 {
			list = append(list, s)
		}
	}
	return list, nil
}
