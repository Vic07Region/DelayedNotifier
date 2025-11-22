package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/wb-go/wbf/zlog"
)

// RequestIDMiddleware добавляет уникальный ID для каждого запроса.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// LoggingMiddleware логирует входящие HTTP запросы и ответы.
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		requestID, exists := c.Get("request_id")
		if !exists {
			requestID = "unknown"
		}

		zlog.Logger.Info().
			Str("request_id", requestID.(string)).
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("user_agent", c.Request.UserAgent()).
			Str("remote_addr", c.ClientIP()).
			Int("content_length", int(c.Request.ContentLength)).
			Time("start_time", start).
			Msg("HTTP request started")

		c.Next()

		duration := time.Since(start)

		var logLevel string
		switch {
		case c.Writer.Status() >= 500:
			logLevel = "error"
		case c.Writer.Status() >= 400:
			logLevel = "warn"
		default:
			logLevel = "info"
		}

		switch logLevel {
		case "error":
			zlog.Logger.Error().
				Str("request_id", requestID.(string)).
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Int("status_code", c.Writer.Status()).
				Int("response_size", c.Writer.Size()).
				Dur("duration", duration).
				Str("error", c.Errors.String()).
				Msg("HTTP request completed with error")
		case "warn":
			zlog.Logger.Warn().
				Str("request_id", requestID.(string)).
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Int("status_code", c.Writer.Status()).
				Int("response_size", c.Writer.Size()).
				Dur("duration", duration).
				Msg("HTTP request completed with warning")
		default:
			zlog.Logger.Info().
				Str("request_id", requestID.(string)).
				Str("method", c.Request.Method).
				Str("path", c.Request.URL.Path).
				Int("status_code", c.Writer.Status()).
				Int("response_size", c.Writer.Size()).
				Dur("duration", duration).
				Msg("HTTP request completed successfully")
		}
	}
}
