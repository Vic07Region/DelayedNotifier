package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// NotificationRepository интерфейс для работы с уведомлениями в базе данных.
type NotificationRepository interface {
	// Create создает новое уведомление
	Create(ctx context.Context, n CreateParams) (*Notification, error)
	// GetByID получает уведомление по ID
	GetByID(ctx context.Context, id uuid.UUID) (*Notification, error)
	// Update обновляет уведомление с указанными параметрами
	Update(ctx context.Context, id uuid.UUID, opts ...UpdateOption) error
	// ListPendingAndProcessingBefore получает список зависших уведомлений
	// (статус pending или processing, обновленных до указанного времени)
	// Если limit или offset равны 0, они не включаются в запрос
	ListPendingAndProcessingBefore(ctx context.Context, t time.Time, limit, offset int) ([]Notification, error)
	// PendingToProcess изменяет статус уведомления с pending на processing
	PendingToProcess(ctx context.Context, id uuid.UUID) (bool, error)
	// IncRetryCount увеличивает счетчик попыток для уведомления
	IncRetryCount(ctx context.Context, id uuid.UUID) error
}

// CreateParams параметры для создания уведомления.
type CreateParams struct {
	Recipient   string
	Channel     Channel
	Status      Status
	Payload     map[string]interface{}
	ScheduledAt time.Time
}

// UpdateOption функция для обновления параметров уведомления.
type UpdateOption func(*UpdateParams)

// OptionalPayload обертка для опционального payload.
type OptionalPayload struct {
	Value map[string]interface{}
	Set   bool
}

// UpdateParams параметры для обновления уведомления.
type UpdateParams struct {
	Status        *Status
	RetryCountInc *bool
	ScheduledAt   *time.Time
	Channel       *Channel
	Payload       *OptionalPayload
}

// WithStatus создает опцию для установки статуса уведомления.
func WithStatus(status Status) UpdateOption {
	return func(p *UpdateParams) {
		p.Status = &status
	}
}

// WithRetryCountInc создает опцию для увеличения счетчика попыток.
func WithRetryCountInc() UpdateOption {
	return func(p *UpdateParams) {
		inc := true
		p.RetryCountInc = &inc
	}
}

// WithScheduledAt создает опцию для установки времени планирования.
func WithScheduledAt(scheduleAt time.Time) UpdateOption {
	return func(p *UpdateParams) {
		p.ScheduledAt = &scheduleAt
	}
}

// WithChannel создает опцию для установки канала уведомления.
func WithChannel(channel Channel) UpdateOption {
	return func(p *UpdateParams) {
		p.Channel = &channel
	}
}

// WithPayload создает опцию для установки payload уведомления.
func WithPayload(payload map[string]interface{}) UpdateOption {
	return func(p *UpdateParams) {
		p.Payload = &OptionalPayload{
			Value: payload,
			Set:   true,
		}
	}
}
