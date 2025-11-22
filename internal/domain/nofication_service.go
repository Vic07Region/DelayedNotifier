package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NotificationService интерфейс для работы с уведомлениями.
type NotificationService interface {
	// CreateNotification создает новое уведомление
	CreateNotification(ctx context.Context,
		params CreateNotificationParams) (*Notification, error)
	// UpdateNotification обновляет уведомление с указанными параметрами
	UpdateNotification(ctx context.Context, n *Notification, opts ...UpdateOption) error
	// GetNotificationByID получает уведомление по ID
	GetNotificationByID(ctx context.Context, id uuid.UUID) (*Notification, error)
	// Cancel отменяет уведомление (статус pending -> cancelled)
	Cancel(ctx context.Context, id uuid.UUID) error
	// Failed помечает уведомление как неуспешное (статус processing -> failed)
	Failed(ctx context.Context, id uuid.UUID) error
	// IncRetryCount увеличивает счетчик попыток для уведомления
	IncRetryCount(ctx context.Context, n *Notification) error
}

// CreateNotificationParams параметры для создания уведомления.
type CreateNotificationParams struct {
	Recipient   string
	Channel     Channel
	Payload     map[string]interface{}
	ScheduledAt time.Time
}
