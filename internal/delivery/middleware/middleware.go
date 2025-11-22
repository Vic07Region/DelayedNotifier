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

// CORSMiddleware настроен с логированием для CORS запросов.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			zlog.Logger.Debug().
				Str("event_type", "cors_request").
				Str("origin", origin).
				Str("method", c.Request.Method).
				Msg("CORS preflight request")
		}
		c.Header("Access-Control-Allow-Origin", "http://localhost:63342")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, X-Request-ID, Authorization, X-IJT")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if c.Request.Method == "OPTIONS" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:63342")
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-IJT")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
