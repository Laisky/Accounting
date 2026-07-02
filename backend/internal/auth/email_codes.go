package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/Laisky/errors/v2"
)

// maxEmailCodeAttempts caps failed verification guesses per issued email code
// before the code is invalidated, bounding brute-force of the six-digit value.
const maxEmailCodeAttempts = 5

// RequestEmailVerification receives an email address and returns a one-time verification code delivery.
func (s *Service) RequestEmailVerification(ctx context.Context, request EmailCodeRequest) (EmailCodeDelivery, error) {
	email, err := normalizeEmail(request.Email)
	if err != nil {
		return EmailCodeDelivery{}, err
	}

	record, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		return EmailCodeDelivery{
			Email:     email,
			ExpiresAt: s.clock().UTC().Add(s.cfg.EmailVerificationTTL).UTC(),
		}, nil
	}
	if record.EmailVerified && record.Status == UserStatusActive {
		return EmailCodeDelivery{
			Email:     email,
			ExpiresAt: s.clock().UTC().Add(s.cfg.EmailVerificationTTL).UTC(),
		}, nil
	}

	delivery, err := s.createEmailCode(ctx, email, EmailCodePurposeVerification)
	if err != nil {
		return EmailCodeDelivery{}, err
	}
	user := record.User
	delivery.User = &user
	if err := s.email.SendAuthCode(ctx, delivery, EmailCodePurposeVerification); err != nil {
		return EmailCodeDelivery{}, errors.Wrap(err, "send email verification code")
	}

	return delivery, nil
}

// ConfirmEmailVerification receives a verification code and activates the matching user email.
func (s *Service) ConfirmEmailVerification(ctx context.Context, request ConfirmEmailRequest) (User, error) {
	email, err := normalizeEmail(request.Email)
	if err != nil {
		return User{}, errors.WithStack(ErrInvalidCredentials)
	}
	if err := s.consumeEmailCode(ctx, email, EmailCodePurposeVerification, request.Code); err != nil {
		return User{}, err
	}

	record, err := s.store.UserByEmail(ctx, email)
	if err != nil {
		return User{}, errors.WithStack(ErrInvalidCredentials)
	}

	now := s.clock().UTC()
	record.EmailVerified = true
	record.Status = UserStatusActive
	record.UpdatedAt = now
	updated, err := s.store.UpdateUser(ctx, record)
	if err != nil {
		return User{}, errors.Wrap(err, "update verified user")
	}

	return updated.User, nil
}

// RequestPasswordReset receives an email address and returns a one-time password reset code delivery.
func (s *Service) RequestPasswordReset(ctx context.Context, request EmailCodeRequest) (EmailCodeDelivery, error) {
	email, err := normalizeEmail(request.Email)
	if err != nil {
		return EmailCodeDelivery{}, err
	}

	record, err := s.store.UserByEmail(ctx, email)
	if err != nil || record.Status != UserStatusActive {
		return EmailCodeDelivery{
			Email:     email,
			ExpiresAt: s.clock().UTC().Add(s.cfg.EmailVerificationTTL).UTC(),
		}, nil
	}

	delivery, err := s.createEmailCode(ctx, email, EmailCodePurposePasswordReset)
	if err != nil {
		return EmailCodeDelivery{}, err
	}
	user := record.User
	delivery.User = &user
	if err := s.email.SendAuthCode(ctx, delivery, EmailCodePurposePasswordReset); err != nil {
		return EmailCodeDelivery{}, errors.Wrap(err, "send password reset code")
	}

	return delivery, nil
}

