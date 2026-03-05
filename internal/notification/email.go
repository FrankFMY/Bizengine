package notification

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/rs/zerolog/log"
)

type EmailMessage struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	HTML    bool     `json:"html"`
}

type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// SMTPConfig holds SMTP server connection parameters.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// SMTPSender sends emails via SMTP.
type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

func (s *SMTPSender) Send(_ context.Context, msg EmailMessage) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	contentType := "text/plain"
	if msg.HTML {
		contentType = "text/html"
	}

	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s; charset=\"utf-8\"\r\n\r\n",
		s.cfg.From, strings.Join(msg.To, ", "), msg.Subject, contentType)

	body := headers + msg.Body

	return smtp.SendMail(addr, auth, s.cfg.From, msg.To, []byte(body))
}

// EmailStub logs emails without sending them.
type EmailStub struct{}

func NewEmailStub() *EmailStub { return &EmailStub{} }

func (s *EmailStub) Send(_ context.Context, msg EmailMessage) error {
	log.Debug().Strs("to", msg.To).Str("subject", msg.Subject).Msg("email stub: Send")
	return nil
}
