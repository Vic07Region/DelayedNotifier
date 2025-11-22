package service_test

import (
	"context"
	"encoding/json"
	"log"
	"testing"
	"time"

	"DelayedNotifier/internal/domain"
	"DelayedNotifier/internal/service"
	rd "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository мок для NotificationRepository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Create(ctx context.Context, n domain.CreateParams) (*domain.Notification, error) {
	args := m.Called(ctx, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockRepository) Update(ctx context.Context, id uuid.UUID, opts ...domain.UpdateOption) error {
	args := m.Called(ctx, id, opts)
	return args.Error(0)
}

func (m *MockRepository) ListPendingAndProcessingBefore(ctx context.Context, t time.Time, limit, offset int) ([]domain.Notification, error) {
	args := m.Called(ctx, t, limit, offset)
	return args.Get(0).([]domain.Notification), args.Error(1)
}

func (m *MockRepository) PendingToProcess(ctx context.Context, id uuid.UUID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockRepository) IncRetryCount(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockPublisher мок для MessageQueuePublisher
type MockPublisher struct {
	mock.Mock
}

func (m *MockPublisher) Publish(ctx context.Context, id uuid.UUID, ttl time.Duration) error {
	args := m.Called(ctx, id, ttl)
	return args.Error(0)
}

// MockRedis мок для RedisRepository
type MockRedis struct {
	mock.Mock
}

func (m *MockRedis) Get(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *MockRedis) SetWithExpiration(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	args := m.Called(ctx, key, value, expiration)
	return args.Error(0)
}

// TestCreateNotification_Success проверяет успешное создание уведомления
func TestCreateNotification_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	publisher := new(MockPublisher)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
	}

	repo.On("Create", ctx, mock.Anything).Return(notification, nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("Publish", ctx, notification.ID, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, publisher, redis, time.Hour)

	params := domain.CreateNotificationParams{
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
	}

	result, err := svc.CreateNotification(ctx, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "test@example.com", result.Recipient)
	assert.Equal(t, domain.ChannelEmail, result.Channel)

	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
	redis.AssertExpectations(t)
}

// TestCreateNotification_InvalidChannel проверяет обработку некорректного канала
func TestCreateNotification_InvalidChannel(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	publisher := new(MockPublisher)
	redis := new(MockRedis)

	svc := service.NewNotificationService(repo, publisher, redis, time.Hour)

	params := domain.CreateNotificationParams{
		Recipient:   "test@example.com",
		Channel:     "invalid_channel",
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
	}

	result, err := svc.CreateNotification(ctx, params)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrInvalidChannel, err)
}

// TestCreateNotification_EmptyRecipient проверяет обработку пустого получателя
func TestCreateNotification_EmptyRecipient(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	publisher := new(MockPublisher)
	redis := new(MockRedis)

	svc := service.NewNotificationService(repo, publisher, redis, time.Hour)

	params := domain.CreateNotificationParams{
		Recipient:   "",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
	}

	result, err := svc.CreateNotification(ctx, params)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrEmptyRecipient, err)
}

// TestCreateNotification_RepositoryError проверяет обработку ошибок репозитория
func TestCreateNotification_RepositoryError(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	publisher := new(MockPublisher)
	redis := new(MockRedis)

	repo.On("Create", ctx, mock.Anything).Return(nil, assert.AnError)

	svc := service.NewNotificationService(repo, publisher, redis, time.Hour)

	params := domain.CreateNotificationParams{
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
	}

	result, err := svc.CreateNotification(ctx, params)

	assert.Error(t, err)
	assert.Nil(t, result)
	repo.AssertExpectations(t)
}

// TestCreateNotification_InvalidScheduleTime проверяет обработку некорректного времени планирования
func TestCreateNotification_InvalidScheduleTime(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	publisher := new(MockPublisher)
	redis := new(MockRedis)

	pastTime := time.Now().Add(-time.Hour)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: pastTime,
		Status:      domain.StatusProcessing, // Должно быть processing для прошлого времени
	}

	repo.On("Create", ctx, mock.MatchedBy(func(params domain.CreateParams) bool {
		return params.ScheduledAt.Before(time.Now()) && params.Status == domain.StatusProcessing
	})).Return(notification, nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("Publish", ctx, notification.ID, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, publisher, redis, time.Hour)

	params := domain.CreateNotificationParams{
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: pastTime,
	}

	result, err := svc.CreateNotification(ctx, params)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.StatusProcessing, result.Status)

	repo.AssertExpectations(t)
}

