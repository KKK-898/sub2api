//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type affiliateRegistrationUserRepoStub struct {
	*userRepoStub
	inviterID int64
	createErr error
}

func (r *affiliateRegistrationUserRepoStub) CreateWithAffiliateInviter(
	ctx context.Context,
	user *User,
	inviterID int64,
) error {
	r.inviterID = inviterID
	if r.createErr != nil {
		return r.createErr
	}
	return r.userRepoStub.Create(ctx, user)
}

func newAffiliateRegistrationAuthService(
	userRepo UserRepository,
	affiliateRepo AffiliateRepository,
) *AuthService {
	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "affiliate-registration-test-secret", ExpireHour: 1},
	}
	settingSvc := NewSettingService(&settingRepoStub{values: map[string]string{
		SettingKeyRegistrationEnabled: "true",
		SettingKeyAffiliateEnabled:    "true",
	}}, cfg)
	return NewAuthService(
		nil,
		userRepo,
		nil,
		nil,
		cfg,
		settingSvc,
		nil,
		nil,
		nil,
		nil,
		nil,
		NewAffiliateService(affiliateRepo, settingSvc, nil, nil),
		nil,
	)
}

func TestRegisterWithAffiliateUsesAtomicRepositoryCapability(t *testing.T) {
	userRepo := &affiliateRegistrationUserRepoStub{
		userRepoStub: &userRepoStub{nextID: 77},
	}
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		inviterSummary: &AffiliateSummary{UserID: 42, AffCode: "INVITE2026"},
	}
	authSvc := newAffiliateRegistrationAuthService(userRepo, affiliateRepo)

	_, user, err := authSvc.RegisterWithVerification(
		context.Background(),
		"invitee@example.com",
		"password",
		"",
		"",
		"",
		"INVITE2026",
	)
	require.NoError(t, err)
	require.Equal(t, int64(77), user.ID)
	require.Equal(t, int64(42), userRepo.inviterID)
}

func TestRegisterWithAffiliateDoesNotReportSuccessWhenAtomicCreateFails(t *testing.T) {
	userRepo := &affiliateRegistrationUserRepoStub{
		userRepoStub: &userRepoStub{nextID: 77},
		createErr:    errors.New("affiliate transaction failed"),
	}
	affiliateRepo := &paymentFulfillmentAffiliateRepoStub{
		inviterSummary: &AffiliateSummary{UserID: 42, AffCode: "INVITE2026"},
	}
	authSvc := newAffiliateRegistrationAuthService(userRepo, affiliateRepo)

	_, user, err := authSvc.RegisterWithVerification(
		context.Background(),
		"invitee@example.com",
		"password",
		"",
		"",
		"",
		"INVITE2026",
	)
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.Nil(t, user)
	require.Equal(t, int64(42), userRepo.inviterID)
}
