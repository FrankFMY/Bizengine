package export

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportCSV(t *testing.T) {
	e := NewExcelExporter()

	cols := []Column{
		{Header: "Name", Field: "name"},
		{Header: "Quantity", Field: "qty"},
		{Header: "Price", Field: "price"},
	}
	rows := []map[string]any{
		{"name": "Widget", "qty": int64(10), "price": 99.5},
		{"name": "Gadget", "qty": int64(5), "price": 149.99},
	}

	data, err := e.ExportCSV(cols, rows)
	require.NoError(t, err)

	csv := string(data)
	// BOM
	assert.True(t, strings.HasPrefix(csv, "\xEF\xBB\xBF"))

	lines := strings.Split(strings.TrimSpace(csv[3:]), "\n")
	assert.Len(t, lines, 3)
	assert.Equal(t, "Name;Quantity;Price", lines[0])
	assert.Equal(t, "Widget;10;99.50", lines[1])
	assert.Equal(t, "Gadget;5;149.99", lines[2])
}

func TestExportCSV_Empty(t *testing.T) {
	e := NewExcelExporter()

	data, err := e.ExportCSV([]Column{{Header: "A", Field: "a"}}, nil)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data[3:])), "\n")
	assert.Len(t, lines, 1) // header only
}

func TestExportText(t *testing.T) {
	p := NewPDFExporter()

	data, err := p.ExportText(ReportData{
		Title:    "Sales Report",
		Subtitle: "Q1 2026",
		Date:     time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Columns: []Column{
			{Header: "Product", Field: "product"},
			{Header: "Revenue", Field: "revenue"},
		},
		Rows: []map[string]any{
			{"product": "Widget", "revenue": int64(50000)},
			{"product": "Gadget", "revenue": int64(30000)},
		},
		Footer: "Total: 80000",
	})
	require.NoError(t, err)

	text := string(data)
	assert.Contains(t, text, "Sales Report")
	assert.Contains(t, text, "Q1 2026")
	assert.Contains(t, text, "Widget")
	assert.Contains(t, text, "50000")
	assert.Contains(t, text, "Total: 80000")
}
