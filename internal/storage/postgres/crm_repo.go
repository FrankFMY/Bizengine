package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/pkg/types"
)

// CRMRepo implements crm.Repository.
type CRMRepo struct {
	pool *pgxpool.Pool
}

// NewCRMRepo creates a new CRMRepo.
func NewCRMRepo(pool *pgxpool.Pool) *CRMRepo {
	return &CRMRepo{pool: pool}
}

// GetOrdersByCustomer returns orders for a customer.
func (r *CRMRepo) GetOrdersByCustomer(ctx context.Context, orgID, customerID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error) {
	page.Normalize()

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE organization_id = $1 AND customer_id = $2`,
		orgID, customerID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT row_to_json(o) FROM (
			SELECT id, status, total, iat AS created_at
			FROM orders
			WHERE organization_id = $1 AND customer_id = $2
			ORDER BY iat DESC
			LIMIT $3 OFFSET $4
		) o`,
		orgID, customerID, page.Limit, page.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []json.RawMessage
	for rows.Next() {
		var data json.RawMessage
		if err := rows.Scan(&data); err != nil {
			return nil, 0, err
		}
		result = append(result, data)
	}
	if result == nil {
		result = []json.RawMessage{}
	}
	return result, total, nil
}

// GetTransactionsByCounterparty returns financial transactions referencing this counterparty.
func (r *CRMRepo) GetTransactionsByCounterparty(ctx context.Context, orgID, counterpartyID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error) {
	page.Normalize()

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM transactions WHERE organization_id = $1 AND reference_id = $2`,
		orgID, counterpartyID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT row_to_json(t) FROM (
			SELECT id, description, amount, iat AS created_at
			FROM transactions
			WHERE organization_id = $1 AND reference_id = $2
			ORDER BY iat DESC
			LIMIT $3 OFFSET $4
		) t`,
		orgID, counterpartyID, page.Limit, page.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []json.RawMessage
	for rows.Next() {
		var data json.RawMessage
		if err := rows.Scan(&data); err != nil {
			return nil, 0, err
		}
		result = append(result, data)
	}
	if result == nil {
		result = []json.RawMessage{}
	}
	return result, total, nil
}

// GetDeliveriesBySupplier returns stock movements (receipts) from a supplier.
func (r *CRMRepo) GetDeliveriesBySupplier(ctx context.Context, orgID, supplierID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error) {
	page.Normalize()

	var total int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM stock_movements WHERE organization_id = $1 AND type = 'receipt' AND reference_id = $2`,
		orgID, supplierID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.pool.Query(ctx,
		`SELECT row_to_json(d) FROM (
			SELECT id, product_id, quantity, cost_per_unit, iat AS created_at
			FROM stock_movements
			WHERE organization_id = $1 AND type = 'receipt' AND reference_id = $2
			ORDER BY iat DESC
			LIMIT $3 OFFSET $4
		) d`,
		orgID, supplierID, page.Limit, page.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []json.RawMessage
	for rows.Next() {
		var data json.RawMessage
		if err := rows.Scan(&data); err != nil {
			return nil, 0, err
		}
		result = append(result, data)
	}
	if result == nil {
		result = []json.RawMessage{}
	}
	return result, total, nil
}
