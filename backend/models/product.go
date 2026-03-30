package models

import "time"

type Product struct {
	ID         int     `json:"id"`
	CategoryID int     `json:"category_id"`
	Name       string  `json:"name"`
	SKU        string  `json:"sku"`
	Price      float64 `json:"price"`
	Stock      int     `json:"stock"`
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Transaction struct {
	ID           int     `json:"id"`
	InvoiceNum   string  `json:"invoice_num"`
	TotalPrice   float64 `json:"total_price"`
	TotalPaid    float64 `json:"paid_amount"`
	ChangeAmount float64 `json:"change_amount"`
	// CreatedAt represents the timestamp when the product record was created in the database.
	// It is automatically set by the database server when a new product is inserted.
	CreatedAt time.Time `json:"created_at"`
}

type TransactionDetail struct {
	ID            int     `json:"id"`
	TransactionID int     `json:"transaction_id"`
	ProductID     int     `json:"product_id"`
	Qty           int     `json:"qty"`
	Subtotal      float64 `json:"subtotal"`
}
