package domain

import "errors"

var (
	// ErrInvalidChannel ошибка невалидного канала уведомления.
	ErrInvalidChannel = errors.New("invalid channel")
	// ErrInvalidStatus ошибка невалидного статуса уведомления.
	ErrInvalidStatus = errors.New("invalid status")
	// ErrEmptyRecipient ошибка пустого получателя.
	ErrEmptyRecipient = errors.New("recipient is empty")
	// ErrEmptyUpdateOptions ошибка пустых параметров обновления.
	ErrEmptyUpdateOptions = errors.New("no update options provided")
)
