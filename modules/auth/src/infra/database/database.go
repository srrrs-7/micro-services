package database

import (
	"context"
	"database/sql"

	_ "github.com/lib/pq"
)

func NewDB(uri string) (*sql.DB, error) {
	db, err := sql.Open("postgres", uri)
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(context.Background()); err != nil {
		return nil, err
	}

	return db, nil
}