// TestCreateNotification_PublisherError проверяет обработку ошибок publisher
func TestCreateNotification_PublisherError(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	publisher := new(MockPublisher)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
	}

	repo.On("Create", ctx, mock.Anything).Return(notification, nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	publisher.On("Publish", ctx, notification.ID, mock.Anything).Return(assert.AnError)
	repo.On("Update", ctx, notification.ID, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, publisher, redis, time.Hour)

	params := domain.CreateNotificationParams{
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
	}

	result, err := svc.CreateNotification(ctx, params)
	log.Println(err)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, domain.StatusPending, result.Status) // Статус должен быть обновлен

	repo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

// TestGetNotificationByID_FromDatabase проверяет получение уведомления из базы данных
func TestGetNotificationByID_FromDatabase(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
	}

	// Redis возвращает ошибку redis nil
	redis.On("Get", ctx, notification.ID.String()).Return("", rd.Nil)
	repo.On("GetByID", ctx, notification.ID).Return(notification, nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, nil, redis, time.Hour)

	result, err := svc.GetNotificationByID(ctx, notification.ID)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, notification.ID, result.ID)

	repo.AssertExpectations(t)
	redis.AssertExpectations(t)
}

// TestGetNotificationByID_FromRedis проверяет получение уведомления из Redis
func TestGetNotificationByID_FromRedis(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
	}

	// Данные есть в Redis
	notificationData, _ := json.Marshal(notification)
	redis.On("Get", ctx, notification.ID.String()).Return(string(notificationData), nil)

	svc := service.NewNotificationService(repo, nil, redis, time.Hour)

	result, err := svc.GetNotificationByID(ctx, notification.ID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, notification.ID, result.ID)

	repo.AssertNotCalled(t, "GetByID")
	redis.AssertExpectations(t)
}

// TestGetNotificationByID_NotFound проверяет обработку отсутствующего уведомления
func TestGetNotificationByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	redis := new(MockRedis)

	notificationID := uuid.New()
	redis.On("Get", ctx, notificationID.String()).Return("", rd.Nil)
	repo.On("GetByID", ctx, notificationID).Return(nil, domain.ErrNotFound)
	svc := service.NewNotificationService(repo, nil, redis, time.Hour)
	result, err := svc.GetNotificationByID(ctx, notificationID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrNotFound, err)

	repo.AssertExpectations(t)
	redis.AssertExpectations(t)
}

// TestUpdateNotification_Success проверяет успешное обновление уведомления
func TestUpdateNotification_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
	}

	repo.On("Update", ctx, notification.ID, mock.Anything).Return(nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, nil, redis, time.Hour)

	err := svc.UpdateNotification(ctx, notification, domain.WithStatus(domain.StatusProcessing))

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusProcessing, notification.Status)

	repo.AssertExpectations(t)
}

// TestCancel_Success проверяет успешную отмену уведомления
func TestCancel_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
	}

	redis.On("Get", ctx, notification.ID.String()).Return("", rd.Nil) // Данные не найдены в Redis
	repo.On("GetByID", ctx, notification.ID).Return(notification, nil)
	repo.On("Update", ctx, notification.ID, mock.Anything).Return(nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, nil, redis, time.Hour)

	err := svc.Cancel(ctx, notification.ID)

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusCancelled, notification.Status)

	repo.AssertExpectations(t)
}

// TestFailed_Success проверяет успешную установку статуса "failed"
func TestFailed_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusProcessing,
	}

	redis.On("Get", ctx, notification.ID.String()).Return("", rd.Nil) // Данные не найдены в Redis
	repo.On("GetByID", ctx, notification.ID).Return(notification, nil)
	repo.On("Update", ctx, notification.ID, mock.Anything).Return(nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, nil, redis, time.Hour)

	err := svc.Failed(ctx, notification.ID)

	assert.NoError(t, err)
	assert.Equal(t, domain.StatusFailed, notification.Status)

	repo.AssertExpectations(t)
}

// TestIncRetryCount_Success проверяет успешное увеличение счетчика повторов
func TestIncRetryCount_Success(t *testing.T) {
	ctx := context.Background()
	repo := new(MockRepository)
	redis := new(MockRedis)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusProcessing,
		RetryCount:  1,
	}

	repo.On("Update", ctx, notification.ID, mock.Anything).Return(nil)
	redis.On("SetWithExpiration", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	svc := service.NewNotificationService(repo, nil, redis, time.Hour)

	err := svc.IncRetryCount(ctx, notification)

	assert.NoError(t, err)
	assert.Equal(t, 1, notification.RetryCount)

	repo.AssertExpectations(t)
}
