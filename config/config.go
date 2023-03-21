package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	SecretAccessKey          string `json:"secret_access_key"`
	AccessKeyID              string `json:"access_key_id"`
	Region                   string `json:"region"`
	BucketName               string `json:"bucket_name"`
	Endpoint                 string `json:"endpoint"`
	SudoKey                  string `json:"sudo_key"`
	HTTPHost                 string `json:"http_host"`
	PostgresConnectionString string `json:"postgres_connection_string"`
}

func loadConfigJson() *Config {
	h, err := os.UserHomeDir()
	if err != nil {
		return &Config{}
	}
	b, err := os.ReadFile(filepath.Join(h, ".contenttruck.json"))
	if err != nil {
		return &Config{}
	}
	var c Config
	err = json.Unmarshal(b, &c)
	if err != nil {
		panic(err)
	}
	return &c
}

type pair[A, B any] struct {
	a0 A
	a1 B
}

func validate(pairs ...pair[string, string]) {
	for _, v := range pairs {
		if v.a1 == "" {
			panic(v.a0 + " is required but not specified")
		}
	}
}

func NewConfig() *Config {
	// Try loading the config from disk.
	conf := loadConfigJson()

	// Load the env variables in.
	e := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if e != "" {
		conf.SecretAccessKey = e
	}
	e = os.Getenv("AWS_ACCESS_KEY_ID")
	if e != "" {
		conf.AccessKeyID = e
	}
	e = os.Getenv("AWS_REGION")
	if e != "" {
		conf.Region = e
	}
	e = os.Getenv("AWS_BUCKET_NAME")
	if e != "" {
		conf.BucketName = e
	}
	e = os.Getenv("AWS_ENDPOINT")
	if e != "" {
		conf.Endpoint = e
	}
	e = os.Getenv("CONTENTTRUCK_SUDO_KEY")
	if e != "" {
		conf.SudoKey = e
	}
	e = os.Getenv("HOST")
	if e != "" {
		conf.HTTPHost = e
	}
	if conf.HTTPHost == "" {
		conf.HTTPHost = "0.0.0.0:6050"
	}
	e = os.Getenv("POSTGRES_CONNECTION_STRING")
	if e != "" {
		conf.PostgresConnectionString = e
	}

	// Validate all the items.
	validate(
		pair[string, string]{"AWS_SECRET_ACCESS_KEY", conf.SecretAccessKey},
		pair[string, string]{"AWS_ACCESS_KEY_ID", conf.AccessKeyID},
		pair[string, string]{"AWS_REGION", conf.Region},
		pair[string, string]{"AWS_BUCKET_NAME", conf.BucketName},
		pair[string, string]{"AWS_ENDPOINT", conf.Endpoint},
		pair[string, string]{"CONTENTTRUCK_SUDO_KEY", conf.SudoKey},
	)
	return conf
}
