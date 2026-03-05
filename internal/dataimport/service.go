// Package dataimport provides mass data import from CSV/XLSX files.
package dataimport

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
)

// ImportResult holds the outcome of an import operation.
type ImportResult struct {
	Total    int           `json:"total"`
	Imported int           `json:"imported"`
	Errors   []ImportError `json:"errors"`
}

// ImportError describes a row-level error.
type ImportError struct {
	Row   int    `json:"row"`
	Field string `json:"field"`
	Error string `json:"error"`
}

// Service provides import operations.
type Service struct {
	entitySvc *entity.Service
	bus       event.Bus
}

// NewService creates a new import service.
func NewService(entitySvc *entity.Service, bus event.Bus) *Service {
	return &Service{entitySvc: entitySvc, bus: bus}
}

// ImportProducts imports products from CSV or XLSX data.
// Expected columns: name, sku, category, selling_price, purchase_price, unit, barcode
func (s *Service) ImportProducts(ctx context.Context, orgID uuid.UUID, data []byte, filename string, actorID *uuid.UUID) (*ImportResult, error) {
	rows, err := s.parseFile(data, filename)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, errs.NewBadRequest("file has no data rows")
	}

	header := normalizeHeader(rows[0])
	colIdx := mapColumns(header, []string{"name", "sku", "category", "selling_price", "purchase_price", "unit", "barcode"})

	result := &ImportResult{Total: len(rows) - 1}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		name := getCol(row, colIdx["name"])
		if name == "" {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Field: "name", Error: "REQUIRED"})
			continue
		}

		sku := getCol(row, colIdx["sku"])
		category := getCol(row, colIdx["category"])
		sellingPrice := parsePrice(getCol(row, colIdx["selling_price"]))
		purchasePrice := parsePrice(getCol(row, colIdx["purchase_price"]))
		unit := getCol(row, colIdx["unit"])
		barcode := getCol(row, colIdx["barcode"])

		if sellingPrice < 0 {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Field: "selling_price", Error: "INVALID"})
			continue
		}

		if unit == "" {
			unit = "шт"
		}

		e, err := s.entitySvc.Create(ctx, orgID, entity.CreateEntityInput{
			Kind: "product",
			Name: name,
		}, actorID)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Field: "name", Error: err.Error()})
			continue
		}

		if category != "" {
			meta, _ := json.Marshal(map[string]any{"category": category})
			s.entitySvc.SetComponent(ctx, orgID, e.ID, "category", meta, actorID)
		}

		identData, _ := json.Marshal(map[string]any{"sku": sku, "barcode": barcode})
		s.entitySvc.SetComponent(ctx, orgID, e.ID, "identity", identData, actorID)

		priceData, _ := json.Marshal(map[string]any{
			"selling_price":  sellingPrice,
			"purchase_price": purchasePrice,
			"currency":       "RUB",
			"unit":           unit,
		})
		s.entitySvc.SetComponent(ctx, orgID, e.ID, "price", priceData, actorID)

		result.Imported++
	}

	return result, nil
}

// ImportCustomers imports customers from CSV or XLSX data.
// Expected columns: name, phone, email, tags
func (s *Service) ImportCustomers(ctx context.Context, orgID uuid.UUID, data []byte, filename string, actorID *uuid.UUID) (*ImportResult, error) {
	rows, err := s.parseFile(data, filename)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, errs.NewBadRequest("file has no data rows")
	}

	header := normalizeHeader(rows[0])
	colIdx := mapColumns(header, []string{"name", "phone", "email", "tags"})

	result := &ImportResult{Total: len(rows) - 1}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		name := getCol(row, colIdx["name"])
		if name == "" {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Field: "name", Error: "REQUIRED"})
			continue
		}

		phone := getCol(row, colIdx["phone"])
		email := getCol(row, colIdx["email"])
		tagsStr := getCol(row, colIdx["tags"])

		components := make(map[string]json.RawMessage)
		contactData, _ := json.Marshal(map[string]any{"phone": phone, "email": email})
		components["contact"] = contactData

		var tags []string
		if tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ";") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}
		profileData, _ := json.Marshal(map[string]any{"tags": tags})
		components["crm_profile"] = profileData

		balanceData, _ := json.Marshal(map[string]any{"balance": 0, "credit_limit": 0, "payment_terms_days": 0})
		components["crm_balance"] = balanceData

		_, err := s.entitySvc.CreateWithComponents(ctx, orgID, entity.CreateEntityInput{
			Kind: "customer",
			Name: name,
		}, components, actorID)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Field: "name", Error: err.Error()})
			continue
		}

		result.Imported++
	}

	return result, nil
}

