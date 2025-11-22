package worker

import (
	"context"
	"encoding/json"
	"errors"

	"DelayedNotifier/internal/domain"
	"DelayedNotifier/pkg/rabbitmq"
	"DelayedNotifier/pkg/retry"
	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	"github.com/wb-go/wbf/zlog"
)

type Consumer struct {
	service       domain.NotificationService
	rabbitClient  *rabbitmq.RabbitClient
	emailSender   domain.EmailSender
	retryStrategy retry.Strategy
}

func NewConsumer(service domain.NotificationService, client *rabbitmq.RabbitClient,
	emailSender domain.EmailSender, strategy retry.Strategy) (*Consumer, error) {
	return &Consumer{
		service:       service,
		rabbitClient:  client,
		emailSender:   emailSender,
		retryStrategy: strategy,
	}, nil
}

func (c *Consumer) Start(ctx context.Context, queueName string, workerNum int, PrefetchCount int) {
	queueArgs := amqp091.Table{
		"x-dead-letter-exchange":    "dlx",              // exchange для DLQ
		"x-dead-letter-routing-key": queueName + ".dlq", // routing key для DLQ
	}
	if workerNum <= 0 {
		workerNum = 1
	}
	if PrefetchCount <= 0 {
		PrefetchCount = 1
	}
	consumer := rabbitmq.NewConsumer(c.rabbitClient, rabbitmq.ConsumerConfig{
		Queue:         queueName,
		Args:          queueArgs,
		Workers:       workerNum,
		PrefetchCount: PrefetchCount,
	}, c.consumerHandler)

	err := consumer.Start(ctx)
	if err != nil {
		return
	}
}

func (c *Consumer) consumerHandler(ctx context.Context, msg amqp091.Delivery) error {
	err := c.sender(ctx, msg.Body)
	if err != nil {
		return err
	}
	return nil
}

func (c *Consumer) sender(ctx context.Context, body []byte) error {
	zlog.Logger.Debug().Str("body", string(body)).Msg("start send")
	j := domain.Job{}
	if err := json.Unmarshal(body, &j); err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to unmarshal body")
		return err
	}

	id, err := uuid.Parse(j.NotificationID)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to parse notification id")
		return err
	}

	n, err := c.service.GetNotificationByID(ctx, id)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("failed to get notification")
	}

	if n.Status == domain.StatusCancelled {
		zlog.Logger.Debug().Msg("notification already cancelled")
		return err
	}

	switch n.Channel {
	case domain.ChannelEmail:
		zlog.Logger.Debug().Msgf(`sending email: id:%s recipient:%s channel:%s payload:%v`,
			n.ID, n.Recipient, n.Channel, n.Payload)
		sendEmail := func() error {
			err := c.emailSender.Send(ctx, n)
			if err != nil {
				zlog.Logger.Debug().Err(err).Msg("failed to send email")
				errInc := c.service.IncRetryCount(ctx, n)
				if errInc != nil {
					return errInc
				}
				return err
			}
			return nil
		}
		err := retry.Do(sendEmail, c.retryStrategy)
		if err != nil {
			zlog.Logger.Error().Err(err).Msg("failed to send email with retry")
			err := c.service.Failed(ctx, n.ID)
			if err != nil {
				zlog.Logger.Error().Err(err).Msg("set status failed")
			}
			return err
		}

	case domain.ChannelTelegram:
		zlog.Logger.Debug().Msgf("sending telegram: id:%s recipient:%s, channel:%s, payload:%v",
			n.ID, n.Recipient, n.Channel, n.Payload)
		// if err set failed status
	default:
		zlog.Logger.Debug().Msg("unknown channel")
		return errors.New("unknown channel " + n.Channel.String())
	}
	err = c.service.UpdateNotification(ctx, n, domain.WithStatus(domain.StatusSent))
	if err != nil {
		return err
	}
	return nil
}
