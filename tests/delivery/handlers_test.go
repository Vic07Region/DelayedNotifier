package delivery_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"DelayedNotifier/internal/delivery/handlers"
	"DelayedNotifier/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockNotificationService мок для NotificationService
type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) CreateNotification(ctx context.Context, params domain.CreateNotificationParams) (*domain.Notification, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) UpdateNotification(ctx context.Context, n *domain.Notification, opts ...domain.UpdateOption) error {
	args := m.Called(ctx, n, opts)
	return args.Error(0)
}

func (m *MockNotificationService) GetNotificationByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) Cancel(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationService) Failed(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationService) IncRetryCount(ctx context.Context, n *domain.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

// TestCreateNotificationHandler_Success проверяет успешное создание уведомления через HTTP
func TestCreateNotificationHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	scheduledAt := time.Now().Add(time.Hour).Format(time.RFC3339)
	expectedScheduledAt, _ := time.Parse(time.RFC3339, scheduledAt)

	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test Email", "body": "Hello World"},
		ScheduledAt: expectedScheduledAt,
		Status:      domain.StatusPending,
	}

	mockService.On("CreateNotification", mock.Anything, mock.MatchedBy(func(params domain.CreateNotificationParams) bool {
		return params.Recipient == "test@example.com" &&
			params.Channel == domain.ChannelEmail &&
			params.ScheduledAt.Equal(expectedScheduledAt) &&
			params.Payload["subject"] == "Test Email" &&
			params.Payload["body"] == "Hello World"
	})).Return(notification, nil)

	reqBody := `{
		"recipient": "test@example.com",
		"channel": "email",
		"payload": "{\"subject\":\"Test Email\",\"body\":\"Hello World\"}",
		"scheduled_at": "` + scheduledAt + `"
	}`

	req, _ := http.NewRequest("POST", "/notifications", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateNotificationHandler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "result")

	mockService.AssertExpectations(t)
}

// TestCreateNotificationHandler_InvalidJSON проверяет обработку некорректного JSON
func TestCreateNotificationHandler_InvalidJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	invalidJSON := `{"recipient": "test@example.com", "channel": "email", "payload": invalid json}`

	req, _ := http.NewRequest("POST", "/notifications", strings.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateNotificationHandler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"], "Некорректный JSON")
}

// TestCreateNotificationHandler_ValidationError проверяет обработку ошибок валидации
func TestCreateNotificationHandler_ValidationError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	reqBody := `{"recipient": "", "channel": "", "payload": "", "scheduled_at": ""}`

	req, _ := http.NewRequest("POST", "/notifications", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateNotificationHandler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "errors")
}

// TestCreateNotificationHandler_InvalidChannel проверяет обработку некорректного канала
func TestCreateNotificationHandler_InvalidChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	scheduledAt := time.Now().Add(time.Hour).Format(time.RFC3339)

	reqBody := `{
		"recipient": "test@example.com",
		"channel": "invalid_channel",
		"payload": "{\"subject\":\"Test\"}",
		"scheduled_at": "` + scheduledAt + `"
	}`

	req, _ := http.NewRequest("POST", "/notifications", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateNotificationHandler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"], "не поддерживается")
}

