package notification

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	emails []EmailMessage
	sms    []SMSMessage
	pushes []PushMessage
}

func (r *recorder) EmailSend(_ context.Context, msg EmailMessage) error {
	r.emails = append(r.emails, msg)
	return nil
}

func (r *recorder) SMSSend(_ context.Context, msg SMSMessage) error {
	r.sms = append(r.sms, msg)
	return nil
}

func (r *recorder) PushSend(_ context.Context, msg PushMessage) error {
	r.pushes = append(r.pushes, msg)
	return nil
}

func (r *recorder) PushSendBulk(_ context.Context, msgs []PushMessage) error {
	r.pushes = append(r.pushes, msgs...)
	return nil
}

type recorderEmail struct{ r *recorder }

func (e *recorderEmail) Send(ctx context.Context, msg EmailMessage) error { return e.r.EmailSend(ctx, msg) }

type recorderSMS struct{ r *recorder }

func (s *recorderSMS) Send(ctx context.Context, msg SMSMessage) error { return s.r.SMSSend(ctx, msg) }

type recorderPush struct{ r *recorder }

func (p *recorderPush) Send(ctx context.Context, msg PushMessage) error { return p.r.PushSend(ctx, msg) }
func (p *recorderPush) SendBulk(ctx context.Context, msgs []PushMessage) error {
	return p.r.PushSendBulk(ctx, msgs)
}

func newTestService() (*Service, *recorder) {
	r := &recorder{}
	svc := NewService(&recorderEmail{r}, &recorderSMS{r}, &recorderPush{r})
	return svc, r
}

func TestSendOrderConfirmation(t *testing.T) {
	svc, r := newTestService()

	err := svc.SendOrderConfirmation(context.Background(), "user@test.com", "ORD-001", 15099)
	require.NoError(t, err)

	require.Len(t, r.emails, 1)
	assert.Equal(t, []string{"user@test.com"}, r.emails[0].To)
	assert.Contains(t, r.emails[0].Subject, "ORD-001")
	assert.Contains(t, r.emails[0].Body, "150.99")
}

func TestSendOrderConfirmation_NoEmail(t *testing.T) {
	svc, r := newTestService()

	err := svc.SendOrderConfirmation(context.Background(), "", "ORD-001", 15099)
	require.NoError(t, err)
	assert.Len(t, r.emails, 0)
}

func TestSendOrderShipped_EmailAndSMS(t *testing.T) {
	svc, r := newTestService()

	err := svc.SendOrderShipped(context.Background(), "user@test.com", "+71234567890", "ORD-002", "TRACK123")
	require.NoError(t, err)

	assert.Len(t, r.emails, 1)
	assert.Contains(t, r.emails[0].Body, "TRACK123")

	assert.Len(t, r.sms, 1)
	assert.Equal(t, "+71234567890", r.sms[0].Phone)
	assert.Contains(t, r.sms[0].Text, "TRACK123")
}

func TestNotifyLowStock(t *testing.T) {
	svc, r := newTestService()
	uid := uuid.New()

	err := svc.NotifyLowStock(context.Background(), uid, "Widget", 3.5)
	require.NoError(t, err)

	assert.Len(t, r.pushes, 1)
	assert.Equal(t, uid, r.pushes[0].UserID)
	assert.Equal(t, "Low stock alert", r.pushes[0].Title)
	assert.Contains(t, r.pushes[0].Body, "Widget")
}

func TestFormatCents(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0.00"},
		{100, "1.00"},
		{15099, "150.99"},
		{1, "0.01"},
		{99, "0.99"},
		{10000, "100.00"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expected, formatCents(tt.input), "formatCents(%d)", tt.input)
	}
}
