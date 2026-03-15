package banking

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog/log"
)

// SberConfig holds Sber Business API configuration.
type SberConfig struct {
	BaseURL      string `json:"base_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	CertPath     string `json:"cert_path"`
}

// SberAdapter implements BankAdapter for Sber Business API.
type SberAdapter struct {
	cfg    SberConfig
	client *http.Client
}

// NewSberAdapter creates a new Sber Business API adapter.
func NewSberAdapter(cfg SberConfig) *SberAdapter {
	return &SberAdapter{
		cfg: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// InitiatePayment creates a payment via Sber API.
func (a *SberAdapter) InitiatePayment(ctx context.Context, req PaymentRequest) (*PaymentResult, error) {
	body := map[string]any{
		"amount":            req.Amount,
		"currency":          req.Currency,
		"purpose":           req.Purpose,
		"recipient_inn":     req.RecipientINN,
		"recipient_bic":     req.RecipientBIC,
		"recipient_account": req.RecipientAccount,
	}
	if req.OrderID != nil {
		body["external_id"] = req.OrderID.String()
	}

	resp, err := a.doRequest(ctx, http.MethodPost, "/fintech/api/v1/payments", body)
	if err != nil {
		return nil, fmt.Errorf("sber: initiate payment: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		PaymentID string `json:"paymentId"`
		Status    string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sber: decode payment response: %w", err)
	}

	return &PaymentResult{
		PaymentID: result.PaymentID,
		Status:    result.Status,
	}, nil
}

// GetPaymentStatus checks payment status via Sber API.
func (a *SberAdapter) GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentStatus, error) {
	resp, err := a.doRequest(ctx, http.MethodGet, "/fintech/api/v1/payments/"+paymentID, nil)
	if err != nil {
		return nil, fmt.Errorf("sber: get payment status: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		PaymentID string `json:"paymentId"`
		Status    string `json:"status"`
		Amount    int64  `json:"amount"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sber: decode status response: %w", err)
	}

	return &PaymentStatus{
		PaymentID: result.PaymentID,
		Status:    result.Status,
		Amount:    result.Amount,
		UpdatedAt: time.Now(),
	}, nil
}