// ConfirmPasswordReset receives a reset code and new password and updates the matching user password.
func (s *Service) ConfirmPasswordReset(ctx context.Context, request ConfirmPasswordResetRequest) (User, error) {
	email, err := normalizeEmail(request.Email)
	if err != nil {
		return User{}, errors.WithStack(ErrInvalidCredentials)
	}
	passwordHash, err := HashPassword(request.NewPassword)
	if err != nil {
		return User{}, err
	}
	if err := s.consumeEmailCode(ctx, email, EmailCodePurposePasswordReset, request.Code); err != nil {
		return User{}, err
	}

	record, err := s.store.UserByEmail(ctx, email)
	if err != nil || record.Status != UserStatusActive {
		return User{}, errors.WithStack(ErrInvalidCredentials)
	}

	record.PasswordHash = passwordHash
	record.UpdatedAt = s.clock().UTC()
	updated, err := s.store.UpdateUser(ctx, record)
	if err != nil {
		return User{}, errors.Wrap(err, "update reset password")
	}
	if err := s.store.ResetFailedLogin(ctx, email); err != nil {
		return User{}, errors.Wrap(err, "reset failed login")
	}

	return updated.User, nil
}

// createEmailCode receives an email and purpose, stores a hashed one-time code, and returns delivery data.
func (s *Service) createEmailCode(ctx context.Context, email string, purpose EmailCodePurpose) (EmailCodeDelivery, error) {
	code, err := newEmailCode()
	if err != nil {
		return EmailCodeDelivery{}, err
	}

	now := s.clock().UTC()
	record := EmailCodeRecord{
		Email:     email,
		Purpose:   purpose,
		CodeHash:  hashEmailCode(email, purpose, code),
		CreatedAt: now,
		ExpiresAt: now.Add(s.cfg.EmailVerificationTTL).UTC(),
	}
	if err := s.store.StoreEmailCode(ctx, record); err != nil {
		return EmailCodeDelivery{}, errors.Wrap(err, "store email code")
	}

	return EmailCodeDelivery{
		Email:     email,
		Code:      code,
		ExpiresAt: record.ExpiresAt,
	}, nil
}

// consumeEmailCode receives email, purpose, and plaintext code and consumes the matching one-time code.
func (s *Service) consumeEmailCode(ctx context.Context, email string, purpose EmailCodePurpose, code string) error {
	if strings.TrimSpace(code) == "" {
		return errors.WithStack(ErrInvalidCredentials)
	}

	record, err := s.store.EmailCode(ctx, email, purpose)
	if err != nil {
		return errors.WithStack(ErrInvalidCredentials)
	}
	if !record.ExpiresAt.After(s.clock().UTC()) {
		if err := s.store.DeleteEmailCode(ctx, email, purpose); err != nil {
			return errors.Wrap(err, "delete expired email code")
		}
		return errors.WithStack(ErrInvalidCredentials)
	}

	actualHash := hashEmailCode(email, purpose, code)
	if subtle.ConstantTimeCompare([]byte(actualHash), []byte(record.CodeHash)) != 1 {
		// Count the wrong guess and invalidate the code once too many accumulate.
		// A six-digit code has only 10^6 possibilities, so without a per-code cap
		// it could be brute-forced within its TTL from rotating source IPs; the
		// HTTP rate limiter alone (keyed on client IP) is not sufficient.
		record.Attempts++
		if record.Attempts >= maxEmailCodeAttempts {
			if err := s.store.DeleteEmailCode(ctx, email, purpose); err != nil {
				return errors.Wrap(err, "delete exhausted email code")
			}
		} else if err := s.store.StoreEmailCode(ctx, record); err != nil {
			return errors.Wrap(err, "record failed email code attempt")
		}
		return errors.WithStack(ErrInvalidCredentials)
	}
	if err := s.store.DeleteEmailCode(ctx, email, purpose); err != nil {
		return errors.Wrap(err, "delete consumed email code")
	}

	return nil
}

// newEmailCode returns a cryptographically random six-digit email code.
func newEmailCode() (string, error) {
	value, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", errors.Wrap(err, "read email code randomness")
	}

	return fmt.Sprintf("%06d", value.Int64()), nil
}

// hashEmailCode receives email, purpose, and code and returns a stable comparison hash.
func hashEmailCode(email string, purpose EmailCodePurpose, code string) string {
	sum := sha256.Sum256([]byte(email + "\x00" + string(purpose) + "\x00" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}
