package domain

import "context"

// EmailSender интерфейс для отправки email уведомлений.
type EmailSender interface {
	// Send отправляет email уведомление.
	Send(ctx context.Context, n *Notification) error
}
