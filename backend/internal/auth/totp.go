package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

const (
	totpPeriodSeconds = 30
	totpSkew          = 1
	totpMaxFailures   = 5
	totpSetupTTL      = 10 * time.Minute
	// totpValidityWindow is the wall-clock span over which a single code stays
	// valid: the current 30s step plus totpSkew steps on each side. The replay
	// marker must outlive this window, otherwise a used code can be replayed
	// after the marker expires but while it still validates.
	totpValidityWindow = totpPeriodSeconds * (2*totpSkew + 1) * time.Second
)

// TOTPStatus receives an actor and returns whether TOTP is enabled for the user.
func (s *Service) TOTPStatus(ctx context.Context, actor Actor) (TOTPStatus, error) {
	if actor.UserID == "" {
		return TOTPStatus{}, errors.Wrap(ErrInvalidCredentials, "actor user id is required")
	}
	if !s.cfg.TOTPEnabled {
		return TOTPStatus{Enabled: false}, nil
	}

	record, err := s.store.UserByID(ctx, actor.UserID)
	if err != nil {
		return TOTPStatus{}, errors.Wrap(err, "load totp user")
	}

	return TOTPStatus{Enabled: record.TOTPEnabled}, nil
}

// SetupTOTP receives actor and session data, creates a pending setup secret, and returns an otpauth URI.
func (s *Service) SetupTOTP(ctx context.Context, request TOTPSetupRequest) (TOTPSetup, error) {
	if !s.cfg.TOTPEnabled {
		return TOTPSetup{}, errors.WithStack(errors.New("totp is disabled"))
	}
	if request.Actor.UserID == "" || request.Session.ID == "" {
		return TOTPSetup{}, errors.Wrap(ErrInvalidCredentials, "authenticated session is required")
	}

	record, err := s.store.UserByID(ctx, request.Actor.UserID)
	if err != nil {
		return TOTPSetup{}, errors.Wrap(err, "load totp setup user")
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      s.cfg.TOTPIssuer,
		AccountName: record.Email,
		Period:      totpPeriodSeconds,
		SecretSize:  20,
		Digits:      otp.DigitsSix,
		Algorithm:   otp.AlgorithmSHA1,
	})
	if err != nil {
		return TOTPSetup{}, errors.Wrap(err, "generate totp secret")
	}

	now := s.clock().UTC()
	setup := PendingTOTPSetup{
		UserID:    request.Actor.UserID,
		Secret:    key.Secret(),
		Otpauth:   key.URL(),
		CreatedAt: now,
		ExpiresAt: now.Add(totpSetupTTL).UTC(),
	}
	if err := s.store.StorePendingTOTPSetup(ctx, request.Session.ID, setup); err != nil {
		return TOTPSetup{}, errors.Wrap(err, "store pending totp setup")
	}

	return TOTPSetup{
		Secret:    setup.Secret,
		Otpauth:   setup.Otpauth,
		ExpiresAt: setup.ExpiresAt,
	}, nil
}

// ConfirmTOTP receives a pending setup code and enables TOTP for the authenticated user.
func (s *Service) ConfirmTOTP(ctx context.Context, request TOTPConfirmRequest) (TOTPStatus, error) {
	if !s.cfg.TOTPEnabled {
		return TOTPStatus{}, errors.WithStack(errors.New("totp is disabled"))
	}
	if request.Actor.UserID == "" || request.Session.ID == "" {
		return TOTPStatus{}, errors.Wrap(ErrInvalidCredentials, "authenticated session is required")
	}

	setup, err := s.store.PendingTOTPSetup(ctx, request.Session.ID)
	if err != nil {
		return TOTPStatus{}, errors.WithStack(ErrInvalidCredentials)
	}
	if setup.UserID != request.Actor.UserID || !setup.ExpiresAt.After(s.clock().UTC()) {
		if deleteErr := s.store.DeletePendingTOTPSetup(ctx, request.Session.ID); deleteErr != nil {
			return TOTPStatus{}, errors.Wrap(deleteErr, "delete invalid pending totp setup")
		}
		return TOTPStatus{}, errors.WithStack(ErrInvalidCredentials)
	}
	failures, err := s.store.FailedTOTPCount(ctx, request.Actor.UserID)
	if err != nil {
		return TOTPStatus{}, errors.Wrap(err, "load totp setup failures")
	}
	if failures >= totpMaxFailures {
		return TOTPStatus{}, errors.WithStack(ErrInvalidCredentials)
	}
	if err := s.validateTOTPValue(setup.Secret, request.Code); err != nil {
		if _, failErr := s.store.IncrementFailedTOTP(ctx, request.Actor.UserID); failErr != nil {
			return TOTPStatus{}, errors.Wrap(failErr, "record failed totp setup")
		}
		return TOTPStatus{}, err
	}

	record, err := s.store.UserByID(ctx, request.Actor.UserID)
	if err != nil {
		return TOTPStatus{}, errors.Wrap(err, "load totp confirm user")
	}
	record.TOTPEnabled = true
	record.TOTPSecret = setup.Secret
	record.UpdatedAt = s.clock().UTC()
	if _, err := s.store.UpdateUser(ctx, record); err != nil {
		return TOTPStatus{}, errors.Wrap(err, "enable totp")
	}
	if err := s.store.DeletePendingTOTPSetup(ctx, request.Session.ID); err != nil {
		return TOTPStatus{}, errors.Wrap(err, "delete confirmed pending totp setup")
	}
	if err := s.store.ResetFailedTOTP(ctx, request.Actor.UserID); err != nil {
		return TOTPStatus{}, errors.Wrap(err, "reset totp failures")
	}

	return TOTPStatus{Enabled: true}, nil
}

