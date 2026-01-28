package model

type Product struct {
	ID        int     `json:"id"`
	Barcode   string  `json:"barcode"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`      // 售价
	CostPrice float64 `json:"cost_price"` // 进价 (新增)
	Stock     int     `json:"stock"`
}
