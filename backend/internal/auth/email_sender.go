package auth

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/Laisky/errors/v2"
)

// EmailSender sends one-time authentication codes through a delivery backend.
type EmailSender interface {
	SendAuthCode(ctx context.Context, delivery EmailCodeDelivery, purpose EmailCodePurpose) error
}

// NoopEmailSender intentionally drops auth email deliveries for local tests and development.
type NoopEmailSender struct{}

// SendAuthCode receives delivery data and returns nil without sending a message.
func (NoopEmailSender) SendAuthCode(context.Context, EmailCodeDelivery, EmailCodePurpose) error {
	return nil
}

// SMTPConfig contains SMTP settings for auth email delivery.
type SMTPConfig struct {
	Host           string
	Port           int
	Username       string
	Password       string
	From           string
	ForceTLSVerify bool
}

// SMTPEmailSender sends auth email codes through an SMTP server.
type SMTPEmailSender struct {
	cfg SMTPConfig
}

// NewSMTPEmailSender receives SMTP settings and returns a configured email sender.
func NewSMTPEmailSender(cfg SMTPConfig) (*SMTPEmailSender, error) {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.From = strings.TrimSpace(cfg.From)
	if cfg.Host == "" {
		return nil, errors.WithStack(errors.New("smtp host is required"))
	}
	if cfg.Port <= 0 {
		return nil, errors.WithStack(errors.New("smtp port is required"))
	}
	if cfg.From == "" {
		return nil, errors.WithStack(errors.New("smtp from address is required"))
	}

	return &SMTPEmailSender{cfg: cfg}, nil
}

// SendAuthCode receives delivery data and sends the one-time code by SMTP.
func (s *SMTPEmailSender) SendAuthCode(ctx context.Context, delivery EmailCodeDelivery, purpose EmailCodePurpose) error {
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "smtp auth email canceled")
	default:
	}
	if strings.TrimSpace(delivery.Code) == "" {
		return errors.WithStack(errors.New("auth email code is required"))
	}

	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	if strings.TrimSpace(s.cfg.Username) == "" {
		auth = nil
	}
	message := authEmailMessage(s.cfg.From, delivery.Email, purpose, delivery.Code)
	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{delivery.Email}, []byte(message)); err != nil {
		return errors.Wrap(err, "send auth email")
	}

	return nil
}

// tlsConfig receives SMTP config and returns the TLS settings used by future SMTP clients.
func tlsConfig(cfg SMTPConfig) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: !cfg.ForceTLSVerify, //nolint:gosec // Explicit local-development escape hatch from runtime config.
		ServerName:         cfg.Host,
		MinVersion:         tls.VersionTLS12,
	}
}

// authEmailMessage receives message fields and returns a plain-text auth email.
func authEmailMessage(from string, to string, purpose EmailCodePurpose, code string) string {
	subject := "Accounting verification code"
	if purpose == EmailCodePurposePasswordReset {
		subject = "Accounting password reset code"
	}

	return strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		"Your Accounting code is " + strings.TrimSpace(code) + ".",
		"This code expires soon. If you did not request it, ignore this email.",
		"",
	}, "\r\n")
}
