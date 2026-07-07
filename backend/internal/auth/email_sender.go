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
	message := authEmailMessage(s.cfg.From, delivery.Email, purpose, delivery.Code)
	if err := s.sendMail(ctx, addr, delivery.Email, []byte(message)); err != nil {
		return errors.Wrap(err, "send auth email")
	}

	return nil
}

// sendMail receives SMTP address, recipient, and message data, then sends it through STARTTLS.
func (s *SMTPEmailSender) sendMail(ctx context.Context, addr string, recipient string, message []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return errors.Wrap(err, "dial smtp")
	}
	defer func() {
		_ = client.Close()
	}()
	if err := checkContext(ctx); err != nil {
		return err
	}
	ok, _ := client.Extension("STARTTLS")
	if !ok {
		return errors.WithStack(errors.New("smtp starttls is unavailable"))
	}
	if err := client.StartTLS(tlsConfig(s.cfg)); err != nil {
		return errors.Wrap(err, "start smtp tls")
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(s.cfg.Username) != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return errors.Wrap(err, "authenticate smtp")
		}
	}
	if err := client.Mail(s.cfg.From); err != nil {
		return errors.Wrap(err, "set smtp sender")
	}
	if err := client.Rcpt(recipient); err != nil {
		return errors.Wrap(err, "set smtp recipient")
	}
	writer, err := client.Data()
	if err != nil {
		return errors.Wrap(err, "open smtp data")
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return errors.Wrap(err, "write smtp data")
	}
	if err := writer.Close(); err != nil {
		return errors.Wrap(err, "close smtp data")
	}
	if err := client.Quit(); err != nil {
		return errors.Wrap(err, "quit smtp")
	}

	return nil
}

// checkContext receives a context and returns its cancellation error when canceled.
func checkContext(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "smtp auth email canceled")
	default:
		return nil
	}
}

// tlsConfig receives SMTP config and returns the TLS settings used by SMTP clients.
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
