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
}

// GetSalesData 获取指定时间范围内的所有销售明细
func (r *ReportRepo) GetSalesData(start, end time.Time) ([]SaleRecord, error) {
	startStr := start.UTC().Format("2006-01-02 15:04:05")
	endStr := end.UTC().Format("2006-01-02 15:04:05")

	sqlStr := `
		SELECT o.id, o.created_at, oi.product_name, oi.price, p.cost_price, oi.qty_picked, COALESCE(oi.qty_refunded, 0)
		FROM order_items oi
		JOIN orders o ON oi.order_id = o.id
		LEFT JOIN products p ON oi.product_id = p.id
		WHERE o.status IN ('Completed', 'Partial') AND o.created_at >= ? AND o.created_at < ?
		ORDER BY o.created_at ASC
	`
	rows, err := r.DB.Query(sqlStr, startStr, endStr)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SaleRecord
	for rows.Next() {
		var s SaleRecord
		var cost sql.NullFloat64
		var qtyPicked, qtyRefunded int

		err := rows.Scan(&s.OrderID, &s.CreatedAt, &s.ProductName, &s.Price, &cost, &qtyPicked, &qtyRefunded)
		if err != nil {
			continue
		}

		s.CostPrice = cost.Float64
		s.Qty = qtyPicked - qtyRefunded

		if s.Qty > 0 {
			list = append(list, s)
		}
	}
	return list, nil
}
