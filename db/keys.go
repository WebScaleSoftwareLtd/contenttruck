package db

import (
	"context"

	"github.com/jackc/pgx/v4"
)

// InsertKey is used to insert a key.
func (d *DB) InsertKey(ctx context.Context, key string, partitions []string) error {
	const query = "INSERT INTO keys (key, partition) VALUES ($1, $2)"
	batch := pgx.Batch{}
	for _, partition := range partitions {
		batch.Queue(query, key, partition)
	}
	results := d.conn.SendBatch(ctx, &batch)
	defer results.Close()
	for i := 0; i < len(partitions); i++ {
		_, err := results.Exec()
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteKey is used to delete a key.
func (d *DB) DeleteKey(ctx context.Context, key string) error {
	const query = "DELETE FROM keys WHERE key = $1"
	_, err := d.conn.Exec(ctx, query, key)
	return err
}
