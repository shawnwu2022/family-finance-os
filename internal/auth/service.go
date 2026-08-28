package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	LoginStepEnrollTOTP = "enroll_totp"
	LoginStepVerifyTOTP = "verify_totp"

	challengeTTL        = 5 * time.Minute
	sessionIdleTimeout  = 30 * time.Minute
	sessionAbsoluteTTL  = 12 * time.Hour
	sessionTouchCadence = 5 * time.Minute
	recoveryCodeCount   = 10
	recoveryCodeBytes   = 10
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidSecondFactor = errors.New("invalid second factor")
	ErrUnauthenticated     = errors.New("unauthenticated")
)

type LoginResult struct {
	ChallengeToken string
	Step           string
	TOTPSecret     string
	OTPAuthURI     string
}

type SessionIssue struct {
	SessionToken  string
	CSRFToken     string
	RecoveryCodes []string
}

type SessionIdentity struct {
	UserID      int64
	Username    string
	HouseholdID int64
	CSRFToken   string
}

type Service struct {
	store     *PostgresStore
	secretBox *SecretBox
	dummyHash string
}

func NewService(store *PostgresStore, secretBox *SecretBox) (*Service, error) {
	if store == nil || store.pool == nil {
		return nil, errors.New("auth store is required")
	}
	if secretBox == nil || secretBox.aead == nil {
		return nil, errors.New("auth secret box is required")
	}
	dummyHash, err := HashPassword("invalid authentication password")
	if err != nil {
		return nil, fmt.Errorf("create dummy password hash: %w", err)
	}
	return &Service{store: store, secretBox: secretBox, dummyHash: dummyHash}, nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func (s *Service) BeginLogin(ctx context.Context, username, password string, now time.Time) (LoginResult, error) {
	normalized := normalizeUsername(username)
	if normalized == "" {
		_, _ = VerifyPassword(s.dummyHash, password)
		return LoginResult{}, ErrInvalidCredentials
	}

	user, err := s.store.GetUserByNormalizedUsername(ctx, normalized)
	if errors.Is(err, ErrNotFound) {
		_, _ = VerifyPassword(s.dummyHash, password)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("load auth user: %w", err)
	}
	passwordOK, verifyErr := VerifyPassword(user.PasswordHash, password)
	if verifyErr != nil || !passwordOK || user.DisabledAt != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	challengeToken, err := NewOpaqueToken()
	if err != nil {
		return LoginResult{}, err
	}
	challengeHash := HashOpaqueToken(challengeToken)
	challenge := ChallengeRecord{
		TokenHash: challengeHash[:],
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(challengeTTL),
	}
	result := LoginResult{ChallengeToken: challengeToken}

	if user.TOTPEnrolledAt == nil || len(user.TOTPSecretCiphertext) == 0 {
		secret, err := GenerateTOTPSecret()
		if err != nil {
			return LoginResult{}, err
		}
		ciphertext, err := s.secretBox.Seal([]byte(secret))
		if err != nil {
			return LoginResult{}, err
		}
		challenge.Kind = ChallengeTOTPEnrollment
		challenge.PendingTOTPSecretCiphertext = ciphertext
		result.Step = LoginStepEnrollTOTP
		result.TOTPSecret = secret
		result.OTPAuthURI = buildOTPAuthURI(user.Username, secret)
	} else {
		challenge.Kind = ChallengeLogin
		result.Step = LoginStepVerifyTOTP
	}
	if err := s.store.CreateChallenge(ctx, challenge); err != nil {
		return LoginResult{}, fmt.Errorf("create login challenge: %w", err)
	}
	return result, nil
}

func (s *Service) ConfirmEnrollment(ctx context.Context, challengeToken, code string, now time.Time) (SessionIssue, error) {
	challengeHash := HashOpaqueToken(challengeToken)
	challenge, err := s.store.GetChallenge(ctx, challengeHash[:], now)
	if err != nil || challenge.Kind != ChallengeTOTPEnrollment || len(challenge.PendingTOTPSecretCiphertext) == 0 {
		return SessionIssue{}, ErrInvalidSecondFactor
	}
	plaintext, err := s.secretBox.Open(challenge.PendingTOTPSecretCiphertext)
	if err != nil {
		return SessionIssue{}, ErrInvalidSecondFactor
	}
	secret := string(plaintext)
	counter, err := VerifyTOTP(secret, strings.TrimSpace(code), now, -1)
	if err != nil {
		return SessionIssue{}, ErrInvalidSecondFactor
	}

	issue, session, err := s.newSession(challenge.UserID, now)
	if err != nil {
		return SessionIssue{}, err
	}
	recoveryCodes, recoveryHashes, err := generateRecoveryCodes()
	if err != nil {
		return SessionIssue{}, err
	}
	if err := s.store.CompleteEnrollment(ctx, CompleteEnrollmentParams{
		ChallengeTokenHash: challengeHash[:],
		Counter:            counter,
		Session:            session,
		RecoveryCodeHashes: recoveryHashes,
		Now:                now,
	}); err != nil {
		if errors.Is(err, ErrNotFound) {
			return SessionIssue{}, ErrInvalidSecondFactor
		}
		return SessionIssue{}, fmt.Errorf("complete TOTP enrollment: %w", err)
	}
	issue.RecoveryCodes = recoveryCodes
	return issue, nil
}

func (s *Service) VerifySecondFactor(ctx context.Context, challengeToken, code string, recovery bool, now time.Time) (SessionIssue, error) {
	challengeHash := HashOpaqueToken(challengeToken)
	challenge, err := s.store.GetChallenge(ctx, challengeHash[:], now)
	if err != nil || challenge.Kind != ChallengeLogin {
		return SessionIssue{}, ErrInvalidSecondFactor
	}
	user, err := s.store.GetUserByID(ctx, challenge.UserID)
	if err != nil || user.DisabledAt != nil || user.TOTPEnrolledAt == nil || len(user.TOTPSecretCiphertext) == 0 {
		return SessionIssue{}, ErrInvalidSecondFactor
	}
	issue, session, err := s.newSession(user.ID, now)
	if err != nil {
		return SessionIssue{}, err
	}

	if recovery {
		normalized, ok := normalizeRecoveryCode(code)
		if !ok {
			return SessionIssue{}, ErrInvalidSecondFactor
		}
		hash := sha256.Sum256([]byte(normalized))
		err = s.store.CompleteRecoveryLogin(ctx, CompleteRecoveryLoginParams{
			ChallengeTokenHash: challengeHash[:],
			RecoveryCodeHash:   hash[:],
			Session:            session,
			Now:                now,
		})
	} else {
		plaintext, openErr := s.secretBox.Open(user.TOTPSecretCiphertext)
		if openErr != nil {
			return SessionIssue{}, ErrInvalidSecondFactor
		}
		lastCounter := int64(-1)
		if user.TOTPLastCounter != nil {
			lastCounter = *user.TOTPLastCounter
		}
		counter, verifyErr := VerifyTOTP(string(plaintext), strings.TrimSpace(code), now, lastCounter)
		if verifyErr != nil {
			return SessionIssue{}, ErrInvalidSecondFactor
		}
		err = s.store.CompleteTOTPLogin(ctx, CompleteTOTPLoginParams{
			ChallengeTokenHash: challengeHash[:],
			Counter:            counter,
			Session:            session,
			Now:                now,
		})
	}
	if errors.Is(err, ErrNotFound) {
		return SessionIssue{}, ErrInvalidSecondFactor
	}
	if err != nil {
		return SessionIssue{}, fmt.Errorf("complete second-factor login: %w", err)
	}
	return issue, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, rawSessionToken string, now time.Time) (SessionIdentity, error) {
	if strings.TrimSpace(rawSessionToken) == "" {
		return SessionIdentity{}, ErrUnauthenticated
	}
	tokenHash := HashOpaqueToken(rawSessionToken)
	session, err := s.store.GetSession(ctx, tokenHash[:], now)
	if errors.Is(err, ErrNotFound) {
		return SessionIdentity{}, ErrUnauthenticated
	}
	if err != nil {
		return SessionIdentity{}, fmt.Errorf("load auth session: %w", err)
	}
	if !now.Before(session.LastSeenAt.Add(sessionIdleTimeout)) {
		_ = s.store.RevokeSession(ctx, tokenHash[:], now)
		return SessionIdentity{}, ErrUnauthenticated
	}
	csrf, err := s.secretBox.Open(session.CSRFTokenCiphertext)
	if err != nil {
		return SessionIdentity{}, ErrUnauthenticated
	}
	if !now.Before(session.LastSeenAt.Add(sessionTouchCadence)) {
		if err := s.store.TouchSession(ctx, tokenHash[:], now); err != nil {
			return SessionIdentity{}, ErrUnauthenticated
		}
	}
	return SessionIdentity{
		UserID:      session.UserID,
		Username:    session.Username,
		HouseholdID: session.HouseholdID,
		CSRFToken:   string(csrf),
	}, nil
}

func (s *Service) Logout(ctx context.Context, rawSessionToken string, now time.Time) error {
	if strings.TrimSpace(rawSessionToken) == "" {
		return nil
	}
	tokenHash := HashOpaqueToken(rawSessionToken)
	if err := s.store.RevokeSession(ctx, tokenHash[:], now); err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("revoke auth session: %w", err)
	}
	return nil
}

