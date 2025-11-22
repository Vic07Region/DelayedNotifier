package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"DelayedNotifier/internal/domain"
	"github.com/google/uuid"
	"github.com/wb-go/wbf/dbpg"
	"github.com/wb-go/wbf/zlog"
)

// PostgresRepo структура для работы с PostgreSQL.
type PostgresRepo struct {
	DB *dbpg.DB
}

// NewPostgresRepo создает новый экземпляр PostgresRepo.
func NewPostgresRepo(db *dbpg.DB) *PostgresRepo {
	return &PostgresRepo{
		DB: db,
	}
}

// Create создает новое уведомление в базе данных.
func (p *PostgresRepo) Create(ctx context.Context, n domain.CreateParams) (*domain.Notification, error) {
	sqlQuery := `INSERT INTO notifications (recipient,channel,payload,scheduled_at,status) VALUES ($1, $2, $3, $4, $5)
 RETURNING id, retry_count, created_at, updated_at`
	jsonData, err := json.Marshal(n.Payload)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Error marshalling notification payload")
		return nil, err
	}
	var result domain.Notification
	if err = p.DB.QueryRowContext(ctx, sqlQuery, n.Recipient, n.Channel, jsonData, n.ScheduledAt, n.Status).Scan(
		&result.ID, &result.RetryCount, &result.CreatedAt, &result.UpdatedAt); err != nil {
		zlog.Logger.Error().Err(err).Msg("Error scanning notification")
		return nil, err
	}
	result.Recipient = n.Recipient
	result.Channel = n.Channel
	result.Payload = n.Payload
	result.Status = n.Status
	result.ScheduledAt = n.ScheduledAt

	zlog.Logger.Debug().Msgf(
		"Created notification id: %s to:%s, channel:%s, payload: %s, scheduledAt:, %v",
		result.ID,
		n.Recipient,
		n.Channel,
		n.Payload,
		n.ScheduledAt,
	)

	return &result, nil
}

// GetByID получает уведомление по ID из базы данных.
func (p *PostgresRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	start := time.Now()

	sqlQuery := `SELECT id, recipient, channel, 
       payload, scheduled_at, status, 
       retry_count, created_at, updated_at 
	FROM notifications WHERE id = $1 LIMIT 1`

	var result domain.Notification
	var payloadRaw []byte

	if err := p.DB.QueryRowContext(ctx, sqlQuery, id).Scan(&result.ID, &result.Recipient, &result.Channel,
		&payloadRaw, &result.ScheduledAt, &result.Status,
		&result.RetryCount, &result.CreatedAt, &result.UpdatedAt); err != nil {
		zlog.Logger.Error().Err(err).Msg("Error scan notification fields")
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}

	err := json.Unmarshal(payloadRaw, &result.Payload)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Error unmarshalling notification payload")
	}
	zlog.Logger.Debug().Msgf("Get notification by id: %s result: %v : TIME: %s", id, result, time.Since(start))
	return &result, nil
}

// Update обновляет уведомление в базе данных с указанными параметрами.
func (p *PostgresRepo) Update(ctx context.Context, id uuid.UUID, opts ...domain.UpdateOption) error {
	if len(opts) == 0 {
		return errors.New("no update options provided")
	}

	params := &domain.UpdateParams{}
	for _, opt := range opts {
		opt(params)
	}

	query, args, err := buildUpdateSQL(id, params)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Error build update sql notification")
		return err
	}

	result, err := p.DB.ExecContext(ctx, query, args...)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Error exec update sql notification")
		return err
	}
	rowAffected, _ := result.RowsAffected()
	if rowAffected == 0 {
		zlog.Logger.Warn().Msgf("Update notification id: %v No rows affected", id)
		return domain.ErrNoRowAffected
	}

	return nil
}

// ListPendingAndProcessingBefore получает список зависших уведомлений
// (статус pending или processing, обновленных до указанного времени).
func (p *PostgresRepo) ListPendingAndProcessingBefore(ctx context.Context, t time.Time,
	limit, offset int) ([]domain.Notification, error) {
	sqlQuery := `SELECT id, recipient, channel, payload, scheduled_at, status, retry_count, created_at, updated_at
    FROM notifications
    WHERE scheduled_at <= $1
      AND status = $2 OR (status = $3 AND updated_at < NOW() - INTERVAL '10 minutes')`

	if limit > 0 {
		sqlQuery += fmt.Sprintf(" LIMIT %d", limit)
	}
	if offset > 0 {
		sqlQuery += fmt.Sprintf(" OFFSET %d", offset)
	}

	rows, err := p.DB.QueryContext(ctx, sqlQuery, t, domain.StatusPending, domain.StatusProcessing)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Error exec list pending before sql")
		return nil, err
	}

	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	var n []domain.Notification

	for rows.Next() {
		var val domain.Notification
		var payloadRaw []byte

		err = rows.Scan(&val.ID, &val.Recipient,
			&val.Channel, &payloadRaw, &val.ScheduledAt,
			&val.Status, &val.RetryCount, &val.CreatedAt, &val.UpdatedAt)
		if err != nil {
			zlog.Logger.Error().Err(err).Msg("Error scan list pending before sql")
			return nil, err
		}

		err = json.Unmarshal(payloadRaw, &val.Payload)
		if err != nil {
			zlog.Logger.Error().Err(err).Msg("Error unmarshalling notification payload")
			return nil, err
		}

		n = append(n, val)
	}
	if len(n) == 0 {
		zlog.Logger.Debug().Msgf("No pending notifications found")
		return n, domain.ErrNotFound
	}
	return n, nil
}

// PendingToProcess изменяет статус уведомления с pending на processing.
func (p *PostgresRepo) PendingToProcess(ctx context.Context, id uuid.UUID) (bool, error) {
	sqlQuery := `UPDATE notifications SET status = $1 WHERE id = $2 AND status = $3`

	r, err := p.DB.ExecContext(ctx, sqlQuery, domain.StatusProcessing, id, domain.StatusPending)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Error exec pending to process notifications")
		return false, err
	}
	rows, _ := r.RowsAffected()
	return rows > 0, nil
}

// IncRetryCount увеличивает счетчик попыток для уведомления.
func (p *PostgresRepo) IncRetryCount(ctx context.Context, id uuid.UUID) error {
	sqlQuery := `UPDATE notifications SET retry_count = retry_count + 1 WHERE id = $1`

	r, err := p.DB.ExecContext(ctx, sqlQuery, id)
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Error exec retry count")
		return err
	}
	rows, _ := r.RowsAffected()
	if rows == 0 {
		return errors.New("no retry count found")
	}
	return nil
}
