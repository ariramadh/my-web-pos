package main

import (
	"context"
	"fmt"
	"net/http"
	"pos-backend/models" // Sesuaikan dengan nama module Anda

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

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

	// 2. Endpoint API untuk Tambah Produk
	r.POST("/products", func(c *gin.Context) {
		var newProduct models.Product

		// Bind JSON dari request ke struct
		if err := c.ShouldBindJSON(&newProduct); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
			// DEBUG: tampilkan error asli untuk identifikasi cepat
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Produk berhasil ditambahkan!",
			"data":    newProduct,
		})
	})

	// Endpoint API untuk Menampilkan Semua Produk
	r.GET("/products", func(c *gin.Context) {
		var products []models.Product

		// Query ke Database
		rows, err := conn.Query(context.Background(), "SELECT id, category_id, name, sku, price, stock FROM products")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengambil data"})
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

		c.JSON(http.StatusOK, products)
	})

	// Transaction untuk kasir (UPDATE stok produk berdasarkan transaksi yang terjadi)
	r.POST("/transactions", func(c *gin.Context) {
		var req struct {
			ProductID int `json:"product_id"`
			Quantity  int `json:"quantity"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Stok tidak cukup atau gagal update"})
			return
		}

		tx.Commit(context.Background())
		c.JSON(http.StatusOK, gin.H{"message": "Transaksi Berhasil!"})
	})

	r.POST("/products/checkout", func(c *gin.Context) {
		var items []struct {
			ID       int `json:"id"`
			Quantity int `json:"quantity"`
		}

		if err := c.ShouldBindJSON(&items); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Format data salah"})
			return
		}

		// Mulai Database Transaction (Atomicity)
		// Jika satu barang gagal update, semua perubahan akan dibatalkan (rollback)
		tx, err := conn.Begin(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal memulai transaksi"})
			return
		}
		defer tx.Rollback(context.Background())

		for _, item := range items {
			// Query Update Stok: Kurangi stok hanya jika stok cukup (stock >= quantity)
			result, err := tx.Exec(context.Background(),
				"UPDATE products SET stock = stock - $1 WHERE id = $2 AND stock >= $1",
				item.Quantity, item.ID)

			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal update stok"})
				return
			}

			// Cek apakah ada baris yang ter-update
			if result.RowsAffected() == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Stok produk ID %d tidak mencukupi", item.ID)})
				return
			}
		}

		// Commit jika semua lancar
		err = tx.Commit(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan transaksi"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Transaksi berhasil dan stok telah diperbarui!"})
	})

	r.Run(":8080") // Jalankan server di port 8080
}
