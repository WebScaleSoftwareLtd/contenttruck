package httpserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"contenttruck/db"
	"contenttruck/validations"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
)

// ErrorCode is used to define the error code.
type ErrorCode string

const (
	// ErrorCodeInternalServerError is used when an internal server error occurs.
	ErrorCodeInternalServerError ErrorCode = "internal_server_error"

	// ErrorCodeInvalidType is used when the type is invalid.
	ErrorCodeInvalidType ErrorCode = "invalid_type"

	// ErrorTypeInvalidJSON is used when the JSON is invalid.
	ErrorTypeInvalidJSON ErrorCode = "invalid_json"

	// ErrorCodeInvalidKey is used when the key is invalid.
	ErrorCodeInvalidKey ErrorCode = "invalid_key"

	// ErrorCodeInvalidPath is used when the path is invalid.
	ErrorCodeInvalidPath ErrorCode = "invalid_path"

	// ErrorCodeInvalidPartition is used when the partition is invalid.
	ErrorCodeInvalidPartition ErrorCode = "invalid_partition"

	// ErrorCodeInvalidHeaders is used when the generic HTTP headers are invalid.
	ErrorCodeInvalidHeaders ErrorCode = "invalid_headers"

	// ErrorCodeTooLarge is used when the content is too large.
	ErrorCodeTooLarge ErrorCode = "too_large"

	// ErrorCodeValidationFailed is used when the validation failed.
	ErrorCodeValidationFailed ErrorCode = "validation_failed"

	// ErrorCodePartitionsEmpty is used when the partitions are empty.
	ErrorCodePartitionsEmpty ErrorCode = "partitions_empty"

	// ErrorCodeInvalidRuleSet is used when the rule set is invalid.
	ErrorCodeInvalidRuleSet ErrorCode = "invalid_rule_set"

	// ErrorCodePartitionExists is used when the partition already exists.
	ErrorCodePartitionExists ErrorCode = "partition_exists"
)

// APIError is used to define an API error.
type APIError struct {
	status int

	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

type apiServer struct {
	s *Server
}

func (s *apiServer) getKeys(ctx context.Context, key string) (partitions []*db.Partition, err *APIError) {
	partitions, e1 := s.s.DB.GetPartitionsByKey(ctx, key)
	if e1 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error getting partitions: %s", e1)
		return nil, &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	if len(partitions) == 0 {
		return nil, &APIError{
			status:  http.StatusNotFound,
			Code:    ErrorCodeInvalidKey,
			Message: "Invalid key",
		}
	}

	return partitions, nil
}

// UploadRequest is used to define the upload request.
type UploadRequest struct {
	Key          string `json:"key,omitempty"`
	Partition    string `json:"partition"`
	RelativePath string `json:"relative_path"`
}

// UploadResponse is used to define the upload response.
type UploadResponse struct {
	Size int64 `json:"size"`
}

// Upload is used to upload a file.
func (s *apiServer) Upload(r *http.Request, req *UploadRequest) (*UploadResponse, *APIError) {
	// Get the partitions.
	partitions, err := s.getKeys(r.Context(), req.Key)
	if err != nil {
		return nil, err
	}

	// Get the partition.
	var partition *db.Partition
	for _, p := range partitions {
		if p.Name == req.Partition {
			partition = p
			break
		}
	}

	// Check if the partition was found.
	if partition == nil {
		return nil, &APIError{
			status:  http.StatusNotFound,
			Code:    ErrorCodeInvalidPartition,
			Message: "Partition not found or not associated with key",
		}
	}

	// Create the path based on the partition information.
	p := partition.PathPrefix
	if !partition.Exact && req.RelativePath != "" {
		// Join the path.
		p = path.Join(p, req.RelativePath)
	}

	// Check Content-Length is present.
	if r.ContentLength == -1 {
		return nil, &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorCodeInvalidHeaders,
			Message: "Content-Length header is required",
		}
	}

	// Pre-allocate that amount of space from the partition.
	e2 := s.s.DB.WriteToPartitionUsagePool(
		r.Context(), partition.Name, uint32(r.ContentLength))
	if e2 != nil {
		if e2 == db.ErrFileTooLarge {
			return nil, &APIError{
				status:  http.StatusRequestEntityTooLarge,
				Code:    ErrorCodeTooLarge,
				Message: "File is too large for partition",
			}
		}

		_, _ = fmt.Fprintf(os.Stderr, "Error writing to partition usage pool: %s", e2)
		return nil, &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}
	rollback := true
	defer func() {
		if rollback {
			err := s.s.DB.RollbackPartitionUsagePool(context.Background(), partition.Name, uint32(r.ContentLength))
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error rolling back partition usage pool: %s", err)
			}
		}
	}()

	// Defines the reader. This is because if we need to consume for validation, it will be a different reader.
	defer r.Body.Close()
	var re io.Reader = io.LimitReader(r.Body, r.ContentLength)

	// Pass off to the validations engine if needed.
	if partition.Validates != "" {
		re, e2 = validations.Execute(re, partition.Validates)
		if e2 != nil {
			return nil, &APIError{
				status:  http.StatusBadRequest,
				Code:    ErrorCodeValidationFailed,
				Message: e2.Error(),
			}
		}
	}

	// Create a s3 upload manager.
	uploader := s3manager.NewUploaderWithClient(s.s.S3)

	// Upload the file to S3.
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	acl := "public-read"
	_, e2 = uploader.Upload(&s3manager.UploadInput{
		Bucket:      &s.s.Config.BucketName,
		Key:         &p,
		Body:        re,
		ContentType: &contentType,
		ACL:         &acl,
	})
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error uploading to S3: %s", e2)
		return nil, &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Write the file to the database.
	e2 = s.s.DB.WritePartitionFile(r.Context(), partition.Name, p)
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error writing partition file: %s", e2)
		return nil, &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Do not roll back the usage pool.
	rollback = false

	// Return the response.
	return &UploadResponse{
		Size: r.ContentLength,
	}, nil
}