// DisableTOTP receives an actor and valid code and disables TOTP for the user.
func (s *Service) DisableTOTP(ctx context.Context, request TOTPDisableRequest) (TOTPStatus, error) {
	if !s.cfg.TOTPEnabled {
		return TOTPStatus{}, errors.WithStack(errors.New("totp is disabled"))
	}
	if request.Actor.UserID == "" {
		return TOTPStatus{}, errors.Wrap(ErrInvalidCredentials, "actor user id is required")
	}

	record, err := s.store.UserByID(ctx, request.Actor.UserID)
	if err != nil {
		return TOTPStatus{}, errors.Wrap(err, "load totp disable user")
	}
	if !record.TOTPEnabled {
		return TOTPStatus{Enabled: false}, nil
	}
	if err := s.verifyTOTPCode(ctx, record, request.Code); err != nil {
		return TOTPStatus{}, err
	}

	record.TOTPEnabled = false
	record.TOTPSecret = ""
	record.UpdatedAt = s.clock().UTC()
	if _, err := s.store.UpdateUser(ctx, record); err != nil {
		return TOTPStatus{}, errors.Wrap(err, "disable totp")
	}

	return TOTPStatus{Enabled: false}, nil
}

// verifyTOTPCode receives a user record and code and enforces validation, rate limits, and replay checks.
func (s *Service) verifyTOTPCode(ctx context.Context, record UserRecord, code string) error {
	failures, err := s.store.FailedTOTPCount(ctx, record.ID)
	if err != nil {
		return errors.Wrap(err, "load totp failures")
	}
	if failures >= totpMaxFailures {
		return errors.WithStack(ErrInvalidCredentials)
	}

	codeHash := hashTOTPCode(record.ID, code)
	replayed, err := s.store.TOTPReplayExists(ctx, record.ID, codeHash, s.clock().UTC())
	if err != nil {
		return errors.Wrap(err, "load totp replay")
	}
	if replayed {
		if _, failErr := s.store.IncrementFailedTOTP(ctx, record.ID); failErr != nil {
			return errors.Wrap(failErr, "record replayed totp")
		}
		return errors.WithStack(ErrInvalidCredentials)
	}
	if err := s.validateTOTPValue(record.TOTPSecret, code); err != nil {
		if _, failErr := s.store.IncrementFailedTOTP(ctx, record.ID); failErr != nil {
			return errors.Wrap(failErr, "record failed totp")
		}
		return err
	}

	// Keep the replay marker at least as long as the code stays valid so a
	// captured code cannot be replayed once the configured cache duration is
	// shorter than the skew validity window.
	replayRetention := s.cfg.TOTPReplayCacheDuration
	if replayRetention < totpValidityWindow {
		replayRetention = totpValidityWindow
	}
	expiresAt := s.clock().UTC().Add(replayRetention).UTC()
	if err := s.store.StoreTOTPReplay(ctx, record.ID, codeHash, expiresAt); err != nil {
		return errors.Wrap(err, "store totp replay")
	}
	if err := s.store.ResetFailedTOTP(ctx, record.ID); err != nil {
		return errors.Wrap(err, "reset totp failures")
	}

	return nil
}

// validateTOTPValue receives a secret and code and returns an error when the TOTP value is invalid.
func (s *Service) validateTOTPValue(secret string, code string) error {
	ok, err := totp.ValidateCustom(strings.TrimSpace(code), secret, s.clock().UTC(), totp.ValidateOpts{
		Period:    totpPeriodSeconds,
		Skew:      totpSkew,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		return errors.Wrap(err, "validate totp")
	}
	if !ok {
		return errors.WithStack(ErrInvalidCredentials)
	}

	return nil
}

// hashTOTPCode receives a user id and code and returns a stable replay cache hash.
func hashTOTPCode(userID string, code string) string {
	sum := sha256.Sum256([]byte(userID + "\x00" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}
