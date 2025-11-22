package rabbit

import (
	"context"
	"time"

	"DelayedNotifier/pkg/rabbitmq"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	"github.com/wb-go/wbf/zlog"
)

// Publisher структура для публикации сообщений в RabbitMQ.
type Publisher struct {
	client    *rabbitmq.RabbitClient
	publisher *rabbitmq.Publisher
	dlqName   string
	exchange  string
}

// NewPublisher создает новый экземпляр Publisher.
func NewPublisher(client *rabbitmq.RabbitClient, exchange, contentType, dlqName string) *Publisher {
	pub := rabbitmq.NewPublisher(client, exchange, contentType)
	return &Publisher{publisher: pub, client: client, dlqName: dlqName, exchange: exchange}
}

// Publish публикует уведомление в очередь с указанным TTL.
func (r *Publisher) Publish(ctx context.Context, id uuid.UUID, ttl time.Duration) error {
	exp := ttl + 2*time.Second
	queueArgs := amqp091.Table{
		"x-dead-letter-exchange":    r.exchange, // exchange для DLQ
		"x-dead-letter-routing-key": r.dlqName,  // routing key для DLQ
		"x-expires":                 exp.Milliseconds(),
	}
	queueName := "queue:" + id.String()
	err := r.client.DeclareQueue(
		queueName,
		r.exchange,
		id.String(), false,
		true,
		false,
		queueArgs,
	)
	if err != nil {
		return err
	}
	body := []byte(`{"notification_id":"` + id.String() + `"}`)

	err = r.publisher.Publish(ctx, body, id.String(), rabbitmq.WithExpiration(ttl))
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to publish notification")
		return err
	}

	return nil
}
