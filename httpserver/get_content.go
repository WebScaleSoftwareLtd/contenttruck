package httpserver

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/disintegration/imaging"
)

func parseInt(s string) uint64 {
	i, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func default_[T any](x T, ptr *T) T {
	if ptr == nil {
		return x
	}
	return *ptr
}

func (s *Server) getContent(w http.ResponseWriter, r *http.Request) {
	// Handle if this is a OPTIONS request.
	if r.Method == "OPTIONS" {
		supportedMethods := "OPTIONS, GET"
		if r.URL.Path == "/_contenttruck" {
			supportedMethods += ", POST"
		}
		w.Header().Set("Access-Control-Allow-Methods", supportedMethods)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-Json-Body, X-Type")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Max-Age", "600")
		w.Header().Set("Content-Length", "0")
		w.Header().Set("Cache-Control", "max-age=600")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Get the bucket key.
	bucketKey := r.URL.Path[1:]

	// Handle blank key.
	if bucketKey == "" {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
		return
	}

	// Get from the bucket using the AWS SDK.
	resp, err := s.S3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.Config.BucketName),
		Key:    aws.String(bucketKey),
	})

	// Check if it was not found. Explicitly check the error type.
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "NoSuchKey" {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte("Not Found"))
				return
			}
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
		_, _ = fmt.Fprintf(os.Stderr, "Error getting object %s from S3: %s", bucketKey, err.Error())
		return
	}

	// Ensure the body gets closed.
	defer resp.Body.Close()

	// Check if the w and/or h query parameters are set.
	wParam := parseInt(r.URL.Query().Get("w"))
	hParam := parseInt(r.URL.Query().Get("h"))

	// Make sure the w and h parameters are not too large.
	if wParam > 10000 || hParam > 10000 {
		// Return a bad request.
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("w and h parameters must be less than 10000"))
		return
	}

	// If the w and h query parameters are set, then we need to try and resize the possible image whilst
	// being efficient and preventing a DoS attack.
	if wParam != 0 && hParam != 0 {
		// Try and read the image.
		img, err := imaging.Decode(io.LimitReader(resp.Body, 1024*1024*20))
		if err != nil {
			// Return a bad request.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("Could not load as image"))
			_, _ = fmt.Fprintf(os.Stderr, "Error decoding image %s: %s", bucketKey, err.Error())
			return
		}

		// Resize the image.
		img = imaging.Resize(img, int(wParam), int(hParam), imaging.Lanczos)

		// Write the image to the response.
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = imaging.Encode(w, img, imaging.PNG)
		return
	}

	// Set all the headers.
	w.Header().Set("Content-Type", default_("application/octet-stream", resp.ContentType))
	w.Header().Set("Cache-Control", "max-age=3600")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if resp.ContentLength != nil {
		w.Header().Set("Content-Length", strconv.FormatInt(*resp.ContentLength, 10))
	}

	// Copy the body to the response.
	_, _ = io.Copy(w, resp.Body)
}
