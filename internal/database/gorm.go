package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func OpenGormSQLite(db *sql.DB) (*gorm.DB, error) {
	gormDB, err := gorm.Open(sqlite.Dialector{Conn: db}, &gorm.Config{})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := applySQLitePragmas(ctx, db); err != nil {
		return nil, err
	}
	return gormDB, nil
}
