package main

import (
	"crypto/subtle"
	"fmt"
	"net/http"

	"contenttruck/config"
	"contenttruck/db"
	"contenttruck/httpserver"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func isSudoKey(key string) func(string) bool {
	keyB := []byte(key)
	return func(s string) bool {
		return subtle.ConstantTimeCompare(keyB, []byte(s)) == 1
	}
}

func main() {
	// Display the log.
	fmt.Println("Contenttruck. Copyright (C) 2023 Web Scale Software Ltd.")

	// Load the config.
	conf := config.NewConfig()

	// Get the key comparer.
	comparer := isSudoKey(conf.SudoKey)
	conf.SudoKey = ""

	// Connect to the database.
	conn := db.NewDB(conf.PostgresConnectionString)
	conf.PostgresConnectionString = ""

	// Initialise the S3 client.
	sess := session.Must(session.NewSessionWithOptions(
		session.Options{
			Config: aws.Config{
				Endpoint: aws.String(conf.Endpoint),
				Region:   aws.String(conf.Region),
				Credentials: credentials.NewStaticCredentials(
					conf.AccessKeyID, conf.SecretAccessKey, ""),
			},
		}))
	s3Client := s3.New(sess)
	conf.AccessKeyID = ""
	conf.SecretAccessKey = ""
	conf.Region = ""
	conf.Endpoint = ""

	// Create the HTTP server and listen.
	s := &httpserver.Server{
		Config:           conf,
		DB:               conn,
		SudoKeyValidator: comparer,
		S3:               s3Client,
	}
	err := http.ListenAndServe(conf.HTTPHost, h2c.NewHandler(s, &http2.Server{}))
	if err != nil {
		panic(err)
	}
}
