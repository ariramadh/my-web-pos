package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"pos-backend/models" // Sesuaikan dengan nama module Anda
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func respondError(c *gin.Context, httpCode, responseCode int, responseMessage string, err error) {
	if err != nil {
		log.Printf("%s: %v", responseMessage, err)
	} else {
		log.Printf("%s", responseMessage)
	}
	c.JSON(httpCode, gin.H{
		"responseCode":    responseCode,
		"responseMessage": responseMessage,
		"data":            nil,
	})
}

func respondSuccess(c *gin.Context, responseCode int, responseMessage string, data interface{}) {
	log.Printf("%s", responseMessage)
	c.JSON(http.StatusOK, gin.H{
		"responseCode":    responseCode,
		"responseMessage": responseMessage,
		"data":            data,
	})
}

func authMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" {
			respondError(c, http.StatusUnauthorized, 401, "Authorization header wajib", nil)
			c.Abort()
			return
		}

		const bearerPrefix = "Bearer "
		if len(auth) <= len(bearerPrefix) || auth[:len(bearerPrefix)] != bearerPrefix {
			respondError(c, http.StatusUnauthorized, 401, "Invalid Authorization scheme", nil)
			c.Abort()
			return
		}

		if auth[len(bearerPrefix):] != token {
			respondError(c, http.StatusUnauthorized, 401, "Invalid token", nil)
			c.Abort()
			return
		}

		c.Next()
	}
}

