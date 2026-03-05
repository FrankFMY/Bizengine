package export

import (
	"context"
	"time"
)

type Service struct {
	csv *ExcelExporter
	pdf *PDFExporter
}

func NewService() *Service {
	return &Service{
		csv: NewExcelExporter(),
		pdf: NewPDFExporter(),
	}
}

type ExportRequest struct {
	Format  string           `json:"format"` // csv, pdf
	Title   string           `json:"title"`
	Columns []Column         `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

type ExportResult struct {
	Data        []byte `json:"-"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
}

func (s *Service) Export(_ context.Context, req ExportRequest) (*ExportResult, error) {
	now := time.Now().Format("20060102_1504")

	switch req.Format {
	case "pdf":
		data, err := s.pdf.ExportText(ReportData{
			Title:   req.Title,
			Date:    time.Now(),
			Columns: req.Columns,
			Rows:    req.Rows,
		})
		if err != nil {
			return nil, err
		}
		return &ExportResult{
			Data:        data,
			ContentType: "text/plain; charset=utf-8",
			Filename:    req.Title + "_" + now + ".txt",
		}, nil

	default: // csv
		data, err := s.csv.ExportCSV(req.Columns, req.Rows)
		if err != nil {
			return nil, err
		}
		return &ExportResult{
			Data:        data,
			ContentType: "text/csv; charset=utf-8",
			Filename:    req.Title + "_" + now + ".csv",
		}, nil
	}
}