// TestCreateNotificationHandler_ServiceError проверяет обработку ошибок сервиса
func TestCreateNotificationHandler_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	scheduledAt := time.Now().Add(time.Hour).Format(time.RFC3339)

	mockService.On("CreateNotification", mock.Anything, mock.Anything).Return(nil, assert.AnError)

	reqBody := `{
		"recipient": "test@example.com",
		"channel": "email",
		"payload": "{\"subject\":\"Test\"}",
		"scheduled_at": "` + scheduledAt + `"
	}`

	req, _ := http.NewRequest("POST", "/notifications", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateNotificationHandler(c)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestCreateNotificationHandler_InvalidScheduledAt проверяет обработку некорректного времени
func TestCreateNotificationHandler_InvalidScheduledAt(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	reqBody := `{
		"recipient": "test@example.com",
		"channel": "email",
		"payload": "{\"subject\":\"Test\"}",
		"scheduled_at": "invalid-time"
	}`

	req, _ := http.NewRequest("POST", "/notifications", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateNotificationHandler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t,
		(response != nil && response["error"] != nil) ||
			(response != nil && response["errors"] != nil),
		"Response should contain either 'error' or 'errors' field")
}

// TestCreateNotificationHandler_InvalidPayloadJSON проверяет обработку некорректного JSON в payload
func TestCreateNotificationHandler_InvalidPayloadJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	scheduledAt := time.Now().Add(time.Hour).Format(time.RFC3339)

	reqBody := `{
		"recipient": "test@example.com",
		"channel": "email",
		"payload": "not-valid-json",
		"scheduled_at": "` + scheduledAt + `"
	}`

	req, _ := http.NewRequest("POST", "/notifications", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.CreateNotificationHandler(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	assert.True(t,
		(response != nil && response["error"] != nil) ||
			(response != nil && response["errors"] != nil),
		"Response should contain either 'error' or 'errors' field")
}

// TestGetNotificationHandler_Success проверяет успешное получение уведомления через HTTP
func TestGetNotificationHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	notificationID := uuid.New()
	notification := &domain.Notification{
		ID:          notificationID,
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
	}

	mockService.On("GetNotificationByID", mock.Anything, notificationID).Return(notification, nil)

	req, _ := http.NewRequest("GET", "/notifications/"+notificationID.String(), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: notificationID.String()}}

	h.GetNotificationHandler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "result")

	mockService.AssertExpectations(t)
}

// TestGetNotificationHandler_InvalidID проверяет обработку некорректного ID
func TestGetNotificationHandler_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	req, _ := http.NewRequest("GET", "/notifications/invalid-id", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: "invalid-id"}}

	h.GetNotificationHandler(c)

	// Проверяем результат
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"], "id is invalid")
}

// TestGetNotificationHandler_ServiceError проверяет обработку ошибок сервиса при получении
func TestGetNotificationHandler_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	notificationID := uuid.New()

	// Настраиваем мок для возврата ошибки
	mockService.On("GetNotificationByID", mock.Anything, notificationID).Return(nil, assert.AnError)

	// Создаем HTTP запрос
	req, _ := http.NewRequest("GET", "/notifications/"+notificationID.String(), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: notificationID.String()}}

	h.GetNotificationHandler(c)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}

// TestDeleteNotificationHandler_Success проверяет успешное удаление уведомления через HTTP
func TestDeleteNotificationHandler_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	notificationID := uuid.New()

	// Настраиваем ожидания мока
	mockService.On("Cancel", mock.Anything, notificationID).Return(nil)

	// Создаем HTTP запрос
	req, _ := http.NewRequest("DELETE", "/notifications/"+notificationID.String(), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: notificationID.String()}}

	h.DeleteNotificationHandler(c)

	// Проверяем результат
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "result")

	mockService.AssertExpectations(t)
}

// TestDeleteNotificationHandler_InvalidID проверяет обработку некорректного ID при удалении
func TestDeleteNotificationHandler_InvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	// Создаем HTTP запрос с некорректным ID
	req, _ := http.NewRequest("DELETE", "/notifications/invalid-id", nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: "invalid-id"}}

	h.DeleteNotificationHandler(c)

	// Проверяем результат
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
	assert.Contains(t, response["error"], "id is invalid")
}

// TestDeleteNotificationHandler_ServiceError проверяет обработку ошибок сервиса при удалении
func TestDeleteNotificationHandler_ServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := new(MockNotificationService)
	h := handlers.NewHandlersSet(mockService)

	notificationID := uuid.New()

	// Настраиваем мок для возврата ошибки
	mockService.On("Cancel", mock.Anything, notificationID).Return(assert.AnError)

	// Создаем HTTP запрос
	req, _ := http.NewRequest("DELETE", "/notifications/"+notificationID.String(), nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = []gin.Param{{Key: "id", Value: notificationID.String()}}

	h.DeleteNotificationHandler(c)

	// Проверяем результат
	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "error")
}
