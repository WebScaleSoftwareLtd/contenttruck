package httpserver

import (
	"github.com/aws/aws-sdk-go/service/s3"
	"net/http"

	"contenttruck/config"
	"contenttruck/db"
)

// Server is used to define the HTTP server.
type Server struct {
	Config           *config.Config
	DB               *db.DB
	SudoKeyValidator func(string) bool
	S3               *s3.S3
}

// ServeHTTP is used to serve a HTTP request.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" && r.URL.Path == "/_contenttruck" {
		s.api(w, r)
		return
	}
	s.getContent(w, r)
}
