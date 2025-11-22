package main

import (
	"fmt"
	"os"

	"DelayedNotifier/internal/app"
)

func main() {
	// Создаем новое приложение
	application, err := app.New()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to create application: %v\n", err)
		os.Exit(1)
	}

	// Запускаем приложение
	if err := application.Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}
}
