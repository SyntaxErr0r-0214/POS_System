package model

// Product 商品模型
type Product struct {
	ID      int     `json:"id"`
	Barcode string  `json:"barcode"`
	Name    string  `json:"name"`
	Price   float64 `json:"price"`
	Stock   int     `json:"stock"`
}
