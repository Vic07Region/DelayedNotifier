package domain

import "errors"

var (
	// ErrNoRowAffected ошибка, когда ни одна строка не была изменена.
	ErrNoRowAffected = errors.New("no row affected")
	// ErrNotFound ошибка, когда уведомление не найдено.
	ErrNotFound = errors.New("notification not found")
)
