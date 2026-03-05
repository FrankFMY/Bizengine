package export

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

type PDFExporter struct{}

func NewPDFExporter() *PDFExporter { return &PDFExporter{} }

type ReportData struct {
	Title    string
	Subtitle string
	Date     time.Time
	Columns  []Column
	Rows     []map[string]any
	Footer   string
}

// ExportText generates a plain-text report formatted for PDF-like output.
// For MVP this generates structured text. Can integrate a real PDF library (gofpdf, pdfcpu) later.
func (p *PDFExporter) ExportText(data ReportData) ([]byte, error) {
	var buf bytes.Buffer

	// Header
	buf.WriteString(strings.Repeat("=", 80) + "\n")
	buf.WriteString(centerText(data.Title, 80) + "\n")
	if data.Subtitle != "" {
		buf.WriteString(centerText(data.Subtitle, 80) + "\n")
	}
	buf.WriteString(centerText(data.Date.Format("02.01.2006"), 80) + "\n")
	buf.WriteString(strings.Repeat("=", 80) + "\n\n")

	// Column widths
	widths := make([]int, len(data.Columns))
	for i, col := range data.Columns {
		widths[i] = len(col.Header)
	}
	for _, row := range data.Rows {
		for i, col := range data.Columns {
			val := formatValue(row[col.Field])
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}
	for i := range widths {
		if widths[i] > 30 {
			widths[i] = 30
		}
	}

	// Header row
	for i, col := range data.Columns {
		buf.WriteString(padRight(col.Header, widths[i]) + " | ")
	}
	buf.WriteString("\n")
	for i := range data.Columns {
		buf.WriteString(strings.Repeat("-", widths[i]) + "-+-")
	}
	buf.WriteString("\n")

	// Data rows
	for _, row := range data.Rows {
		for i, col := range data.Columns {
			val := formatValue(row[col.Field])
			if len(val) > widths[i] {
				val = val[:widths[i]-1] + "…"
			}
			buf.WriteString(padRight(val, widths[i]) + " | ")
		}
		buf.WriteString("\n")
	}

	buf.WriteString("\n")
	if data.Footer != "" {
		buf.WriteString(data.Footer + "\n")
	}
	buf.WriteString(fmt.Sprintf("Generated: %s\n", time.Now().Format("02.01.2006 15:04")))

	return buf.Bytes(), nil
}

func centerText(s string, width int) string {
	if len(s) >= width {
		return s
	}
	pad := (width - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s[:width]
	}
	return s + strings.Repeat(" ", width-len(s))
}