// DeleteRequest is used to define the delete request.
type DeleteRequest struct {
	Key          string `json:"key"`
	Partition    string `json:"partition"`
	RelativePath string `json:"relative_path"`
}

// Delete is used to delete a file.
func (s *apiServer) Delete(r *http.Request, req *DeleteRequest) *APIError {
	// Get the partitions.
	partitions, err := s.getKeys(r.Context(), req.Key)
	if err != nil {
		return err
	}

	// Get the partition.
	var partition *db.Partition
	for _, p := range partitions {
		if p.Name == req.Partition {
			partition = p
			break
		}
	}

	// Check if the partition was found.
	if partition == nil {
		return &APIError{
			status:  http.StatusNotFound,
			Code:    ErrorCodeInvalidPartition,
			Message: "Partition not found or not associated with key",
		}
	}

	// Create the path based on the partition information.
	p := partition.PathPrefix
	if !partition.Exact && req.RelativePath != "" {
		// Join the path.
		p = path.Join(p, req.RelativePath)
	}

	// Stat the file from S3.
	st, e2 := s.s.S3.HeadObject(&s3.HeadObjectInput{
		Bucket: &s.s.Config.BucketName,
		Key:    &p,
	})
	if e2 != nil {
		// If the file was not found, return a 404.
		if awsErr, ok := e2.(awserr.Error); ok {
			if awsErr.Code() == "NoSuchKey" {
				return &APIError{
					status:  http.StatusNotFound,
					Code:    ErrorCodeInvalidPath,
					Message: "File not found",
				}
			}
		}

		// Otherwise, return a 500.
		_, _ = fmt.Fprintf(os.Stderr, "Error stating in S3: %s", e2)
		return &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Delete the file from S3.
	_, e2 = s.s.S3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: &s.s.Config.BucketName,
		Key:    &p,
	})
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error deleting from S3: %s", e2)
		return &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Delete the file from the database.
	e2 = s.s.DB.DeletePartitionFile(r.Context(), partition.Name, p)
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error deleting partition file: %s", e2)
		return &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Reclaim from the usage pool.
	e2 = s.s.DB.RollbackPartitionUsagePool(r.Context(), partition.Name, uint32(*st.ContentLength))
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error rolling back usage pool: %s", e2)
		return &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Return no errors.
	return nil
}

