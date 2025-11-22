package domain

import (
	"context"
	"time"
)

// RedisRepository интерфейс для работы с Redis.
type RedisRepository interface {
	// Get получает значение по ключу
	Get(ctx context.Context, key string) (string, error)
	// SetWithExpiration устанавливает значение с временем жизни.
	SetWithExpiration(ctx context.Context, key string, value interface{}, expiration time.Duration) error
}
