package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Request body dari frontend
type CheckoutRequest struct {
	TransactionCode string         `json:"transaction_code"`
	TotalPaid       float64        `json:"total_paid"`
	Items           []CheckoutItem `json:"items"`
}

type CheckoutItem struct {
	ProductID int     `json:"product_id"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
}

func ProcessCheckout(db *pgx.Conn, req CheckoutRequest) error {
	ctx := context.Background()

	// 1. Mulai Transaksi Database
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	// Pastikan Rollback jika terjadi error di tengah jalan
	defer tx.Rollback(ctx)

	var totalOrder float64
	for _, item := range req.Items {
		totalOrder += item.Price * float64(item.Qty)
	}

	changeAmount := req.TotalPaid - totalOrder

	// 2. Simpan ke tabel TRANSACTIONS
	var transactionID int
	queryTx := `INSERT INTO transactions (transaction_code, total_price, total_paid, change_amount) 
                VALUES ($1, $2, $3, $4) RETURNING id`

	err = tx.QueryRow(ctx, queryTx, req.TransactionCode, totalOrder, req.TotalPaid, changeAmount).Scan(&transactionID)
	if err != nil {
		return fmt.Errorf("gagal simpan header transaksi: %v", err)
	}

	// 3. Loop untuk setiap item
	for _, item := range req.Items {
		subtotal := item.Price * float64(item.Qty)

		// A. Simpan ke TRANSACTION_DETAILS
		queryDetail := `INSERT INTO transaction_details (transaction_id, product_id, qty, price_at_time, subtotal) 
                        VALUES ($1, $2, $3, $4, $5)`
		_, err = tx.Exec(ctx, queryDetail, transactionID, item.ProductID, item.Qty, item.Price, subtotal)
		if err != nil {
			return fmt.Errorf("gagal simpan detail produk %d: %v", item.ProductID, err)
		}

		// B. Update stok di tabel PRODUCTS
		queryUpdateStock := `UPDATE products SET current_stock = current_stock - $1 
                             WHERE id = $2 AND current_stock >= $1`
		res, err := tx.Exec(ctx, queryUpdateStock, item.Qty, item.ProductID)
		if err != nil {
			return err
		}

		// Cek apakah stok cukup (Constraint check)
		if res.RowsAffected() == 0 {
			return fmt.Errorf("stok produk ID %d tidak cukup", item.ProductID)
		}

		// C. (Opsional) Catat ke STOCK_LOGS
		queryLog := `INSERT INTO stock_logs (product_id, type, amount, reason) 
                     VALUES ($1, 'OUT', $2, $3)`
		reason := fmt.Sprintf("Penjualan %s", req.TransactionCode)
		_, _ = tx.Exec(ctx, queryLog, item.ProductID, item.Qty, reason)
	}

	// 4. Commit jika semua langkah di atas berhasil
	return tx.Commit(ctx)
}
