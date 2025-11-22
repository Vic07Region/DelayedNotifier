package handlers

import (
	"time"

	"github.com/google/uuid"
)

type NotificationResponse struct {
	ID          uuid.UUID              `json:"id"`
	Recipient   string                 `json:"recipient"`
	Channel     string                 `json:"channel"`
	Payload     map[string]interface{} `json:"payload"`
	ScheduledAt time.Time              `json:"scheduled_at"`
	Status      string                 `json:"status"`
	RetryCount  int                    `json:"retry_count"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}
