# Этап сборки - используем golang image для компиляции
FROM golang:1.25-alpine AS builder

# Установка git и ca-certificates (нужны для некоторых go модулей)
RUN apk add --no-cache git ca-certificates tzdata

# Создаем рабочую директорию
WORKDIR /app

# Копируем go mod файлы
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Компилируем приложение
# CGO_ENABLED=0 отключает C extensions для статической линковки
# -ldflags "-w -s" убирает debug info для уменьшения размера
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -extldflags '-static'" -o delayed-notifier ./cmd/main.go

# Финальный этап - используем scratch image (минимальный размер)
FROM scratch

# Копируем сертификаты из builder этапа
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Копируем timezone данные
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Копируем скомпилированный бинарник
COPY --from=builder /app/delayed-notifier /delayed-notifier

# Копируем статические файлы веб-интерфейса
COPY --from=builder /app/web /web

# Копируем миграции
COPY --from=builder /app/migrations /migrations

# Указываем пользователя для безопасности
USER 65534

# Экспортируем порт (если нужен для debug)
EXPOSE 8080

# Запускаем приложение
ENTRYPOINT ["/delayed-notifier"]