func main() {
	// 1. Koneksi Database
	connStr := "postgres://postgres:password@localhost:5432/web_pos_db"
	conn, _ := pgx.Connect(context.Background(), connStr)
	defer conn.Close(context.Background())

	r := gin.Default()

	// Tambahkan Middleware CORS ini
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Authorization Bearer Token middleware
	const expectedBearerToken = "your-secret-bearer-token"
	r.Use(authMiddleware(expectedBearerToken))

	// 2. Endpoint API untuk Tambah Produk
	r.POST("/products", func(c *gin.Context) {
		var newProduct models.Product

		// Bind JSON dari request ke struct
		if err := c.ShouldBindJSON(&newProduct); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"responseCode":    400,
				"responseMessage": "Format data salah",
				"data":            nil,
			})
			return
		}

		// 3. Query Insert ke PostgreSQL
		// Query Upsert: Jika SKU sudah ada, update datanya. Jika belum, masukkan data baru.
		query := `
			INSERT INTO products (category_id, name, sku, price, stock) 
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (sku) 
			DO UPDATE SET 
				name = EXCLUDED.name,
				price = EXCLUDED.price,
				stock = EXCLUDED.stock,
				category_id = EXCLUDED.category_id
			RETURNING id`

		err := conn.QueryRow(context.Background(), query,
			newProduct.CategoryID, newProduct.Name, newProduct.SKU, newProduct.Price, newProduct.Stock,
		).Scan(&newProduct.ID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"responseCode":    500,
				"responseMessage": "Gagal menambahkan produk",
				"data":            nil,
			})
			return
		}

		respondSuccess(c, 200, "Sukses", newProduct)
	})

	r.GET("/products/categories", func(c *gin.Context) {
		var categories []models.Category

		rows, err := conn.Query(context.Background(), "SELECT id, name FROM categories")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"responseCode":    500,
				"responseMessage": "Gagal mengambil data kategori",
				"data":            nil,
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var cat models.Category
			err := rows.Scan(&cat.ID, &cat.Name)
			if err != nil {
				continue
			}
			categories = append(categories, cat)
		}

		respondSuccess(c, 200, "Sukses", categories)
	})

	// Endpoint API untuk Menampilkan Semua Produk
	r.GET("/products", func(c *gin.Context) {
		var products []models.Product

		// Query ke Database
		rows, err := conn.Query(context.Background(), "SELECT id, category_id, name, sku, price, stock FROM products")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"responseCode":    500,
				"responseMessage": "Gagal mengambil data",
				"data":            nil,
			})
			return
		}
		defer rows.Close()

		// Looping hasil query ke dalam slice (list)
		for rows.Next() {
			var p models.Product
			err := rows.Scan(&p.ID, &p.CategoryID, &p.Name, &p.SKU, &p.Price, &p.Stock)
			if err != nil {
				continue
			}
			products = append(products, p)
		}

		respondSuccess(c, 200, "Sukses", products)
	})

	// Endpoint API untuk Menampilkan Produk Low Stock (stock < threshold)
	r.GET("/products/low-stock", func(c *gin.Context) {
		var products []models.Product

		threshold := 5
		if ts := c.Query("threshold"); ts != "" {
			if parsed, err := strconv.Atoi(ts); err != nil || parsed < 0 {
				c.JSON(http.StatusBadRequest, gin.H{
					"responseCode":    400,
					"responseMessage": "Threshold harus angka positif",
					"data":            nil,
				})
				return
			} else {
				threshold = parsed
			}
		}

		rows, err := conn.Query(context.Background(), "SELECT id, category_id, name, sku, price, stock FROM products WHERE stock < $1", threshold)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"responseCode":    500,
				"responseMessage": "Gagal mengambil data low stock",
				"data":            nil,
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var p models.Product
			err := rows.Scan(&p.ID, &p.CategoryID, &p.Name, &p.SKU, &p.Price, &p.Stock)
			if err != nil {
				continue
			}
			products = append(products, p)
		}

		respondSuccess(c, 200, "Sukses", products)
	})

	// Transaction untuk kasir (UPDATE stok produk berdasarkan transaksi yang terjadi)
	r.POST("/transactions", func(c *gin.Context) {
		var req struct {
			ProductID int `json:"product_id"`
			Quantity  int `json:"quantity"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"responseCode":    400,
				"responseMessage": "Format data salah",
				"data":            nil,
			})
			return
		}

		// Gunakan Transaction SQL agar stok tidak "balapan" (Race Condition)
		tx, _ := conn.Begin(context.Background())
		defer tx.Rollback(context.Background())

		// 1. Update Stok
		_, err := tx.Exec(context.Background(),
			"UPDATE products SET stock = stock - $1 WHERE id = $2 AND stock >= $1",
			req.Quantity, req.ProductID)

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"responseCode":    400,
				"responseMessage": "Stok tidak cukup atau gagal update",
				"data":            nil,
			})
			return
		}

		tx.Commit(context.Background())
		respondSuccess(c, 200, "Sukses", nil)
	})

	// API Riwayat Transaksi berdasarkan rentang tanggal
	r.GET("/transactions/history", func(c *gin.Context) {
		var transactions []models.Transaction

		start := c.Query("start_date")
		end := c.Query("end_date")
		query := "SELECT id, invoice_num, total_price, total_paid, change_amount, created_at FROM transactions"
		params := []interface{}{}
		where := []string{}

		if start != "" {
			startTime, err := time.Parse("2006-01-02", start)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"responseCode": 400, "responseMessage": "start_date harus format YYYY-MM-DD", "data": nil})
				return
			}
			where = append(where, fmt.Sprintf("created_at >= $%d", len(params)+1))
			params = append(params, startTime)
		}

		if end != "" {
			endTime, err := time.Parse("2006-01-02", end)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"responseCode": 400, "responseMessage": "end_date harus format YYYY-MM-DD", "data": nil})
				return
			}
			endTime = endTime.Add(24*time.Hour - time.Nanosecond)
			where = append(where, fmt.Sprintf("created_at <= $%d", len(params)+1))
			params = append(params, endTime)
		}

		if len(where) > 0 {
			query += " WHERE " + strings.Join(where, " AND ")
		}

		rows, err := conn.Query(context.Background(), query, params...)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"responseCode":    500,
				"responseMessage": "Gagal mengambil history transaksi",
				"data":            nil,
			})
			return
		}
		defer rows.Close()

		for rows.Next() {
			var t models.Transaction
			if err := rows.Scan(&t.ID, &t.InvoiceNum, &t.TotalPrice, &t.TotalPaid, &t.ChangeAmount, &t.CreatedAt); err != nil {
				continue
			}
			transactions = append(transactions, t)
		}

		respondSuccess(c, 200, "Sukses", transactions)
	})

	r.POST("/products/checkout", func(c *gin.Context) {
		var req struct {
			InvoiceNum string  `json:"invoice_num"`
			PaidAmount float64 `json:"paid_amount"`
			Items      []struct {
				ID       int `json:"id"`
				Quantity int `json:"quantity"`
			} `json:"items"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			respondError(c, http.StatusBadRequest, 400, "Format data salah", err)
			return
		}

		if len(req.Items) == 0 {
			respondError(c, http.StatusBadRequest, 400, "Items tidak boleh kosong", nil)
			return
		}

		// Mulai Database Transaction (Atomicity)
		tx, err := conn.Begin(context.Background())
		if err != nil {
			respondError(c, http.StatusInternalServerError, 500, "Gagal memulai transaksi", err)
			return
		}
		defer tx.Rollback(context.Background())

		var totalPrice float64
		var detailInserts []models.TransactionDetail

		for _, item := range req.Items {
			var product models.Product
			if err := tx.QueryRow(context.Background(),
				"SELECT id, category_id, name, sku, price, stock FROM products WHERE id =$1",
				item.ID).Scan(&product.ID, &product.CategoryID, &product.Name, &product.SKU, &product.Price, &product.Stock); err != nil {
				respondError(c, http.StatusBadRequest, 400, fmt.Sprintf("Produk ID %d tidak ditemukan", item.ID), err)
				return
			}

			if product.Stock < item.Quantity {
				respondError(c, http.StatusBadRequest, 400, fmt.Sprintf("Stok produk ID %d tidak mencukupi", item.ID), nil)
				return
			}

			subtotal := float64(item.Quantity) * product.Price
			totalPrice += subtotal

			result, err := tx.Exec(context.Background(),
				"UPDATE products SET stock = stock - $1 WHERE id = $2 AND stock >= $1",
				item.Quantity, item.ID)
			if err != nil {
				respondError(c, http.StatusInternalServerError, 500, "Gagal update stok", err)
				return
			}

			if result.RowsAffected() == 0 {
				respondError(c, http.StatusBadRequest, 400, fmt.Sprintf("Stok produk ID %d tidak mencukupi", item.ID), nil)
				return
			}

			detailInserts = append(detailInserts, models.TransactionDetail{
				ProductID: product.ID,
				Qty:       item.Quantity,
				Subtotal:  subtotal,
			})
		}

		if req.PaidAmount < totalPrice {
			respondError(c, http.StatusBadRequest, 400, "Nominal bayar kurang dari total harga", nil)
			return
		}

		change := req.PaidAmount - totalPrice

		var transactionID int
		if err := tx.QueryRow(context.Background(),
			"INSERT INTO transactions (invoice_num, total_price, total_paid, change_amount) VALUES ($1,$2,$3,$4) RETURNING id",
			req.InvoiceNum, totalPrice, req.PaidAmount, change).Scan(&transactionID); err != nil {
			respondError(c, http.StatusInternalServerError, 500, "Gagal menyimpan transaksi", err)
			return
		}

		for _, detail := range detailInserts {
			if _, err := tx.Exec(context.Background(),
				"INSERT INTO transaction_details (transaction_id, product_id, qty, subtotal) VALUES ($1,$2,$3,$4)",
				transactionID, detail.ProductID, detail.Qty, detail.Subtotal); err != nil {
				respondError(c, http.StatusInternalServerError, 500, "Gagal menyimpan detail transaksi", err)
				return
			}
		}

		if err := tx.Commit(context.Background()); err != nil {
			respondError(c, http.StatusInternalServerError, 500, "Gagal komit transaksi", err)
			return
		}

		respondSuccess(c, 200, "Checkout sukses", gin.H{
			"transaction_id": transactionID,
			"invoice_num":    req.InvoiceNum,
			"total_price":    totalPrice,
			"paid_amount":    req.PaidAmount,
			"change_amount":  change,
		})
	})

	r.Run(":8080") // Jalankan server di port 8080
}
