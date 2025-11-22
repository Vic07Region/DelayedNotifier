package repository_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"DelayedNotifier/internal/domain"
	"DelayedNotifier/internal/repository/pg"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/wb-go/wbf/dbpg"
)

func TestPostgresRepo_Create_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	// Create dbpg.DB instance
	dbpgDB := &dbpg.DB{Master: db}

	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	now := time.Now()
	notificationID := uuid.New()

	// Mock the INSERT query and RETURNING clause
	jsonPayload, _ := json.Marshal(map[string]interface{}{"subject": "test"})
	mock.ExpectQuery(`INSERT INTO notifications`).
		WithArgs("test@example.com", domain.ChannelEmail, jsonPayload, sqlmock.AnyArg(), domain.StatusPending).
		WillReturnRows(sqlmock.NewRows([]string{"id", "retry_count", "created_at", "updated_at"}).
			AddRow(notificationID, 0, now, now))

	// Execute
	params := domain.CreateParams{
		Recipient:   "test@example.com",
		Channel:     domain.ChannelEmail,
		Status:      domain.StatusPending,
		Payload:     map[string]interface{}{"subject": "test"},
		ScheduledAt: now,
	}

	result, err := repo.Create(context.Background(), params)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, notificationID, result.ID)
	assert.Equal(t, "test@example.com", result.Recipient)
	assert.Equal(t, domain.ChannelEmail, result.Channel)
	assert.Equal(t, domain.StatusPending, result.Status)
}

func TestPostgresRepo_GetByID_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	now := time.Now()
	notificationID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{"subject": "test"})

	mock.ExpectQuery(`SELECT id, recipient, channel, payload, scheduled_at, status, retry_count, created_at, updated_at`).
		WithArgs(notificationID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "recipient", "channel", "payload", "scheduled_at", "status", "retry_count", "created_at", "updated_at"}).
			AddRow(notificationID, "test@example.com", domain.ChannelEmail, payload, now, domain.StatusPending, 0, now, now))

	// Execute
	result, err := repo.GetByID(context.Background(), notificationID)

	// Assertions
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, notificationID, result.ID)
	assert.Equal(t, "test@example.com", result.Recipient)
	assert.Equal(t, domain.ChannelEmail, result.Channel)
}

func TestPostgresRepo_GetByID_NotFound(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectQuery(`SELECT id, recipient, channel, payload, scheduled_at, status, retry_count, created_at, updated_at`).
		WithArgs(notificationID).
		WillReturnError(sql.ErrNoRows)

	// Execute
	result, err := repo.GetByID(context.Background(), notificationID)

	// Assertions
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, domain.ErrNotFound, err)
}

func TestPostgresRepo_Update_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectExec(`UPDATE notifications SET status = \$1 WHERE id = \$2`).
		WithArgs(domain.StatusProcessing, notificationID).
		WillReturnResult(sqlmock.NewResult(0, 1)) // 1 row affected

	// Execute
	err = repo.Update(context.Background(), notificationID, domain.WithStatus(domain.StatusProcessing))

	// Assertions
	assert.NoError(t, err)
}

func TestPostgresRepo_Update_NoRowsAffected(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectExec(`UPDATE notifications SET status = \$1 WHERE id = \$2`).
		WithArgs(domain.StatusProcessing, notificationID).
		WillReturnResult(sqlmock.NewResult(0, 0)) // 0 rows affected

	// Execute
	err = repo.Update(context.Background(), notificationID, domain.WithStatus(domain.StatusProcessing))

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, domain.ErrNoRowAffected, err)
}

func TestPostgresRepo_Update_EmptyOptions(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations - no queries should be executed
	// Мок для пустого запроса - не должен вызываться
	mock.MatchExpectationsInOrder(false)

	// Execute
	err = repo.Update(context.Background(), uuid.New())

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no update options provided")
}

