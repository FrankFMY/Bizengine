package banking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/internal/core/event"
	adapter "github.com/bizengine/engine/internal/integration/banking"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// FinanceAutoTxCreator creates finance transactions from events.
type FinanceAutoTxCreator interface {
	CreateAutoTransaction(ctx context.Context, orgID uuid.UUID, date, description, debitCode, creditCode string, amount int64, refType *string, refID *uuid.UUID) error
}

// Service provides banking operations.
type Service struct {
	repo       Repository
	adapter    adapter.BankAdapter
	bus        event.Bus
	financeSvc FinanceAutoTxCreator
}

// NewService creates a new banking service.
func NewService(repo Repository, bankAdapter adapter.BankAdapter, bus event.Bus, financeSvc FinanceAutoTxCreator) *Service {
	return &Service{
		repo:       repo,
		adapter:    bankAdapter,
		bus:        bus,
		financeSvc: financeSvc,
	}
}

// InitiatePayment starts a payment through the bank adapter.
func (s *Service) InitiatePayment(ctx context.Context, orgID uuid.UUID, input InitiatePaymentInput) (*adapter.PaymentResult, error) {
	if input.Amount <= 0 {
		return nil, errs.NewBadRequest("amount must be positive")
	}
	if input.OrderID == nil {
		return nil, errs.NewBadRequest("order_id is required")
	}

	currency := input.Currency
	if currency == "" {
		currency = "RUB"
	}

	result, err := s.adapter.InitiatePayment(ctx, adapter.PaymentRequest{
		OrganizationID:   orgID,
		Amount:           input.Amount,
		Currency:         currency,
		Purpose:          input.Purpose,
		RecipientINN:     input.RecipientINN,
		RecipientBIC:     input.RecipientBIC,
		RecipientAccount: input.RecipientAccount,
		OrderID:          input.OrderID,
	})
	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, nil, "bank.payment.initiated", map[string]any{
		"payment_id": result.PaymentID,
		"amount":     input.Amount,
		"order_id":   input.OrderID,
		"status":     result.Status,
	})

	return result, nil
}

// GetPaymentStatus returns the current status of a payment.
func (s *Service) GetPaymentStatus(ctx context.Context, paymentID string) (*adapter.PaymentStatus, error) {
	return s.adapter.GetPaymentStatus(ctx, paymentID)
}

// HandlePaymentCallback processes a bank payment callback.
func (s *Service) HandlePaymentCallback(ctx context.Context, orgID uuid.UUID, callback PaymentCallback) error {
	if callback.Status == "completed" {
		s.publishEvent(ctx, orgID, callback.OrderID, "bank.payment.completed", map[string]any{
			"payment_id": callback.PaymentID,
			"amount":     callback.Amount,
			"order_id":   callback.OrderID,
			"method":     callback.Method,
		})
	} else if callback.Status == "failed" {
		s.publishEvent(ctx, orgID, callback.OrderID, "bank.payment.failed", map[string]any{
			"payment_id": callback.PaymentID,
			"order_id":   callback.OrderID,
			"reason":     callback.FailReason,
		})
	}
	return nil
}

// AutoReconcile performs automatic bank statement reconciliation.
func (s *Service) AutoReconcile(ctx context.Context, orgID uuid.UUID) (*ReconciliationResult, error) {
	dateTo := time.Now()
	dateFrom := dateTo.AddDate(0, 0, -7)

	statement, err := s.adapter.GetStatement(ctx, adapter.StatementRequest{
		OrganizationID: orgID,
		DateFrom:       dateFrom,
		DateTo:         dateTo,
	})
	if err != nil {
		return nil, fmt.Errorf("get statement: %w", err)
	}

	var bankPartnerID *uuid.UUID
	bp, err := s.repo.GetOrgBankPartner(ctx, orgID)
	if err == nil && bp != nil {
		bankPartnerID = &bp.ID
	}

	recon := &Reconciliation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		BankPartnerID:  bankPartnerID,
		DateFrom:       dateFrom,
		DateTo:         dateTo,
		TotalEntries:   len(statement.Entries),
		Status:         "completed",
	}

	var entries []ReconciliationEntry
	matched := 0
	unmatched := 0

	err = s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateReconciliation(ctx, tx, recon); err != nil {
			return err
		}

		for _, se := range statement.Entries {
			entry := ReconciliationEntry{
				ID:               uuid.New(),
				ReconciliationID: recon.ID,
				OrganizationID:   orgID,
				StatementEntry: map[string]any{
					"date":              se.Date,
					"amount":            se.Amount,
					"direction":         se.Direction,
					"purpose":           se.Purpose,
					"counterparty_inn":  se.CounterpartyINN,
					"counterparty_name": se.CounterpartyName,
					"document_number":   se.DocumentNumber,
				},
				Status: "pending",
			}

			if se.Direction == "credit" {
				orderID, err := s.repo.FindOrderByAmountAndINN(ctx, orgID, se.Amount, se.CounterpartyINN)
				if err == nil && orderID != nil {
					matchType := "order"
					entry.MatchedType = &matchType
					entry.MatchedID = orderID
					entry.Status = "matched"

					date := se.Date.Format("2006-01-02")
					refType := "reconciliation"
					if s.financeSvc != nil {
						_ = s.financeSvc.CreateAutoTransaction(ctx, orgID, date, "Bank reconciliation: "+se.Purpose, "51", "62", se.Amount, &refType, orderID)
					}
					matched++
				} else {
					entry.Status = "manual"
					unmatched++
				}
			} else {
				entry.Status = "manual"
				unmatched++
			}

			if err := s.repo.CreateReconciliationEntry(ctx, tx, &entry); err != nil {
				return err
			}
			entries = append(entries, entry)
		}

		recon.Matched = matched
		recon.Unmatched = unmatched
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, nil, "bank.reconciliation.completed", map[string]any{
		"reconciliation_id": recon.ID,
		"total":             recon.TotalEntries,
		"matched":           matched,
		"unmatched":         unmatched,
	})

	return &ReconciliationResult{
		Reconciliation: *recon,
		Entries:        entries,
	}, nil
}