func (s *Service) newSession(userID int64, now time.Time) (SessionIssue, SessionRecord, error) {
	sessionToken, err := NewOpaqueToken()
	if err != nil {
		return SessionIssue{}, SessionRecord{}, err
	}
	csrfToken, err := NewOpaqueToken()
	if err != nil {
		return SessionIssue{}, SessionRecord{}, err
	}
	csrfCiphertext, err := s.secretBox.Seal([]byte(csrfToken))
	if err != nil {
		return SessionIssue{}, SessionRecord{}, err
	}
	sessionHash := HashOpaqueToken(sessionToken)
	csrfHash := HashOpaqueToken(csrfToken)
	return SessionIssue{
			SessionToken: sessionToken,
			CSRFToken:    csrfToken,
		}, SessionRecord{
			TokenHash:           sessionHash[:],
			UserID:              userID,
			CSRFTokenHash:       csrfHash[:],
			CSRFTokenCiphertext: csrfCiphertext,
			CreatedAt:           now,
			LastSeenAt:          now,
			ExpiresAt:           now.Add(sessionAbsoluteTTL),
		}, nil
}

func buildOTPAuthURI(username, secret string) string {
	issuer := "Family Finance OS"
	label := issuer + ":" + username
	query := url.Values{}
	query.Set("secret", secret)
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", "30")
	return (&url.URL{Scheme: "otpauth", Host: "totp", Path: label, RawQuery: query.Encode()}).String()
}

func generateRecoveryCodes() ([]string, [][]byte, error) {
	codes := make([]string, 0, recoveryCodeCount)
	hashes := make([][]byte, 0, recoveryCodeCount)
	for len(codes) < recoveryCodeCount {
		raw := make([]byte, recoveryCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}
		normalized := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		code := normalized[0:4] + "-" + normalized[4:8] + "-" + normalized[8:12] + "-" + normalized[12:16]
		hash := sha256.Sum256([]byte(normalized))
		codes = append(codes, code)
		hashes = append(hashes, append([]byte(nil), hash[:]...))
	}
	return codes, hashes, nil
}

func normalizeRecoveryCode(code string) (string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	if len(normalized) != 16 {
		return "", false
	}
	for _, r := range normalized {
		if (r < 'A' || r > 'Z') && (r < '2' || r > '7') {
			return "", false
		}
	}
	return normalized, true
}