func TestPostgresRepo_Update_WithRetryCount(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectExec(`UPDATE notifications SET retry_count = retry_count \+ 1 WHERE id = \$1`).
		WithArgs(notificationID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	err = repo.Update(context.Background(), notificationID, domain.WithRetryCountInc())

	// Assertions
	assert.NoError(t, err)
}

func TestPostgresRepo_ListPendingAndProcessingBefore_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	now := time.Now()
	stuckTime := now.Add(-10 * time.Minute)

	notificationID1 := uuid.New()
	notificationID2 := uuid.New()

	payload1, _ := json.Marshal(map[string]interface{}{"subject": "test1"})
	payload2, _ := json.Marshal(map[string]interface{}{"subject": "test2"})

	mock.ExpectQuery(`SELECT id, recipient, channel, payload, scheduled_at, status, retry_count, created_at, updated_at`).
		WithArgs(stuckTime, domain.StatusPending, domain.StatusProcessing).
		WillReturnRows(sqlmock.NewRows([]string{"id", "recipient", "channel", "payload", "scheduled_at", "status", "retry_count", "created_at", "updated_at"}).
			AddRow(notificationID1, "test1@example.com", domain.ChannelEmail, payload1, now, domain.StatusPending, 0, now, now).
			AddRow(notificationID2, "test2@example.com", domain.ChannelTelegram, payload2, now, domain.StatusProcessing, 1, now, now))

	// Execute
	result, err := repo.ListPendingAndProcessingBefore(context.Background(), stuckTime, 0, 0)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, notificationID1, result[0].ID)
	assert.Equal(t, notificationID2, result[1].ID)
}

func TestPostgresRepo_ListPendingAndProcessingBefore_Empty(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	stuckTime := time.Now().Add(-10 * time.Minute)

	mock.ExpectQuery(`SELECT id, recipient, channel, payload, scheduled_at, status, retry_count, created_at, updated_at`).
		WithArgs(stuckTime, domain.StatusPending, domain.StatusProcessing).
		WillReturnRows(sqlmock.NewRows([]string{"id", "recipient", "channel", "payload", "scheduled_at", "status", "retry_count", "created_at", "updated_at"}))

	// Execute
	result, err := repo.ListPendingAndProcessingBefore(context.Background(), stuckTime, 0, 0)

	// Assertions
	assert.Error(t, err)
	assert.Equal(t, domain.ErrNotFound, err)
	assert.Len(t, result, 0)
}

func TestPostgresRepo_PendingToProcess_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectExec(`UPDATE notifications SET status = \$1 WHERE id = \$2 AND status = \$3`).
		WithArgs(domain.StatusProcessing, notificationID, domain.StatusPending).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	updated, err := repo.PendingToProcess(context.Background(), notificationID)

	// Assertions
	assert.NoError(t, err)
	assert.True(t, updated)
}

func TestPostgresRepo_PendingToProcess_NotUpdated(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectExec(`UPDATE notifications SET status = \$1 WHERE id = \$2 AND status = \$3`).
		WithArgs(domain.StatusProcessing, notificationID, domain.StatusPending).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Execute
	updated, err := repo.PendingToProcess(context.Background(), notificationID)

	// Assertions
	assert.NoError(t, err)
	assert.False(t, updated)
}

func TestPostgresRepo_IncRetryCount_Success(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectExec(`UPDATE notifications SET retry_count = retry_count \+ 1 WHERE id = \$1`).
		WithArgs(notificationID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Execute
	err = repo.IncRetryCount(context.Background(), notificationID)

	// Assertions
	assert.NoError(t, err)
}

func TestPostgresRepo_IncRetryCount_NotFound(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	notificationID := uuid.New()

	mock.ExpectExec(`UPDATE notifications SET retry_count = retry_count \+ 1 WHERE id = \$1`).
		WithArgs(notificationID).
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Execute
	err = repo.IncRetryCount(context.Background(), notificationID)

	// Assertions
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no retry count found")
}

func TestPostgresRepo_Update_WithLimit(t *testing.T) {
	// Setup
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	dbpgDB := &dbpg.DB{Master: db}
	repo := pg.NewPostgresRepo(dbpgDB)

	// Setup mock expectations
	stuckTime := time.Now().Add(-10 * time.Minute)
	notificationID := uuid.New()

	payload, _ := json.Marshal(map[string]interface{}{"subject": "test"})

	mock.ExpectQuery(`SELECT id, recipient, channel, payload, scheduled_at, status, retry_count, created_at, updated_at`).
		WithArgs(stuckTime, domain.StatusPending, domain.StatusProcessing).
		WillReturnRows(sqlmock.NewRows([]string{"id", "recipient", "channel", "payload", "scheduled_at", "status", "retry_count", "created_at", "updated_at"}).
			AddRow(notificationID, "test@example.com", domain.ChannelEmail, payload, time.Now(), domain.StatusPending, 0, time.Now(), time.Now()))

	// Execute with limit
	result, err := repo.ListPendingAndProcessingBefore(context.Background(), stuckTime, 10, 0)

	// Assertions
	assert.NoError(t, err)
	assert.Len(t, result, 1)
}
