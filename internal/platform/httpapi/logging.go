package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

const RequestIDHeader = "X-Request-ID"

type loggerContextKey struct{}

// Logger returns the request-scoped logger, falling back to Zap's global logger.
func Logger(ctx context.Context) *zap.Logger {
	if logger, ok := ctx.Value(loggerContextKey{}).(*zap.Logger); ok && logger != nil {
		return logger
	}
	return zap.L()
}

// WithLogger stores a structured logger in a context for downstream operations.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// WithFields enriches the logger already associated with a request context.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return WithLogger(ctx, Logger(ctx).With(fields...))
}

// LogRequests records one structured completion event for every HTTP request and
// turns panics into a JSON 500 response while preserving their stack trace.
func LogRequests(logger *zap.Logger, next http.Handler) http.Handler {
	if logger == nil {
		logger = zap.L()
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		requestID := requestID(request.Header.Get(RequestIDHeader))
		response.Header().Set(RequestIDHeader, requestID)
		requestLogger := logger.With(
			zap.String("request_id", requestID),
			zap.String("http.method", request.Method),
			zap.String("http.path", request.URL.Path),
			zap.String("client.address", clientAddress(request.RemoteAddr)),
		)
		recorder := &responseRecorder{ResponseWriter: response, status: http.StatusOK}
		request = request.WithContext(WithLogger(request.Context(), requestLogger))

		defer func() {
			if recovered := recover(); recovered != nil {
				requestLogger.Error("HTTP handler panicked",
					zap.Any("panic", recovered),
					zap.ByteString("stack", debug.Stack()),
				)
				if !recorder.wroteHeader {
					WriteError(recorder, http.StatusInternalServerError, "internal_error", "Controlled could not complete the request.")
				}
			}

			fields := []zap.Field{
				zap.Int("http.status", recorder.status),
				zap.Int64("http.response_bytes", recorder.bytes),
				zap.Duration("duration", time.Since(started)),
			}
			if recorder.apiErrorCode != "" {
				fields = append(fields, zap.String("error.code", recorder.apiErrorCode))
			}
			switch {
			case recorder.status >= http.StatusInternalServerError:
				requestLogger.Error("HTTP request completed", fields...)
			case recorder.status >= http.StatusBadRequest:
				requestLogger.Warn("HTTP request completed", fields...)
			default:
				requestLogger.Info("HTTP request completed", fields...)
			}
		}()

		next.ServeHTTP(recorder, request)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status       int
	bytes        int64
	wroteHeader  bool
	apiErrorCode string
}

func (recorder *responseRecorder) WriteHeader(status int) {
	if recorder.wroteHeader {
		return
	}
	recorder.status = status
	recorder.wroteHeader = true
	recorder.ResponseWriter.WriteHeader(status)
}

func (recorder *responseRecorder) Write(body []byte) (int, error) {
	if !recorder.wroteHeader {
		recorder.WriteHeader(http.StatusOK)
	}
	written, err := recorder.ResponseWriter.Write(body)
	recorder.bytes += int64(written)
	return written, err
}

// Unwrap lets http.ResponseController reach optional interfaces on the original writer.
func (recorder *responseRecorder) Unwrap() http.ResponseWriter { return recorder.ResponseWriter }

func (recorder *responseRecorder) recordAPIError(code string) { recorder.apiErrorCode = code }

var fallbackRequestID atomic.Uint64

func requestID(provided string) string {
	if validRequestID(provided) {
		return provided
	}
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return time.Now().UTC().Format("20060102T150405.000000000") + "-" + stringID(fallbackRequestID.Add(1))
}

func validRequestID(value string) bool {
	if len(value) < 1 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < 'A' || character > 'Z') &&
			(character < '0' || character > '9') && character != '-' && character != '_' && character != '.' {
			return false
		}
	}
	return true
}

func clientAddress(remoteAddress string) string {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err == nil {
		return host
	}
	return remoteAddress
}

func stringID(value uint64) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	position := len(buffer)
	for value > 0 {
		position--
		buffer[position] = digits[value%10]
		value /= 10
	}
	return string(buffer[position:])
}