// GetReconciliation returns a reconciliation with its entries.
func (s *Service) GetReconciliation(ctx context.Context, orgID, id uuid.UUID) (*ReconciliationResult, error) {
	recon, err := s.repo.GetReconciliation(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.GetReconciliationEntries(ctx, id)
	if err != nil {
		return nil, err
	}
	return &ReconciliationResult{
		Reconciliation: *recon,
		Entries:        entries,
	}, nil
}

// ListReconciliations returns reconciliations for an organization.
func (s *Service) ListReconciliations(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]Reconciliation, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.ListReconciliations(ctx, orgID, limit, offset)
}

// PayViaBank sends payroll batch through the bank.
func (s *Service) PayViaBank(ctx context.Context, orgID uuid.UUID, batch adapter.PayrollBatch) (*adapter.PayrollBatchResult, error) {
	if len(batch.Employees) == 0 {
		return nil, errs.NewBadRequest("no employees in batch")
	}

	batch.OrganizationID = orgID
	result, err := s.adapter.CreatePayrollBatch(ctx, batch)
	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, nil, "bank.payroll.sent", map[string]any{
		"batch_id":  result.BatchID,
		"count":     result.Count,
		"month":     batch.Month,
		"year":      batch.Year,
	})

	return result, nil
}

// GetAuthURL returns the bank OAuth authorization URL.
func (s *Service) GetAuthURL(ctx context.Context, redirectURL string) (string, error) {
	return s.adapter.GetAuthURL(ctx, redirectURL)
}

// ExchangeCode exchanges an OAuth code for bank user info.
func (s *Service) ExchangeCode(ctx context.Context, code string) (*adapter.BankUser, error) {
	return s.adapter.ExchangeCode(ctx, code)
}

// GetWhiteLabel returns the white-label branding for an organization.
func (s *Service) GetWhiteLabel(ctx context.Context, orgID uuid.UUID) (*WhiteLabel, error) {
	bp, err := s.repo.GetOrgBankPartner(ctx, orgID)
	if err != nil || bp == nil {
		return &WhiteLabel{
			AppName:  "BizEngine",
			BankName: "",
		}, nil
	}
	return &bp.WhiteLabel, nil
}

// InitiatePaymentInput is the input for payment initiation.
type InitiatePaymentInput struct {
	OrderID          *uuid.UUID `json:"order_id"`
	Amount           int64      `json:"amount"`
	Currency         string     `json:"currency"`
	Method           string     `json:"method"`
	Purpose          string     `json:"purpose"`
	RecipientINN     string     `json:"recipient_inn"`
	RecipientBIC     string     `json:"recipient_bic"`
	RecipientAccount string     `json:"recipient_account"`
}

// PaymentCallback is the payload from a bank payment webhook.
type PaymentCallback struct {
	PaymentID  string     `json:"payment_id"`
	OrderID    *uuid.UUID `json:"order_id,omitempty"`
	Amount     int64      `json:"amount"`
	Status     string     `json:"status"`
	Method     string     `json:"method"`
	FailReason string     `json:"fail_reason,omitempty"`
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, entityID *uuid.UUID, eventType string, data map[string]any) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       entityID,
		Type:           eventType,
		Data:           payload,
		Timestamp:      time.Now(),
	}
	if err := s.bus.Publish(ctx, ev); err != nil {
		log.Error().Err(err).Str("type", eventType).Msg("banking: failed to publish event")
	}
}
