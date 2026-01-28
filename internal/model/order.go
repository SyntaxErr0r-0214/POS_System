package model

import "time"

// Order 订单主表
type Order struct {
	ID           int         `json:"id"`
	CustomerName string      `json:"customer_name"`
	Phone        string      `json:"phone"`
	Status       string      `json:"status"`
	CreatedAt    time.Time   `json:"created_at"`
	Items        []OrderItem `json:"items,omitempty"`
}

// OrderItem 订单明细
type OrderItem struct {
	ID          int     `json:"id"`
	OrderID     int     `json:"order_id"`
	ProductID   int     `json:"product_id"`
	ProductName string  `json:"product_name"`
	Price       float64 `json:"price"`
	QtyOrdered  int     `json:"qty_ordered"`
	QtyPicked   int     `json:"qty_picked"`
}

// CheckoutRequest 结算请求
type CheckoutRequest struct {
	Items []struct {
		ID    int     `json:"id"`
		Qty   int     `json:"qty"`
		Price float64 `json:"price"` // <--- 关键：必须要有这个字段，不然 Service 层会报错
	} `json:"items"`
}

// BookingRequest 预订请求
type BookingRequest struct {
	CustomerName string `json:"customer_name"`
	Phone        string `json:"phone"`
	Items        []struct {
		ID  int `json:"id"`
		Qty int `json:"qty"`
	} `json:"items"`
}

// PickupRequest 提货请求
type PickupRequest struct {
	OrderID int `json:"order_id"`
	Items   []struct {
		ItemID int `json:"item_id"`
		Qty    int `json:"qty"`
	} `json:"items"`
}
