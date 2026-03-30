package main

import (
	"context"
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
		query := `INSERT INTO products (category_id, name, sku, price, stock) 
                  VALUES ($1, $2, $3, $4, $5) RETURNING id`

		err := conn.QueryRow(context.Background(), query,
			newProduct.CategoryID, newProduct.Name, newProduct.SKU, newProduct.Price, newProduct.Stock,
		).Scan(&newProduct.ID)

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal simpan data"})
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

	r.Run(":8080") // Jalankan server di port 8080
}
