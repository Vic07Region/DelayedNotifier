package pg

import (
	"encoding/json"
	"fmt"
	"strings"

	"DelayedNotifier/internal/domain"
	"github.com/google/uuid"
)

// buildUpdateSQL строит SQL запрос для обновления уведомления.
func buildUpdateSQL(id uuid.UUID, params *domain.UpdateParams) (string, []interface{}, error) {
	var (
		sets   []string
		args   []interface{}
		argIdx = 1
	)

	if params.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *params.Status)
		argIdx++
	}
	if params.RetryCountInc != nil {
		sets = append(sets, "retry_count = retry_count + 1")
	}
	if params.ScheduledAt != nil {
		sets = append(sets, fmt.Sprintf("scheduled_at = $%d", argIdx))
		args = append(args, *params.ScheduledAt)
		argIdx++
	}
	if params.Channel != nil {
		sets = append(sets, fmt.Sprintf("channel = $%d", argIdx))
		args = append(args, *params.Channel)
		argIdx++
	}
	if params.Payload != nil && params.Payload.Set {
		jsonData, err := json.Marshal(params.Payload.Value)
		if err != nil {
			return "", nil, err
		}
		sets = append(sets, fmt.Sprintf("payload = $%d", argIdx))
		args = append(args, jsonData)
		argIdx++
	}
	if len(sets) == 0 {
		return "", nil, fmt.Errorf("no fields to update")
	}
	query := fmt.Sprintf("UPDATE notifications SET %s WHERE id = $%d",
		strings.Join(sets, ", "), argIdx) //nolint:nolint
	args = append(args, id)

	return query, args, nil
}
