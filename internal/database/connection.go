package database

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"github.com/magendooro/magento2-customer-graphql-go/internal/config"
)

func NewConnection(cfg config.DatabaseConfig) (*sql.DB, error) {
	var dsn string
	if cfg.Host == "localhost" && cfg.Socket != "" {
		// Unix socket connection (matches Magento's "localhost" behavior)
		dsn = fmt.Sprintf("%s:%s@unix(%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
			cfg.User, cfg.Password, cfg.Socket, cfg.Name,
		)
	} else if cfg.Host == "localhost" {
		// Try default MySQL socket path
		dsn = fmt.Sprintf("%s:%s@unix(/tmp/mysql.sock)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
			cfg.User, cfg.Password, cfg.Name,
		)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&time_zone=%%27%%2B00%%3A00%%27",
			cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name,
		)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
