package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/documents"
)

// DocumentRepo provides data fetching for document generation.
type DocumentRepo struct {
	pool *pgxpool.Pool
}

// NewDocumentRepo creates a new DocumentRepo.
func NewDocumentRepo(pool *pgxpool.Pool) *DocumentRepo {
	return &DocumentRepo{pool: pool}
}

// GetOrderForDocument returns order data for rendering.
func (r *DocumentRepo) GetOrderForDocument(ctx context.Context, orgID, orderID uuid.UUID) (*documents.OrderData, error) {
	var order documents.OrderData
	var customerID *uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT id, COALESCE(meta->>'number', id::text), iat, status, total, customer_id
		 FROM orders WHERE organization_id = $1 AND id = $2`,
		orgID, orderID,
	).Scan(&order.ID, &order.Number, &order.Date, &order.Status, &order.Total, &customerID)
	if err != nil {
		return nil, fmt.Errorf("order not found: %w", err)
	}

	if customerID != nil {
		var name string
		_ = r.pool.QueryRow(ctx,
			`SELECT name FROM entities WHERE id = $1 AND organization_id = $2`,
			*customerID, orgID,
		).Scan(&name)
		order.CustomerName = name
	}

	order.TotalFormatted = fmt.Sprintf("%d.%02d", order.Total/100, order.Total%100)

	rows, err := r.pool.Query(ctx,
		`SELECT oi.id, COALESCE(e.name, ''), COALESCE(oi.sku, ''), oi.quantity, oi.unit_price, oi.total
		 FROM order_items oi
		 LEFT JOIN entities e ON e.id = oi.product_id
		 WHERE oi.order_id = $1
		 ORDER BY oi.iat`,
		orderID,
	)
	if err != nil {
		return &order, nil
	}
	defer rows.Close()

	idx := 0
	for rows.Next() {
		idx++
		var itemID uuid.UUID
		var name, sku string
		var qty int
		var unitPrice, total int64
		if err := rows.Scan(&itemID, &name, &sku, &qty, &unitPrice, &total); err != nil {
			continue
		}
		order.Items = append(order.Items, documents.OrderItemRow{
			Index:     idx,
			Name:      name,
			SKU:       sku,
			Quantity:  qty,
			UnitPrice: fmt.Sprintf("%d.%02d", unitPrice/100, unitPrice%100),
			Total:     fmt.Sprintf("%d.%02d", total/100, total%100),
			VATRate:   20,
		})
	}

	return &order, nil
}

// GetOrgRequisites returns organization requisites from settings.
func (r *DocumentRepo) GetOrgRequisites(ctx context.Context, orgID uuid.UUID) (*documents.OrgRequisites, error) {
	var name string
	var requisitesJSON json.RawMessage
	err := r.pool.QueryRow(ctx,
		`SELECT o.name, COALESCE(s.requisites, '{}')
		 FROM organizations o
		 LEFT JOIN organization_settings s ON s.organization_id = o.id
		 WHERE o.id = $1`,
		orgID,
	).Scan(&name, &requisitesJSON)
	if err != nil {
		return nil, err
	}

	var reqs map[string]any
	json.Unmarshal(requisitesJSON, &reqs)

	getString := func(key string) string {
		if v, ok := reqs[key].(string); ok {
			return v
		}
		return ""
	}

	return &documents.OrgRequisites{
		Name:    name,
		INN:     getString("inn"),
		KPP:     getString("kpp"),
		OGRN:    getString("ogrn"),
		Address: getString("address"),
		Phone:   getString("phone"),
		Bank:    getString("bank"),
		BIK:     getString("bik"),
		Account: getString("account"),
	}, nil
}

// GetProductTags returns product data for price tag rendering.
func (r *DocumentRepo) GetProductTags(ctx context.Context, orgID uuid.UUID, productIDs []uuid.UUID) ([]documents.ProductTag, error) {
	var tags []documents.ProductTag
	for _, pid := range productIDs {
		var name string
		err := r.pool.QueryRow(ctx,
			`SELECT name FROM entities WHERE id = $1 AND organization_id = $2`,
			pid, orgID,
		).Scan(&name)
		if err != nil {
			continue
		}

		var priceData json.RawMessage
		_ = r.pool.QueryRow(ctx,
			`SELECT data FROM components WHERE entity_id = $1 AND organization_id = $2 AND type = 'price'`,
			pid, orgID,
		).Scan(&priceData)

		var pd map[string]any
		json.Unmarshal(priceData, &pd)

		price := ""
		if sp, ok := pd["selling_price"].(float64); ok {
			kopecks := int64(sp)
			price = fmt.Sprintf("%d.%02d", kopecks/100, kopecks%100)
		}

		sku := ""
		barcode := ""
		unit := "шт"
		if v, ok := pd["unit"].(string); ok && v != "" {
			unit = v
		}

		var identData json.RawMessage
		_ = r.pool.QueryRow(ctx,
			`SELECT data FROM components WHERE entity_id = $1 AND organization_id = $2 AND type = 'identity'`,
			pid, orgID,
		).Scan(&identData)

		var id map[string]any
		json.Unmarshal(identData, &id)
		if v, ok := id["sku"].(string); ok {
			sku = v
		}
		if v, ok := id["barcode"].(string); ok {
			barcode = v
		}

		tags = append(tags, documents.ProductTag{
			Name:      name,
			SKU:       sku,
			Price:     price,
			Barcode:   barcode,
			Unit:      unit,
			ValidFrom: time.Now().Format("02.01.2006"),
		})
	}
	return tags, nil
}
