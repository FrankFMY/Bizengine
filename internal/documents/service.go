// Package documents provides HTML/PDF document generation for business forms.
package documents

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/errs"
)

//go:embed templates/*.html
var templateFS embed.FS

// OrderData represents order data for document rendering.
type OrderData struct {
	ID             uuid.UUID      `json:"id"`
	Number         string         `json:"number"`
	Date           time.Time      `json:"date"`
	CustomerName   string         `json:"customer_name"`
	CustomerPhone  string         `json:"customer_phone"`
	Items          []OrderItemRow `json:"items"`
	Subtotal       int64          `json:"subtotal"`
	VATAmount      int64          `json:"vat_amount"`
	Total          int64          `json:"total"`
	TotalFormatted string         `json:"total_formatted"`
	Status         string         `json:"status"`
}

// OrderItemRow is a line item in a document.
type OrderItemRow struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
	UnitPrice string `json:"unit_price"`
	Total     string `json:"total"`
	VATRate   int    `json:"vat_rate"`
}

// OrgRequisites holds organization details for document headers.
type OrgRequisites struct {
	Name    string `json:"name"`
	INN     string `json:"inn"`
	KPP     string `json:"kpp"`
	OGRN    string `json:"ogrn"`
	Address string `json:"address"`
	Phone   string `json:"phone"`
	Bank    string `json:"bank"`
	BIK     string `json:"bik"`
	Account string `json:"account"`
}

// ProductTag holds data for price tag rendering.
type ProductTag struct {
	Name      string `json:"name"`
	SKU       string `json:"sku"`
	Price     string `json:"price"`
	Barcode   string `json:"barcode"`
	Unit      string `json:"unit"`
	ValidFrom string `json:"valid_from"`
}

// DocumentData wraps all data needed for template rendering.
type DocumentData struct {
	Order       *OrderData     `json:"order,omitempty"`
	Org         *OrgRequisites `json:"org,omitempty"`
	Products    []ProductTag   `json:"products,omitempty"`
	GeneratedAt string         `json:"generated_at"`
}

// OrderFetcher retrieves order data for document generation.
type OrderFetcher interface {
	GetOrderForDocument(ctx context.Context, orgID, orderID uuid.UUID) (*OrderData, error)
}

// OrgFetcher retrieves organization requisites.
type OrgFetcher interface {
	GetOrgRequisites(ctx context.Context, orgID uuid.UUID) (*OrgRequisites, error)
}

// ProductFetcher retrieves product data for price tags.
type ProductFetcher interface {
	GetProductTags(ctx context.Context, orgID uuid.UUID, productIDs []uuid.UUID) ([]ProductTag, error)
}

// Service generates HTML documents from templates.
type Service struct {
	templates    *template.Template
	orderFetch   OrderFetcher
	orgFetch     OrgFetcher
	productFetch ProductFetcher
}

// NewService creates a new document service.
func NewService(orderFetch OrderFetcher, orgFetch OrgFetcher, productFetch ProductFetcher) (*Service, error) {
	funcMap := template.FuncMap{
		"kopecksToRubles": func(k int64) string {
			return fmt.Sprintf("%d.%02d", k/100, k%100)
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return &Service{
		templates:    tmpl,
		orderFetch:   orderFetch,
		orgFetch:     orgFetch,
		productFetch: productFetch,
	}, nil
}

// RenderInvoice generates an invoice HTML for an order.
func (s *Service) RenderInvoice(ctx context.Context, orgID, orderID uuid.UUID) ([]byte, error) {
	return s.renderOrderDoc(ctx, orgID, orderID, "invoice.html")
}

// RenderTORG12 generates a TORG-12 waybill HTML for an order.
func (s *Service) RenderTORG12(ctx context.Context, orgID, orderID uuid.UUID) ([]byte, error) {
	return s.renderOrderDoc(ctx, orgID, orderID, "torg12.html")
}

// RenderAct generates an act HTML for an order.
func (s *Service) RenderAct(ctx context.Context, orgID, orderID uuid.UUID) ([]byte, error) {
	return s.renderOrderDoc(ctx, orgID, orderID, "act.html")
}

// RenderReceipt generates a receipt HTML for an order.
func (s *Service) RenderReceipt(ctx context.Context, orgID, orderID uuid.UUID) ([]byte, error) {
	return s.renderOrderDoc(ctx, orgID, orderID, "receipt.html")
}

// RenderPriceTags generates price tags HTML for given products.
func (s *Service) RenderPriceTags(ctx context.Context, orgID uuid.UUID, productIDs []uuid.UUID) ([]byte, error) {
	if len(productIDs) == 0 {
		return nil, errs.NewBadRequest("product_ids is required")
	}

	products, err := s.productFetch.GetProductTags(ctx, orgID, productIDs)
	if err != nil {
		return nil, err
	}

	org, _ := s.orgFetch.GetOrgRequisites(ctx, orgID)

	data := DocumentData{
		Products:    products,
		Org:         org,
		GeneratedAt: time.Now().Format("02.01.2006 15:04"),
	}

	return s.render("price_tag.html", data)
}

func (s *Service) renderOrderDoc(ctx context.Context, orgID, orderID uuid.UUID, templateName string) ([]byte, error) {
	order, err := s.orderFetch.GetOrderForDocument(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}

	org, _ := s.orgFetch.GetOrgRequisites(ctx, orgID)

	data := DocumentData{
		Order:       order,
		Org:         org,
		GeneratedAt: time.Now().Format("02.01.2006 15:04"),
	}

	return s.render(templateName, data)
}

func (s *Service) render(templateName string, data any) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, templateName, data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", templateName, err)
	}
	return buf.Bytes(), nil
}
