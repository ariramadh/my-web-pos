package config

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func ConnectDB() *pgx.Conn {
	// Sesuaikan: user:password@host:port/dbname
	connStr := "postgres://postgres:PASSWORD_ANDA@localhost:5432/web_pos_db"
	conn, err := pgx.Connect(context.Background(), connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Gagal koneksi ke database: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Berhasil terhubung ke PostgreSQL!")
	return conn
}
