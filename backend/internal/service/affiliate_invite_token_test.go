//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAffiliateInviteTokenIssueAndResolve(t *testing.T) {
	repo := &paymentFulfillmentAffiliateRepoStub{
		inviterSummary: &AffiliateSummary{
			UserID:  42,
			AffCode: "INVITE2026",
		},
	}
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{
		values: map[string]string{SettingKeyAffiliateEnabled: "true"},
	}, &config.Config{})
	affiliateSvc := NewAffiliateService(repo, settingSvc, nil, nil)
	authSvc := &AuthService{
		cfg:              &config.Config{JWT: config.JWTConfig{Secret: "test-affiliate-invite-secret"}},
		affiliateService: affiliateSvc,
	}

	result, err := authSvc.IssueAffiliateInviteToken(context.Background(), 42)
	require.NoError(t, err)
	require.NotEmpty(t, result.Token)
	require.False(t, result.ExpiresAt.IsZero())

	summary, expiresAt, err := authSvc.ResolveAffiliateInviteToken(context.Background(), result.Token)
	require.NoError(t, err)
	require.Equal(t, int64(42), summary.UserID)
	require.Equal(t, "INVITE2026", summary.AffCode)
	require.Equal(t, result.ExpiresAt.Unix(), expiresAt.Unix())

	_, err = authSvc.ValidateToken(result.Token)
	require.ErrorIs(t, err, ErrInvalidToken, "an invitation token must never authenticate API requests")
}

func TestAffiliateInviteTokenRejectsTamperingAndCodeReset(t *testing.T) {
	repo := &paymentFulfillmentAffiliateRepoStub{
		inviterSummary: &AffiliateSummary{
			UserID:  42,
			AffCode: "INVITE2026",
		},
	}
	settingSvc := NewSettingService(&paymentFulfillmentSettingRepoStub{
		values: map[string]string{SettingKeyAffiliateEnabled: "true"},
	}, &config.Config{})
	authSvc := &AuthService{
		cfg: &config.Config{JWT: config.JWTConfig{Secret: "test-affiliate-invite-secret"}},
		affiliateService: NewAffiliateService(
			repo,
			settingSvc,
			nil,
			nil,
		),
	}

	result, err := authSvc.IssueAffiliateInviteToken(context.Background(), 42)
	require.NoError(t, err)

	tampered := result.Token[:len(result.Token)-1] + "x"
	_, _, err = authSvc.ResolveAffiliateInviteToken(context.Background(), tampered)
	require.ErrorIs(t, err, ErrAffiliateInviteTokenInvalid)

	repo.inviterSummary.AffCode = "RESET2026"
	_, _, err = authSvc.ResolveAffiliateInviteToken(context.Background(), result.Token)
	require.ErrorIs(t, err, ErrAffiliateInviteTokenInvalid)
}