func (s *apiServer) validateSudoKey(key string) *APIError {
	valid := s.s.SudoKeyValidator(key)
	if !valid {
		return &APIError{
			status:  http.StatusUnauthorized,
			Code:    ErrorCodeInvalidKey,
			Message: "Invalid key",
		}
	}
	return nil
}

// CreateKeyRequest is used to define the create key request.
type CreateKeyRequest struct {
	SudoKey    string   `json:"sudo_key"`
	Partitions []string `json:"partitions"`
}

// CreateKeyResponse is used to define the create key response.
type CreateKeyResponse struct {
	Key string `json:"key"`
}

// CreateKey is used to create a new key.
func (s *apiServer) CreateKey(r *http.Request, req *CreateKeyRequest) (*CreateKeyResponse, *APIError) {
	// Validate the sudo key.
	err := s.validateSudoKey(req.SudoKey)
	if err != nil {
		return nil, err
	}

	// Check if the partitions exist.
	if len(req.Partitions) == 0 {
		return nil, &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorCodePartitionsEmpty,
			Message: "No partitions specified",
		}
	}

	// Generate a random key.
	key := uuid.Must(uuid.NewRandom()).String()

	// Insert the key.
	e2 := s.s.DB.InsertKey(r.Context(), key, req.Partitions)
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error inserting key: %s", e2)
		return nil, &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Return the key.
	return &CreateKeyResponse{Key: key}, nil
}

// DeleteKeyRequest is used to define the delete key request.
type DeleteKeyRequest struct {
	SudoKey string `json:"sudo_key"`
	Key     string `json:"key"`
}

// DeleteKey is used to delete a key.
func (s *apiServer) DeleteKey(r *http.Request, req *DeleteKeyRequest) *APIError {
	// Validate the sudo key.
	err := s.validateSudoKey(req.SudoKey)
	if err != nil {
		return err
	}

	// Delete the key.
	e2 := s.s.DB.DeleteKey(r.Context(), req.Key)
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error deleting key: %s", e2)
		return &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Return success.
	return nil
}

// CreatePartitionRequest is used to define the create partition request.
type CreatePartitionRequest struct {
	SudoKey string `json:"sudo_key"`
	Name    string `json:"name"`
	RuleSet string `json:"rule_set"`
}

const halftb uint32 = 500 * 1024 * 1024

// Parses a string of N b/kb/mb/gb/tb and returns the number of bytes.
func parseSize(s string) (uint32, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) == 0 {
		return 0, fmt.Errorf("empty input")
	}
	var (
		size uint64
		unit string
	)
	for i := len(s) - 1; i >= 0; i-- {
		c := s[i]
		if c >= '0' && c <= '9' {
			sizeStr := s[:i+1]
			var err error
			size, err = strconv.ParseUint(sizeStr, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("invalid size: %v", err)
			}
			unit = s[i+1:]
			break
		}
	}
	switch strings.TrimSpace(unit) {
	case "":
		return uint32(size), nil
	case "b":
		return uint32(size), nil
	case "kb":
		return uint32(size * 1024), nil
	case "mb":
		return uint32(size * 1024 * 1024), nil
	case "gb":
		return uint32(size * 1024 * 1024 * 1024), nil
	case "tb":
		return uint32(size * 1024 * 1024 * 1024 * 1024), nil
	default:
		return 0, fmt.Errorf("invalid size unit: %q", unit)
	}
}

