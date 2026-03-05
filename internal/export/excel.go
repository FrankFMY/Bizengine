package export

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"time"
)

type Column struct {
	Header string
	Field  string
}

type ExcelExporter struct{}

func NewExcelExporter() *ExcelExporter { return &ExcelExporter{} }

// ExportCSV generates a CSV file from tabular data.
// For MVP we use CSV which Excel opens natively. Can upgrade to xlsx later.
func (e *ExcelExporter) ExportCSV(columns []Column, rows []map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	// UTF-8 BOM for Excel compatibility
	buf.Write([]byte{0xEF, 0xBB, 0xBF})

	w := csv.NewWriter(&buf)
	w.Comma = ';'

	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.Header
	}
	if err := w.Write(headers); err != nil {
		return nil, fmt.Errorf("export: write headers: %w", err)
	}

	for _, row := range rows {
		record := make([]string, len(columns))
		for i, col := range columns {
			record[i] = formatValue(row[col.Field])
		}
		if err := w.Write(record); err != nil {
			return nil, fmt.Errorf("export: write row: %w", err)
		}
	}

	w.Flush()
	return buf.Bytes(), w.Error()
}

func formatValue(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%.2f", val)
	case bool:
		if val {
			return "Yes"
		}
		return "No"
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", val)
	}
}
