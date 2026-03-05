package graphs

import (
	"context"

	"github.com/FrankFMY/arcana"
)

var organizationSettings = arcana.GraphDef{
	Key: "organization_settings",
	Deps: []arcana.TableDep{
		{Table: "organization_settings", Columns: []string{"currency", "timezone", "features", "requisites"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)

		var currency, timezone, orderFmt string
		var requisites, integrations, features []byte
		err := q.QueryRow(ctx, `
			SELECT currency, timezone, order_number_format,
			       COALESCE(requisites, '{}'), COALESCE(integrations, '{}'), COALESCE(features, '{}')
			FROM organization_settings WHERE organization_id = $1
		`, orgID).Scan(&currency, &timezone, &orderFmt, &requisites, &integrations, &features)
		if err != nil {
			result := arcana.NewResult()
			result.AddRow("organization_settings", orgID, map[string]any{
				"currency": "RUB", "timezone": "Europe/Moscow", "order_number_format": "ORD-{SEQ}",
			})
			return result, nil
		}

		result := arcana.NewResult()
		result.AddRef(arcana.Ref{Table: "organization_settings", ID: orgID, Fields: []string{"currency", "timezone", "features"}})
		result.AddRow("organization_settings", orgID, map[string]any{
			"organization_id":    orgID,
			"currency":           currency,
			"timezone":           timezone,
			"order_number_format": orderFmt,
			"requisites":         string(requisites),
			"integrations":       string(integrations),
			"features":           string(features),
		})
		return result, nil
	},
}
