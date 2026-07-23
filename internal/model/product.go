package model

type Product struct {
	ID        int     `json:"id"`
	Barcode   string  `json:"barcode"`
	Name      string  `json:"name"`
	Category  string  `json:"category"`
	Price     float64 `json:"price"`
	CostPrice float64 `json:"cost_price"`
	Stock      int     `json:"stock"`
	Unit       string  `json:"unit"`
	PreOrdered int     `json:"pre_ordered"` // 预订量 (Pending 订单中尚未取走的量)
}
