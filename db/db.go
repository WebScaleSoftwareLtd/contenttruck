package db

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
)

type DB struct {
	conn *pgxpool.Pool
}

func NewDB(connString string) *DB {
	conn, err := pgxpool.Connect(context.Background(), connString)
	if err != nil {
		panic(err)
	}
	return &DB{conn: conn}
}
