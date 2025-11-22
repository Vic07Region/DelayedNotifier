package domain_test

import (
	"testing"
	"time"

	"DelayedNotifier/internal/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   domain.Status
		expected string
	}{
		{domain.StatusPending, "pending"},
		{domain.StatusProcessing, "processing"},
		{domain.StatusSent, "sent"},
		{domain.StatusFailed, "failed"},
		{domain.StatusCancelled, "cancelled"},
	}

	for _, tt := range tests {
		t.Run("status_"+tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		status domain.Status
		valid  bool
	}{
		{domain.StatusPending, true},
		{domain.StatusProcessing, true},
		{domain.StatusSent, true},
		{domain.StatusFailed, true},
		{domain.StatusCancelled, true},
		{"invalid_status", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run("status_"+string(tt.status), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.status.IsValid())
		})
	}
}

func TestChannel_String(t *testing.T) {
	tests := []struct {
		channel  domain.Channel
		expected string
	}{
		{domain.ChannelEmail, "email"},
		{domain.ChannelTelegram, "telegram"},
	}

	for _, tt := range tests {
		t.Run("channel_"+tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.channel.String())
		})
	}
}

func TestChannel_IsValid(t *testing.T) {
	tests := []struct {
		channel domain.Channel
		valid   bool
	}{
		{domain.ChannelEmail, true},
		{domain.ChannelTelegram, true},
		{"invalid_channel", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run("channel_"+string(tt.channel), func(t *testing.T) {
			assert.Equal(t, tt.valid, tt.channel.IsValid())
		})
	}
}

func TestNotification_Create(t *testing.T) {
	notification := &domain.Notification{
		ID:          uuid.New(),
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Payload:     map[string]interface{}{"subject": "Test", "body": "Hello"},
		ScheduledAt: time.Now().Add(time.Hour),
		Status:      domain.StatusPending,
		RetryCount:  0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	assert.NotNil(t, notification)
	assert.Equal(t, "test@example.com", notification.Recipient)
	assert.Equal(t, domain.ChannelEmail, notification.Channel)
	assert.Equal(t, domain.StatusPending, notification.Status)
}

func TestJob_Serialization(t *testing.T) {
	job := domain.Job{
		NotificationID: uuid.New().String(),
	}

	assert.NotEmpty(t, job.NotificationID)
	assert.Len(t, job.NotificationID, 36) // UUID length
}

func TestCreateParams_Validation(t *testing.T) {
	params := domain.CreateParams{
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Status:      domain.StatusPending,
		Payload:     map[string]interface{}{"test": "data"},
		ScheduledAt: time.Now().Add(time.Hour),
	}

	assert.Equal(t, "test@example.com", params.Recipient)
	assert.Equal(t, domain.ChannelEmail, params.Channel)
	assert.Equal(t, domain.StatusPending, params.Status)
}

func TestUpdateOptions(t *testing.T) {
	// Test WithStatus option
	status := domain.StatusProcessing
	option := domain.WithStatus(status)
	params := &domain.UpdateParams{}
	option(params)

	assert.Equal(t, &status, params.Status)

	// Test WithRetryCountInc option
	incOption := domain.WithRetryCountInc()
	params2 := &domain.UpdateParams{}
	incOption(params2)

	assert.NotNil(t, params2.RetryCountInc)
	assert.True(t, *params2.RetryCountInc)
}

func TestOptionalPayload(t *testing.T) {
	payload := map[string]interface{}{"key": "value"}
	optPayload := domain.OptionalPayload{
		Value: payload,
		Set:   true,
	}

	assert.True(t, optPayload.Set)
	assert.Equal(t, payload, optPayload.Value)
}
