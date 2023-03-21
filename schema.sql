CREATE TABLE IF NOT EXISTS partitions (
    name VARCHAR(255) NOT NULL PRIMARY KEY,
    max_size INTEGER NOT NULL,
    path_prefix TEXT NOT NULL,
    exact BOOLEAN NOT NULL,
    validates TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS partitions_files (
    name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    PRIMARY KEY (name, file_path)
    -- Intentionally no foreign key to partitions(name) because we want to
    -- start purging files from partitions that no longer exist.
);

CREATE INDEX IF NOT EXISTS partitions_file_name ON partitions_files (name);

CREATE TABLE IF NOT EXISTS partitions_usage (
    name TEXT NOT NULL PRIMARY KEY,
    size INTEGER NOT NULL,
    FOREIGN KEY (name) REFERENCES partitions(name) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS keys (
    key TEXT NOT NULL,
    partition TEXT NOT NULL,
    FOREIGN KEY (partition) REFERENCES partitions(name) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS keys_key ON keys (key);