// ImportSuppliers imports suppliers from CSV or XLSX data.
// Expected columns: name, inn, phone, email, contact_person
func (s *Service) ImportSuppliers(ctx context.Context, orgID uuid.UUID, data []byte, filename string, actorID *uuid.UUID) (*ImportResult, error) {
	rows, err := s.parseFile(data, filename)
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, errs.NewBadRequest("file has no data rows")
	}

	header := normalizeHeader(rows[0])
	colIdx := mapColumns(header, []string{"name", "inn", "phone", "email", "contact_person"})

	result := &ImportResult{Total: len(rows) - 1}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		rowNum := i + 1

		name := getCol(row, colIdx["name"])
		if name == "" {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Field: "name", Error: "REQUIRED"})
			continue
		}

		inn := getCol(row, colIdx["inn"])
		phone := getCol(row, colIdx["phone"])
		email := getCol(row, colIdx["email"])
		contactPerson := getCol(row, colIdx["contact_person"])

		components := make(map[string]json.RawMessage)
		contactData, _ := json.Marshal(map[string]any{
			"phone":          phone,
			"email":          email,
			"contact_person": contactPerson,
			"inn":            inn,
		})
		components["contact"] = contactData

		profileData, _ := json.Marshal(map[string]any{"tags": []string{}})
		components["crm_profile"] = profileData

		balanceData, _ := json.Marshal(map[string]any{"balance": 0, "credit_limit": 0, "payment_terms_days": 0})
		components["crm_balance"] = balanceData

		_, err := s.entitySvc.CreateWithComponents(ctx, orgID, entity.CreateEntityInput{
			Kind: "supplier",
			Name: name,
		}, components, actorID)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{Row: rowNum, Field: "name", Error: err.Error()})
			continue
		}

		result.Imported++
	}

	return result, nil
}

func (s *Service) parseFile(data []byte, filename string) ([][]string, error) {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".xlsx") {
		return s.parseXLSX(data)
	}
	return s.parseCSV(data)
}

func (s *Service) parseCSV(data []byte) ([][]string, error) {
	// Try UTF-8 first
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = ','
	r.LazyQuotes = true

	records, err := r.ReadAll()
	if err != nil {
		// Try semicolon separator
		r = csv.NewReader(bytes.NewReader(data))
		r.Comma = ';'
		r.LazyQuotes = true
		records, err = r.ReadAll()
	}

	if err != nil {
		// Try Windows-1251 encoding
		decoded, decErr := io.ReadAll(transform.NewReader(bytes.NewReader(data), charmap.Windows1251.NewDecoder()))
		if decErr != nil {
			return nil, fmt.Errorf("failed to parse CSV: %w", err)
		}
		r = csv.NewReader(bytes.NewReader(decoded))
		r.Comma = ';'
		r.LazyQuotes = true
		records, err = r.ReadAll()
		if err != nil {
			r = csv.NewReader(bytes.NewReader(decoded))
			r.Comma = ','
			r.LazyQuotes = true
			records, err = r.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("failed to parse CSV: %w", err)
			}
		}
	}

	return records, nil
}

func (s *Service) parseXLSX(data []byte) ([][]string, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open XLSX: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errs.NewBadRequest("XLSX has no sheets")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read XLSX sheet: %w", err)
	}

	return rows, nil
}

func normalizeHeader(row []string) []string {
	result := make([]string, len(row))
	for i, h := range row {
		result[i] = strings.TrimSpace(strings.ToLower(h))
	}
	return result
}

func mapColumns(header []string, expected []string) map[string]int {
	m := make(map[string]int)
	for _, col := range expected {
		m[col] = -1
	}
	for i, h := range header {
		for _, col := range expected {
			if h == col {
				m[col] = i
			}
		}
	}
	return m
}

func getCol(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

func parsePrice(s string) int64 {
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ",", ".")

	if strings.Contains(s, ".") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return -1
		}
		return int64(f * 100)
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return -1
	}
	return n
}
