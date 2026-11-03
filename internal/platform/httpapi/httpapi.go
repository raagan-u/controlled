// Package httpapi provides the shared JSON boundary for Controlled HTTP handlers.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"go.uber.org/zap"
)

const DefaultMaxBodyBytes int64 = 1 << 20

var ErrInvalidJSON = errors.New("request body must contain one valid JSON object")

type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func ReadJSON(response http.ResponseWriter, request *http.Request, destination any, maxBytes int64) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		Logger(request.Context()).Info("HTTP request JSON rejected", zap.Error(err))
		return errors.Join(ErrInvalidJSON, err)
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("unexpected trailing JSON")
		}
		Logger(request.Context()).Info("HTTP request JSON rejected", zap.Error(err))
		return errors.Join(ErrInvalidJSON, err)
	}
	return nil
}

func WriteError(response http.ResponseWriter, status int, code, message string) {
	if recorder, ok := response.(interface{ recordAPIError(string) }); ok {
		recorder.recordAPIError(code)
	}
	WriteJSON(response, status, ErrorBody{Error: ErrorDetail{Code: code, Message: message}})
}

func WriteJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
