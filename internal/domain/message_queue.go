package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// MessageQueuePublisher интерфейс для публикации сообщений в очередь.
type MessageQueuePublisher interface {
	// Publish публикует сообщение в очередь с указанным TTL
	Publish(ctx context.Context, id uuid.UUID, ttl time.Duration) error
}