// GetStatement fetches bank statement from Sber API.
func (a *SberAdapter) GetStatement(ctx context.Context, req StatementRequest) (*Statement, error) {
	path := fmt.Sprintf("/fintech/api/v1/statement?dateFrom=%s&dateTo=%s",
		req.DateFrom.Format("2006-01-02"),
		req.DateTo.Format("2006-01-02"))

	resp, err := a.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("sber: get statement: %w", err)
	}
	defer resp.Body.Close()

	var raw struct {
		OpenBalance  int64 `json:"openingBalance"`
		CloseBalance int64 `json:"closingBalance"`
		Operations   []struct {
			Date         string `json:"date"`
			Amount       int64  `json:"amount"`
			Direction    string `json:"direction"`
			Purpose      string `json:"purpose"`
			PayerINN     string `json:"payerInn"`
			PayerName    string `json:"payerName"`
			ReceiverINN  string `json:"receiverInn"`
			ReceiverName string `json:"receiverName"`
			DocNumber    string `json:"documentNumber"`
		} `json:"operations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("sber: decode statement: %w", err)
	}

	st := &Statement{
		DateFrom:     req.DateFrom,
		DateTo:       req.DateTo,
		OpenBalance:  raw.OpenBalance,
		CloseBalance: raw.CloseBalance,
	}

	for _, op := range raw.Operations {
		date, _ := time.Parse("2006-01-02", op.Date)
		inn := op.PayerINN
		name := op.PayerName
		if op.Direction == "debit" {
			inn = op.ReceiverINN
			name = op.ReceiverName
		}
		st.Entries = append(st.Entries, StatementEntry{
			Date:             date,
			Amount:           op.Amount,
			Direction:        op.Direction,
			Purpose:          op.Purpose,
			CounterpartyINN:  inn,
			CounterpartyName: name,
			DocumentNumber:   op.DocNumber,
		})
	}

	return st, nil
}

// CreatePayrollBatch sends a salary batch to Sber API.
func (a *SberAdapter) CreatePayrollBatch(ctx context.Context, batch PayrollBatch) (*PayrollBatchResult, error) {
	employees := make([]map[string]any, len(batch.Employees))
	for i, e := range batch.Employees {
		employees[i] = map[string]any{
			"full_name":   e.FullName,
			"card_number": e.CardNumber,
			"amount":      e.Amount,
		}
	}

	body := map[string]any{
		"month":     batch.Month,
		"year":      batch.Year,
		"employees": employees,
	}

	resp, err := a.doRequest(ctx, http.MethodPost, "/fintech/api/v1/salary", body)
	if err != nil {
		return nil, fmt.Errorf("sber: create payroll batch: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		BatchID string `json:"batchId"`
		Status  string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sber: decode payroll response: %w", err)
	}

	return &PayrollBatchResult{
		BatchID: result.BatchID,
		Status:  result.Status,
		Count:   len(batch.Employees),
	}, nil
}

// GetPayrollBatchStatus checks payroll batch status.
func (a *SberAdapter) GetPayrollBatchStatus(ctx context.Context, batchID string) (*PayrollBatchStatus, error) {
	resp, err := a.doRequest(ctx, http.MethodGet, "/fintech/api/v1/salary/"+batchID, nil)
	if err != nil {
		return nil, fmt.Errorf("sber: get payroll status: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		BatchID   string `json:"batchId"`
		Status    string `json:"status"`
		Processed int    `json:"processed"`
		Failed    int    `json:"failed"`
		Total     int    `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sber: decode payroll status: %w", err)
	}

	return &PayrollBatchStatus{
		BatchID:   result.BatchID,
		Status:    result.Status,
		Processed: result.Processed,
		Failed:    result.Failed,
		Total:     result.Total,
		UpdatedAt: time.Now(),
	}, nil
}

// GetAuthURL builds the Sber OAuth authorization URL.
func (a *SberAdapter) GetAuthURL(_ context.Context, redirectURL string) (string, error) {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", a.cfg.ClientID)
	params.Set("redirect_uri", redirectURL)
	params.Set("scope", "openid")
	return a.cfg.BaseURL + "/ic/sso/api/v2/oauth/authorize?" + params.Encode(), nil
}

// ExchangeCode exchanges an OAuth code for bank user info.
func (a *SberAdapter) ExchangeCode(ctx context.Context, code string) (*BankUser, error) {
	body := map[string]any{
		"grant_type":    "authorization_code",
		"code":          code,
		"client_id":     a.cfg.ClientID,
		"client_secret": a.cfg.ClientSecret,
	}

	resp, err := a.doRequest(ctx, http.MethodPost, "/ic/sso/api/v2/oauth/token", body)
	if err != nil {
		return nil, fmt.Errorf("sber: exchange code: %w", err)
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		UserInfo    struct {
			Sub      string `json:"sub"`
			Name     string `json:"name"`
			INN      string `json:"inn"`
			Phone    string `json:"phone_number"`
			Email    string `json:"email"`
		} `json:"user_info"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("sber: decode token response: %w", err)
	}

	return &BankUser{
		BankClientID: tokenResp.UserInfo.Sub,
		FullName:     tokenResp.UserInfo.Name,
		INN:          tokenResp.UserInfo.INN,
		Phone:        tokenResp.UserInfo.Phone,
		Email:        tokenResp.UserInfo.Email,
	}, nil
}

// GetBankInfo returns Sber metadata.
func (a *SberAdapter) GetBankInfo() BankInfo {
	return BankInfo{Code: "sber", Name: "Сбер"}
}

func (a *SberAdapter) doRequest(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.cfg.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		log.Error().
			Int("status", resp.StatusCode).
			Str("body", string(bodyBytes)).
			Str("path", path).
			Msg("sber API error")
		return nil, fmt.Errorf("sber API error: %d %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}