// CreatePartition is used to create a new partition.
func (s *apiServer) CreatePartition(r *http.Request, req *CreatePartitionRequest) *APIError {
	// Validate the sudo key.
	err := s.validateSudoKey(req.SudoKey)
	if err != nil {
		return err
	}

	// Handle if the name is empty.
	if req.Name == "" {
		return &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorCodeInvalidPartition,
			Message: "Partition name is empty",
		}
	}

	// Parse the rule set.
	var p db.Partition
	p.Name = req.Name
	rulesetParts := strings.Split(req.RuleSet, ",")
	for _, v := range rulesetParts {
		// Split the rule.
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		equalsSplit := strings.SplitN(v, "=", 2)
		if len(equalsSplit) != 2 {
			return &APIError{
				status:  http.StatusBadRequest,
				Code:    ErrorCodeInvalidRuleSet,
				Message: "Invalid rule set",
			}
		}

		// Switch on the rule.
		switch equalsSplit[0] {
		case "prefix":
			p.PathPrefix = equalsSplit[1]
		case "exact":
			p.PathPrefix = equalsSplit[1]
			p.Exact = true
		case "max-size":
			maxSize, e2 := parseSize(equalsSplit[1])
			if e2 != nil {
				return &APIError{
					status:  http.StatusBadRequest,
					Code:    ErrorCodeInvalidRuleSet,
					Message: "Invalid rule set",
				}
			}
			p.MaxSize = maxSize
		case "ensure":
			if !validations.Validate(equalsSplit[1]) {
				return &APIError{
					status:  http.StatusBadRequest,
					Code:    ErrorCodeInvalidRuleSet,
					Message: "Invalid rule set",
				}
			}
			p.Validates = equalsSplit[1]
		default:
			return &APIError{
				status:  http.StatusBadRequest,
				Code:    ErrorCodeInvalidRuleSet,
				Message: "Invalid rule set",
			}
		}
	}

	// Validate the ruleset contains a prefix.
	if p.PathPrefix == "" {
		return &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorCodeInvalidRuleSet,
			Message: "Invalid rule set",
		}
	}

	// If max size is not set, set it to the default.
	if p.MaxSize == 0 {
		p.MaxSize = halftb
	}

	// Insert the partition.
	e2 := s.s.DB.InsertPartition(r.Context(), &p)
	if e2 != nil {
		if e2 == db.ErrPartitionExists {
			return &APIError{
				status:  http.StatusBadRequest,
				Code:    ErrorCodePartitionExists,
				Message: "Partition already exists",
			}
		}
		_, _ = fmt.Fprintf(os.Stderr, "Error creating partition: %v", e2)
		return &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Return success.
	return nil
}

// DeletePartitionRequest is used to define the delete partition request.
type DeletePartitionRequest struct {
	SudoKey string `json:"sudo_key"`
	Name    string `json:"name"`
}

// DeletePartition is used to delete a partition.
func (s *apiServer) DeletePartition(r *http.Request, req *DeletePartitionRequest) *APIError {
	// Validate the sudo key.
	err := s.validateSudoKey(req.SudoKey)
	if err != nil {
		return err
	}

	// Delete the partition.
	e2 := s.s.DB.DeletePartition(r.Context(), req.Name)
	if e2 != nil {
		if e2 == db.ErrPartitionNotExists {
			return &APIError{
				status:  http.StatusBadRequest,
				Code:    ErrorCodeInvalidPartition,
				Message: "Partition does not exist",
			}
		}
	}

	// Defines the file handler.
	wg := sync.WaitGroup{}
	hn := func(path string) error {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Call the S3 delete method.
			_, e2 := s.s.S3.DeleteObject(&s3.DeleteObjectInput{
				Bucket: aws.String(s.s.Config.BucketName),
				Key:    aws.String(path),
			})
			if e2 != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error deleting file: %s", e2)
			}
		}()
		return nil
	}

	// Call delete partition files twice.
	for i := 0; i < 2; i++ {
		e2 = s.s.DB.DeletePartitionFiles(r.Context(), req.Name, hn)
		if e2 != nil {
			break
		}
	}

	// Handle any errors.
	if e2 != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error deleting partition files: %s", e2)
		return &APIError{
			status:  http.StatusInternalServerError,
			Code:    ErrorCodeInternalServerError,
			Message: "Internal Server Error",
		}
	}

	// Wait for the file deletions to finish.
	wg.Wait()

	// Return success.
	return nil
}
