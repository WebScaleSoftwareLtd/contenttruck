package db

import (
	"context"
	"errors"
	"strings"
)

// Partition is used to define information about a partition.
type Partition struct {
	Name       string
	MaxSize    uint32
	PathPrefix string
	Exact      bool
	Validates  string
}

// Join is used to join a path to a partition.
func (p *Partition) Join(relPath string) string {
	if !p.Exact && relPath != "" {
		root := p.PathPrefix
		if !strings.HasSuffix(root, "/") {
			root += "/"
		}
		if strings.HasPrefix(relPath, "/") {
			relPath = relPath[1:]
		}
		if strings.HasPrefix(root, "/") {
			root = root[1:]
		}
		return root + relPath
	}
	return p.PathPrefix
}

const partitionByKey = `
	SELECT partitions.name, partitions.max_size, partitions.path_prefix, partitions.exact, partitions.validates
		FROM keys INNER JOIN partitions ON
			partitions.name = keys.partition WHERE keys.key = $1
`

// GetPartitionsByKey is used to get information partitions by a key.
func (d *DB) GetPartitionsByKey(ctx context.Context, key string) ([]*Partition, error) {
	rows, err := d.conn.Query(ctx, partitionByKey, key)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	s := make([]*Partition, 0)
	for rows.Next() {
		var p Partition
		err = rows.Scan(&p.Name, &p.MaxSize, &p.PathPrefix, &p.Exact, &p.Validates)
		if err != nil {
			return nil, err
		}
		s = append(s, &p)
	}
	return s, nil
}

// Writes to a partitions usage pool. You should know the partition exists beforehand.
// If there is no files and the parition is smaller than the size of the file,
// it will return a not-null constraint error. If there are files and adding this
// file will make the partition too big, it will return no inserts or updates.
const partitionSizeWriteQuery = `
	INSERT INTO partitions_usage AS u (name, size) VALUES
	((SELECT name FROM partitions WHERE name = $1 AND max_size >= $2), $2)
	ON CONFLICT (name) DO UPDATE SET size = u.size + $2
	WHERE (SELECT max_size FROM partitions WHERE name = $1) >= u.size + $2
`

var ErrFileTooLarge = errors.New("Partition size is too small for specified file")

// WriteToPartitionUsagePool writes to a partition's usage pool. Returns ErrFileTooLarge if the
// mapped file is too large.
func (d *DB) WriteToPartitionUsagePool(ctx context.Context, name string, size uint32) error {
	tag, err := d.conn.Exec(ctx, partitionSizeWriteQuery, name, size)
	if err != nil {
		// Check if this is a not-null constraint error.
		if strings.Contains(err.Error(), "violates not-null constraint") {
			return ErrFileTooLarge
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrFileTooLarge
	}
	return nil
}

// RollbackPartitionUsagePool updates a partition's usage pool with the data removed.
func (d *DB) RollbackPartitionUsagePool(ctx context.Context, name string, size uint32) error {
	const query = "UPDATE partitions_usage SET size = size - $1 WHERE name = $2 AND size >= $1"
	_, err := d.conn.Exec(ctx, query, size, name)
	return err
}

// WritePartitionFile writes a file to a partition.
func (d *DB) WritePartitionFile(ctx context.Context, name, path string) error {
	const query = "INSERT INTO partitions_files (name, file_path) VALUES ($1, $2)"
	_, err := d.conn.Exec(ctx, query, name, path)
	return err
}

// ErrPartitionExists is returned when a partition already exists.
var ErrPartitionExists = errors.New("Partition already exists")

// InsertPartition inserts a partition. Returns ErrPartitionExists if the partition already exists.
func (d *DB) InsertPartition(ctx context.Context, p *Partition) error {
	const query = "INSERT INTO partitions (name, max_size, path_prefix, exact, validates) VALUES ($1, $2, $3, $4, $5)"
	_, err := d.conn.Exec(ctx, query, p.Name, p.MaxSize, p.PathPrefix, p.Exact, p.Validates)
	if err != nil {
		if strings.Contains(err.Error(), "violates unique constraint") {
			return ErrPartitionExists
		}
		return err
	}
	return nil
}

// ErrPartitionNotExists is returned when a partition does not exist.
var ErrPartitionNotExists = errors.New("Partition does not exist")

// DeletePartition deletes a partition. Returns ErrPartitionNotExists if the partition does not exist.
func (d *DB) DeletePartition(ctx context.Context, name string) error {
	const query = "DELETE FROM partitions WHERE name = $1"
	res, err := d.conn.Exec(ctx, query, name)
	if err != nil {
		return err
	}
	if res.RowsAffected() == 0 {
		return ErrPartitionNotExists
	}
	return nil
}

// DeletePartitionFiles deletes all the files in a partition and calls the function for each file.
func (d *DB) DeletePartitionFiles(ctx context.Context, name string, iter func(string) error) error {
	const query = "SELECT file_path FROM partitions_files WHERE name = $1"
	rows, err := d.conn.Query(ctx, query, name)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var path string
		err = rows.Scan(&path)
		if err != nil {
			return err
		}
		err = iter(path)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeletePartitionFile deletes a file from a partition.
func (d *DB) DeletePartitionFile(ctx context.Context, name, path string) error {
	const query = "DELETE FROM partitions_files WHERE name = $1 AND file_path = $2"
	_, err := d.conn.Exec(ctx, query, name, path)
	return err
}
