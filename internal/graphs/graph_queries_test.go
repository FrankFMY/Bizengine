package graphs

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/FrankFMY/arcana"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type captureQuerier struct {
	sql  string
	args []any
	rows arcana.Rows
}

func (q *captureQuerier) Query(_ context.Context, sql string, args ...any) (arcana.Rows, error) {
	q.sql = sql
	q.args = args
	if q.rows != nil {
		return q.rows, nil
	}
	return &fakeRows{}, nil
}

func (q *captureQuerier) QueryRow(context.Context, string, ...any) arcana.Row {
	return fakeRow{}
}

type fakeRows struct {
	values [][]any
	index  int
}

func (r *fakeRows) Next() bool {
	return r.index < len(r.values)
}

func (r *fakeRows) Scan(dest ...any) error {
	if r.index >= len(r.values) {
		return fmt.Errorf("scan called after rows exhausted")
	}
	values := r.values[r.index]
	r.index++
	if len(dest) != len(values) {
		return fmt.Errorf("scan destination count %d does not match values count %d", len(dest), len(values))
	}
	for i, d := range dest {
		if err := assignScanDest(d, values[i]); err != nil {
			return err
		}
	}
	return nil
}

func (r *fakeRows) Close()     {}
func (r *fakeRows) Err() error { return nil }

type fakeRow struct{}

func (fakeRow) Scan(...any) error {
	return fmt.Errorf("unexpected QueryRow call")
}

func assignScanDest(dest any, value any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("scan destination must be a non-nil pointer")
	}
	target := rv.Elem()
	if value == nil {
		target.Set(reflect.Zero(target.Type()))
		return nil
	}
	v := reflect.ValueOf(value)
	if v.Type().AssignableTo(target.Type()) {
		target.Set(v)
		return nil
	}
	if v.Type().ConvertibleTo(target.Type()) {
		target.Set(v.Convert(target.Type()))
		return nil
	}
	if target.Kind() == reflect.Interface {
		target.Set(v)
		return nil
	}
	return fmt.Errorf("cannot assign %T to %s", value, target.Type())
}

func testArcanaContext() context.Context {
	return arcana.WithIdentity(context.Background(), &arcana.Identity{
		WorkspaceID: "org-1",
		UserID:      "user-1",
		SeanceID:    "seance-1",
	})
}

func TestCRMCustomersListAppliesDeclaredFilters(t *testing.T) {
	q := &captureQuerier{}

	_, err := crmCustomersList.Factory(testArcanaContext(), q, arcana.NewParams(map[string]any{
		"tag":      "vip",
		"category": "retail",
		"limit":    50,
		"offset":   0,
	}))

	require.NoError(t, err)
	assert.Contains(t, q.sql, `COALESCE(cp.data->'tags', '[]'::jsonb) ? $2`)
	assert.Contains(t, q.sql, `cp.data->>'category' = $3`)
	assert.Contains(t, strings.Join(strings.Fields(q.sql), " "), "ORDER BY e.name LIMIT $4 OFFSET $5")
	require.Len(t, q.args, 5)
	assert.Equal(t, "org-1", q.args[0])
	assert.Equal(t, "vip", q.args[1])
	assert.Equal(t, "retail", q.args[2])
}

func TestNotificationsListUsesSeverityColumn(t *testing.T) {
	q := &captureQuerier{rows: &fakeRows{values: [][]any{{
		"notif-1",
		"warning",
		"Low stock",
		"Only two items left",
		false,
		"2026-05-24T10:00:00Z",
		1,
	}}}}

	result, err := notificationsList.Factory(testArcanaContext(), q, arcana.NewParams(map[string]any{
		"limit":  50,
		"offset": 0,
	}))

	require.NoError(t, err)
	assert.Contains(t, q.sql, "SELECT id, severity, title, body, read, iat")
	assert.NotContains(t, q.sql, "SELECT id, type, title")
	row := result.Tables()["notifications"]["notif-1"]
	assert.Equal(t, "warning", row["severity"])
	assert.NotContains(t, row, "type")
}
