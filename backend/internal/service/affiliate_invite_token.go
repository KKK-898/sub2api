package service

import (
	"context"
	"crypto/sha256"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
)

const (
	affiliateInviteTokenIssuer   = "sub2api"
	affiliateInviteTokenAudience = "gaoge-software-invite"
	affiliateInviteTokenTTL      = 365 * 24 * time.Hour
)

var ErrAffiliateInviteTokenInvalid = infraerrors.BadRequest(
	"AFFILIATE_INVITE_TOKEN_INVALID",
	"invalid or expired affiliate invitation link",
)

type AffiliateInviteTokenResult struct {
	Token     string
	ExpiresAt time.Time
}

type affiliateInviteTokenClaims struct {
	UserID  int64  `json:"user_id"`
	AffCode string `json:"aff_code"`
	jwt.RegisteredClaims
}

func affiliateInviteSigningKey(jwtSecret string) []byte {
	sum := sha256.Sum256([]byte("gaoge-affiliate-invite-v1\x00" + jwtSecret))
	return sum[:]
}

// IssueAffiliateInviteToken creates a tamper-proof invitation token bound to
// both the inviter account and its current affiliate code.
func (s *AuthService) IssueAffiliateInviteToken(ctx context.Context, userID int64) (*AffiliateInviteTokenResult, error) {
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.JWT.Secret) == "" {
		return nil, ErrServiceUnavailable
	}
	if userID <= 0 || s.affiliateService == nil {
		return nil, ErrAffiliateInviteTokenInvalid
	}
	if !s.affiliateService.IsEnabled(ctx) {
		return nil, ErrAffiliateDisabled
	}

	summary, err := s.affiliateService.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	expiresAt := now.Add(affiliateInviteTokenTTL)
	claims := affiliateInviteTokenClaims{
		UserID:  summary.UserID,
		AffCode: strings.ToUpper(strings.TrimSpace(summary.AffCode)),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    affiliateInviteTokenIssuer,
			Subject:   strconv.FormatInt(summary.UserID, 10),
			Audience:  jwt.ClaimStrings{affiliateInviteTokenAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(
		affiliateInviteSigningKey(s.cfg.JWT.Secret),
	)
	if err != nil {
		return nil, ErrServiceUnavailable
	}
	return &AffiliateInviteTokenResult{Token: tokenString, ExpiresAt: expiresAt}, nil
}

// ResolveAffiliateInviteToken verifies the signature and confirms that the
// inviter still owns the affiliate code embedded in the token.
func (s *AuthService) ResolveAffiliateInviteToken(ctx context.Context, rawToken string) (*AffiliateSummary, time.Time, error) {
	tokenString := strings.TrimSpace(rawToken)
	if s == nil || s.cfg == nil || strings.TrimSpace(s.cfg.JWT.Secret) == "" ||
		tokenString == "" || len(tokenString) > maxTokenLength || s.affiliateService == nil {
		return nil, time.Time{}, ErrAffiliateInviteTokenInvalid
	}

	var claims affiliateInviteTokenClaims
	parsed, err := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (any, error) {
			return affiliateInviteSigningKey(s.cfg.JWT.Secret), nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithIssuer(affiliateInviteTokenIssuer),
		jwt.WithAudience(affiliateInviteTokenAudience),
		jwt.WithIssuedAt(),
	)
	if err != nil || parsed == nil || !parsed.Valid || claims.UserID <= 0 ||
		claims.ExpiresAt == nil || claims.ExpiresAt.Time.IsZero() {
		return nil, time.Time{}, ErrAffiliateInviteTokenInvalid
	}

	expectedSubject := strconv.FormatInt(claims.UserID, 10)
	if claims.Subject != expectedSubject {
		return nil, time.Time{}, ErrAffiliateInviteTokenInvalid
	}
	summary, err := s.affiliateService.ResolveInviterByCode(ctx, claims.AffCode)
	if err != nil || summary == nil || summary.UserID != claims.UserID ||
		!strings.EqualFold(strings.TrimSpace(summary.AffCode), strings.TrimSpace(claims.AffCode)) {
		return nil, time.Time{}, ErrAffiliateInviteTokenInvalid
	}
	return summary, claims.ExpiresAt.Time.UTC(), nil
}
