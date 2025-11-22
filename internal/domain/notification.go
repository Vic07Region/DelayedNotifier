package domain

import (
	"time"

	"github.com/google/uuid"
)

type Status string

// String возвращает строковое представление статуса.
func (s Status) String() string {
	return string(s)
}

// IsValid проверяет, является ли статус валидным.
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusProcessing, StatusSent, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

type Channel string

// String возвращает строковое представление канала.
func (c Channel) String() string {
	return string(c)
}

// IsValid проверяет, является ли канал валидным.
func (c Channel) IsValid() bool {
	switch c {
	case ChannelEmail, ChannelTelegram:
		return true
	default:
		return false
	}
}

const (
	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusSent       Status = "sent"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

const (
	ChannelEmail    Channel = "email"
	ChannelTelegram Channel = "telegram"
)

// Notification представляет структуру уведомления.
type Notification struct {
	ID          uuid.UUID
	Recipient   string
	Channel     Channel
	Payload     map[string]interface{}
	ScheduledAt time.Time
	Status      Status
	RetryCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Job представляет структуру задачи для обработки уведомлений.
type Job struct {
	NotificationID string `json:"notification_id"`
}
