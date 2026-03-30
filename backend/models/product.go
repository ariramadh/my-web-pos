package models

type Product struct {
	ID         int     `json:"id"`
	CategoryID int     `json:"category_id"`
	Name       string  `json:"name"`
	SKU        string  `json:"sku"`
	Price      float64 `json:"price"`
	Stock      int     `json:"stock"`
}
