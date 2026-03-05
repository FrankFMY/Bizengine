package notification

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Service struct {
	repo  Repository
	email EmailSender
	sms   SMSSender
	push  PushSender
}

func NewService(email EmailSender, sms SMSSender, push PushSender) *Service {
	return &Service{email: email, sms: sms, push: push}
}

func (s *Service) SetRepo(repo Repository) {
	s.repo = repo
}

func (s *Service) CreateNotification(ctx context.Context, n *Notification) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.Create(ctx, n)
}

func (s *Service) ListNotifications(ctx context.Context, orgID, userID uuid.UUID, filter ListFilter) ([]Notification, int, error) {
	if s.repo == nil {
		return nil, 0, nil
	}
	return s.repo.List(ctx, orgID, userID, filter)
}

func (s *Service) MarkRead(ctx context.Context, orgID, userID, notifID uuid.UUID) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.MarkRead(ctx, orgID, userID, notifID)
}

func (s *Service) MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error {
	if s.repo == nil {
		return nil
	}
	return s.repo.MarkAllRead(ctx, orgID, userID)
}

func (s *Service) CountUnread(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	if s.repo == nil {
		return 0, nil
	}
	return s.repo.CountUnread(ctx, orgID, userID)
}

func (s *Service) SendOrderConfirmation(ctx context.Context, email string, orderNumber string, total int64) error {
	if email == "" {
		return nil
	}
	return s.email.Send(ctx, EmailMessage{
		To:      []string{email},
		Subject: "Order " + orderNumber + " confirmed",
		Body:    "Your order " + orderNumber + " has been confirmed. Total: " + formatCents(total) + ".",
	})
}

func (s *Service) SendOrderShipped(ctx context.Context, email, phone, orderNumber, trackingNumber string) error {
	if email != "" {
		if err := s.email.Send(ctx, EmailMessage{
			To:      []string{email},
			Subject: "Order " + orderNumber + " shipped",
			Body:    "Your order " + orderNumber + " has been shipped. Tracking: " + trackingNumber,
		}); err != nil {
			log.Error().Err(err).Str("email", email).Msg("notification: failed to send shipment email")
		}
	}
	if phone != "" {
		if err := s.sms.Send(ctx, SMSMessage{
			Phone: phone,
			Text:  "Order " + orderNumber + " shipped. Track: " + trackingNumber,
		}); err != nil {
			log.Error().Err(err).Str("phone", phone).Msg("notification: failed to send shipment SMS")
		}
	}
	return nil
}

func (s *Service) SendPushToUser(ctx context.Context, userID uuid.UUID, title, body string) error {
	return s.push.Send(ctx, PushMessage{
		UserID: userID,
		Title:  title,
		Body:   body,
	})
}

func (s *Service) NotifyLowStock(ctx context.Context, userID uuid.UUID, productName string, available float64) error {
	return s.push.Send(ctx, PushMessage{
		UserID: userID,
		Title:  "Low stock alert",
		Body:   productName + " is running low (" + formatFloat(available) + " remaining)",
		Data:   map[string]string{"type": "low_stock"},
	})
}

func formatCents(cents int64) string {
	rubles := cents / 100
	kop := cents % 100
	if kop < 0 {
		kop = -kop
	}
	s := ""
	if rubles == 0 {
		s = "0"
	} else {
		n := rubles
		if n < 0 {
			s = "-"
			n = -n
		}
		digits := ""
		for n > 0 {
			digits = string(rune('0'+n%10)) + digits
			n /= 10
		}
		s += digits
	}
	k := ""
	if kop < 10 {
		k = "0" + string(rune('0'+kop))
	} else {
		k = string(rune('0'+kop/10)) + string(rune('0'+kop%10))
	}
	return s + "." + k
}

func formatFloat(f float64) string {
	// Simple formatter for notification text
	i := int64(f * 100)
	return formatCents(i)
}
