package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func handleApiRequest(r *http.Request, s *Server) (any, *APIError) {
	// Get the type.
	type_ := r.Header.Get("X-Type")
	if type_ == "" {
		return nil, &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorCodeInvalidType,
			Message: "X-Type header is required",
		}
	}

	// Create the API server and use reflection to get the handler.
	api := &apiServer{s: s}
	reflectVal := reflect.ValueOf(api)

	// Get the handler.
	handler := reflectVal.MethodByName(type_)
	if !handler.IsValid() {
		return nil, &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorCodeInvalidType,
			Message: "Invalid type",
		}
	}

	// Get the JSON reader either from the body or X-Json-Body.
	re := r.Body
	if r.Header.Get("X-Json-Body") != "" {
		re = io.NopCloser(strings.NewReader(r.Header.Get("X-Json-Body")))
	}

	// Read up to 100KB of JSON.
	b := make([]byte, 100*1024)
	n, err := re.Read(b)
	if err != nil && err != io.EOF {
		return nil, &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorTypeInvalidJSON,
			Message: "Invalid JSON",
		}
	}
	b = b[:n]

	// Get a instance of the first parameters type and decode the JSON into it.
	var v any
	v = reflect.New(handler.Type().In(1).Elem()).Interface()
	if err := json.Unmarshal(b, v); err != nil {
		return nil, &APIError{
			status:  http.StatusBadRequest,
			Code:    ErrorTypeInvalidJSON,
			Message: "Invalid JSON",
		}
	}

	// Call the handler.
	ret := handler.Call([]reflect.Value{
		reflect.ValueOf(r),
		reflect.ValueOf(v),
	})

	// If there is only one return value, return just the error.
	if len(ret) == 1 {
		return nil, ret[0].Interface().(*APIError)
	}

	// Get the last return value which is the error.
	errVal := ret[1].Interface()
	if errVal != (*APIError)(nil) {
		return nil, errVal.(*APIError)
	}

	// Return the first return value.
	return ret[0].Interface(), nil
}

func (s *Server) api(w http.ResponseWriter, r *http.Request) {
	// Render the JSON into the writer.
	var encodeJson func(v any, status int)
	encodeJson = func(v any, status int) {
		// Encode the JSON.
		b, err := json.Marshal(v)
		if err != nil {
			// Write a error response.
			encodeJson(&APIError{
				status:  http.StatusInternalServerError,
				Code:    ErrorCodeInternalServerError,
				Message: "Internal Server Error",
			}, http.StatusInternalServerError)
			_, _ = fmt.Fprintf(os.Stderr, "Error encoding JSON: %s", err.Error())
			return
		}

		// Set headers that will always be sent.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Content-Length", strconv.Itoa(len(b)))

		// Write the status code.
		w.WriteHeader(status)
		_, _ = w.Write(b)
	}

	// Handle recovers.
	defer func() {
		r := recover()
		if r != nil {
			// Write the error.
			encodeJson(&APIError{
				status:  http.StatusInternalServerError,
				Code:    ErrorCodeInternalServerError,
				Message: "Internal Server Error",
			}, http.StatusInternalServerError)

			// Log the error.
			_, _ = fmt.Fprintf(os.Stderr, "Panic: %s", r)
		}
	}()

	// Make the request.
	resp, err := handleApiRequest(r, s)
	if err != nil {
		// Write the error.
		encodeJson(err, err.status)
		return
	}
	if resp == nil {
		// No response.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	encodeJson(resp, http.StatusOK)
}
