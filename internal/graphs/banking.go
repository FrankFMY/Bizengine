package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var bankReconciliationList = arcana.GraphDef{
	Key: "bank_reconciliation_list",
	Deps: []arcana.TableDep{
		{Table: "bank_reconciliations", Columns: []string{"status", "matched", "unmatched", "iat"}},
	},
	Params: arcana.ParamSchema{
		"limit":  arcana.ParamInt().Default(50),
		"offset": arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")

		rows, err := q.Query(ctx,
			fmt.Sprintf(`SELECT id, date_from, date_to, total_entries, matched, unmatched, status, iat, COUNT(*) OVER() AS total_count
			 FROM bank_reconciliations WHERE organization_id = $1
			 ORDER BY iat DESC LIMIT $2 OFFSET $3`),
			orgID, limit, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()
		for rows.Next() {
			var id string
			var dateFrom, dateTo, iat any
			var total, matched, unmatched, cnt int
			var status string
			if err := rows.Scan(&id, &dateFrom, &dateTo, &total, &matched, &unmatched, &status, &iat, &cnt); err != nil {
				return nil, err
			}
			result.SetTotal(cnt)
			result.AddRef(arcana.Ref{Table: "bank_reconciliations", ID: id})
			result.AddRow("bank_reconciliations", id, map[string]any{
				"id": id, "date_from": dateFrom, "date_to": dateTo,
				"total_entries": total, "matched": matched, "unmatched": unmatched,
				"status": status, "iat": iat,
			})
		}
		return result, rows.Err()
	},
}

var bankReconciliationDetail = arcana.GraphDef{
	Key: "bank_reconciliation_detail",
	Deps: []arcana.TableDep{
		{Table: "bank_reconciliations", Columns: []string{"status", "matched", "unmatched"}},
		{Table: "bank_reconciliation_entries", Columns: []string{"status", "matched_type"}},
	},
	Params: arcana.ParamSchema{
		"reconciliation_id": arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		reconID := p.String("reconciliation_id")

		result := arcana.NewResult()

		var id string
		var dateFrom, dateTo, iat any
		var total, matched, unmatched int
		var status string
		err := q.QueryRow(ctx,
			`SELECT id, date_from, date_to, total_entries, matched, unmatched, status, iat
			 FROM bank_reconciliations WHERE id = $1 AND organization_id = $2`,
			reconID, orgID).Scan(&id, &dateFrom, &dateTo, &total, &matched, &unmatched, &status, &iat)
		if err != nil {
			return nil, err
		}
		result.AddRef(arcana.Ref{Table: "bank_reconciliations", ID: id})
		result.AddRow("bank_reconciliations", id, map[string]any{
			"id": id, "date_from": dateFrom, "date_to": dateTo,
			"total_entries": total, "matched": matched, "unmatched": unmatched,
			"status": status, "iat": iat,
		})

		entryRows, err := q.Query(ctx,
			`SELECT id, statement_entry, matched_type, matched_id, status
			 FROM bank_reconciliation_entries WHERE reconciliation_id = $1 ORDER BY iat`, reconID)
		if err != nil {
			return nil, err
		}
		defer entryRows.Close()

		for entryRows.Next() {
			var eid string
			var stEntry []byte
			var mType *string
			var mID *string
			var eStatus string
			if err := entryRows.Scan(&eid, &stEntry, &mType, &mID, &eStatus); err != nil {
				return nil, err
			}
			result.AddRef(arcana.Ref{Table: "bank_reconciliation_entries", ID: eid})
			result.AddRow("bank_reconciliation_entries", eid, map[string]any{
				"id": eid, "statement_entry": string(stEntry),
				"matched_type": mType, "matched_id": mID, "status": eStatus,
			})
		}
		return result, entryRows.Err()
	},
}
