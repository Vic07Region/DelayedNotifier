package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"DelayedNotifier/internal/domain"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/wb-go/wbf/zlog"
)

const (
	redisKeyPrefix = "notification:"
)

type NotificationService struct {
	repo            domain.NotificationRepository
	publisher       domain.MessageQueuePublisher
	redis           domain.RedisRepository
	redisExpiration time.Duration
}

func NewNotificationService(
	repo domain.NotificationRepository,
	publisher domain.MessageQueuePublisher,
	redis domain.RedisRepository,
	redisExpiration time.Duration) *NotificationService {
	return &NotificationService{repo: repo, publisher: publisher, redis: redis, redisExpiration: redisExpiration}
}

func (s *NotificationService) CreateNotification(ctx context.Context,
	params domain.CreateNotificationParams) (*domain.Notification, error) {
	op := "CreateNotification:"
	if !params.Channel.IsValid() {
		zlog.Logger.Warn().Msgf("%s notification (channel = %s) is invalid", op, params.Channel.String())
		return nil, domain.ErrInvalidChannel
	}
	if params.Recipient == "" {
		zlog.Logger.Warn().Msgf("%s recipient is empty", op)
		return nil, domain.ErrEmptyRecipient
	}
	opt := domain.CreateParams{
		Recipient:   params.Recipient,
		Channel:     params.Channel,
		Payload:     params.Payload,
		ScheduledAt: params.ScheduledAt,
	}
	currentTime := time.Now().Add(2 * time.Second)
	var ttl time.Duration
	if params.ScheduledAt.Before(currentTime) {
		ttl = 2 * time.Second
		opt.Status = domain.StatusProcessing
	} else {
		opt.Status = domain.StatusPending
		ttl = params.ScheduledAt.Sub(currentTime)
	}

	n, err := s.repo.Create(ctx, opt)
	if err != nil {
		zlog.Logger.Error().Msgf("%s failed to create notification: %v", op, err)
		return nil, err
	}

	if err := s.marshalAndSet(ctx, n); err != nil {
		return nil, err
	}

	zlog.Logger.Debug().Msgf("%s notification created, ttl:%v", op, ttl)
	err = s.publisher.Publish(ctx, n.ID, ttl)
	if err != nil {
		zlog.Logger.Error().Msgf("%s failed to send notification: %v", op, err)
		err = s.repo.Update(ctx, n.ID, domain.WithStatus(domain.StatusPending))
		if err != nil {
			zlog.Logger.Error().Msgf("%s failed to update status: %v", op, err)
			return nil, err
		}
		n.Status = domain.StatusPending
	}

	return n, nil
}

func (s *NotificationService) UpdateNotification(ctx context.Context, n *domain.Notification,
	opts ...domain.UpdateOption) error {
	op := "UpdateNotification:"
	if len(opts) == 0 {
		return domain.ErrEmptyUpdateOptions
	}
	params := &domain.UpdateParams{}
	for _, opt := range opts {
		opt(params)
	}
	if params.Status != nil {
		if !params.Status.IsValid() {
			zlog.Logger.Warn().Msgf("%s notification (status = %s) is invalid", op, params.Status.String())
			return domain.ErrInvalidStatus
		}
		n.Status = *params.Status
	}
	if params.Channel != nil {
		if params.Channel.IsValid() {
			zlog.Logger.Warn().Msgf("%s channel (channel = %s) is invalid", op, params.Channel.String())
			return domain.ErrInvalidChannel
		}
		n.Channel = *params.Channel
	}

	if err := s.repo.Update(ctx, n.ID, opts...); err != nil {
		if errors.Is(err, domain.ErrNoRowAffected) {
			zlog.Logger.Warn().Msgf("%s Notification Update: %v", op, err)
			return nil
		}
		zlog.Logger.Error().Msgf("%s failed to update notification: %v", op, err)
		return err
	}
	err := s.marshalAndSet(ctx, n)
	if err != nil {
		zlog.Logger.Error().Msgf("%s failed to update notification: %v", op, err)
		return err
	}
	return nil
}

func (s *NotificationService) GetNotificationByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	var n *domain.Notification
	redisData, err := s.redis.Get(ctx, id.String())
	zlog.Logger.Debug().Err(err).Msgf("Get notification by id not found %v", errors.Is(err, redis.Nil))
	if err != nil && !errors.Is(err, redis.Nil) {
		zlog.Logger.Error().Err(err).Msgf("failed to fetch notification: %v", err)
		return nil, err
	}

	if errors.Is(err, redis.Nil) {
		zlog.Logger.Debug().Msgf("%s: notification not found fetch to database", id)
		n, err = s.repo.GetByID(ctx, id)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				zlog.Logger.Warn().Msgf("notification (id = %s) not found", id)
				return nil, domain.ErrNotFound
			}
			return nil, err
		}

		err := s.marshalAndSet(ctx, n)
		if err != nil {
			zlog.Logger.Error().Msgf("%s failed to update to redis notification info: %v", id, err)
			return nil, err
		}

		return n, nil
	}

	zlog.Logger.Debug().Msgf("%s: notification found: %s", id.String(), redisData)
	err = json.Unmarshal([]byte(redisData), &n)
	if err != nil {
		zlog.Logger.Error().Err(err).Msgf("%s: failed to unmarshal notification: %v", id, err)
	}
	return n, nil
}

func (s *NotificationService) transitionStatus(
	ctx context.Context,
	id uuid.UUID,
	allowedStatus domain.Status,
	statusUpdater domain.Status,
	actionName string,
) error {
	n, err := s.GetNotificationByID(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			zlog.Logger.Warn().Msgf("notification (id = %s) not found", id)
			return err
		}
		return err
	}

	if n.Status != allowedStatus {
		return fmt.Errorf("notification id=%s status=%s", id.String(), n.Status)
	}

	if err = s.UpdateNotification(ctx, n, domain.WithStatus(statusUpdater)); err != nil {
		zlog.Logger.Error().Msgf("failed to %s notification: %v", actionName, err)
		return err
	}

	return nil
}

func (s *NotificationService) Cancel(ctx context.Context, id uuid.UUID) error {
	return s.transitionStatus(ctx, id, domain.StatusPending, domain.StatusCancelled, "cancel")
}

func (s *NotificationService) Failed(ctx context.Context, id uuid.UUID) error {
	return s.transitionStatus(ctx, id, domain.StatusProcessing, domain.StatusFailed, "failed")
}

func (s *NotificationService) IncRetryCount(ctx context.Context, n *domain.Notification) error {
	return s.UpdateNotification(ctx, n, domain.WithRetryCountInc())
}

func (s *NotificationService) marshalAndSet(ctx context.Context, n *domain.Notification) error {
	data, err := json.Marshal(n)
	if err != nil {
		zlog.Logger.Error().Msgf("%s failed to marshal notification: %v", n.ID, err)
		return err
	}
	err = s.redis.SetWithExpiration(ctx, redisKeyPrefix+n.ID.String(), data, s.redisExpiration)
	if err != nil {
		zlog.Logger.Error().Msgf("%s failed to set notification expiry: %v", n.ID, err)
		return err
	}
	return nil
}
