package api

import (
	"fmt"
	"net/http"
	"spendings-backend/internal/models"
	"time"

	"go.uber.org/zap"
)

type responseCapture struct {
	writer     http.ResponseWriter
	statusCode int
}

func (resp *responseCapture) Write(body []byte) (int, error) {
	return resp.writer.Write(body)
}

func (resp *responseCapture) WriteHeader(statusCode int) {
	if resp.statusCode == 0 { // for write status code only once
		resp.statusCode = statusCode
		resp.writer.WriteHeader(statusCode)
	}
}

func (resp *responseCapture) Header() http.Header {
	return resp.writer.Header()
}

type Middleware struct {
	logger *zap.SugaredLogger
}

func NewLoggerMiddleware(logger *zap.SugaredLogger) *Middleware {
	return &Middleware{
		logger: logger,
	}
}

func (lm *Middleware) Middleware(next http.HandlerFunc) http.HandlerFunc {
	return func(response http.ResponseWriter, req *http.Request) {
		// Create a custom response writer
		responseWriter := &responseCapture{writer: response}

		startTime := time.Now()

		// Process request
		next.ServeHTTP(responseWriter, req)

		// Get response details
		statusCode := responseWriter.statusCode
		if statusCode == 0 {
			statusCode = http.StatusOK
		}

		method := req.Method
		path := req.URL.Path
		userAgent := req.UserAgent()
		host := req.Host

		// Write the sanitized response body to the response writer
		responseWriter.WriteHeader(statusCode)

		// Calculate latency in milliseconds
		latency := time.Since(startTime).Seconds() * 1000

		// Log details in JSON format
		lm.logger.With(
			"method", method,
			"status_code", statusCode,
			"path", path,
			"user_agent", userAgent,
			"host", host,
			"latency_ms", fmt.Sprintf("%.4fms", latency),
			"username", models.ClaimsFromContext(req.Context()).Nickname,
		).Infof("Request handeled")
	}
}
