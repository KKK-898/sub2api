//go:build unit

package service

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestResolveRebateRatePercent_PerUserOverride verifies that per-inviter
// AffRebateRatePercent overrides the global rate, that NULL falls back to the
// global rate, and that out-of-range exclusive rates are clamped silently.
//
// SettingService is left nil here so globalRebateRatePercent returns the
// documented default (AffiliateRebateRateDefault = 20%) — this exercises the
// fallback path without spinning up a settings stub.
func TestResolveRebateRatePercent_PerUserOverride(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}

	// nil exclusive rate → falls back to global default (20%)
	require.InDelta(t, AffiliateRebateTier0To5Default,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}, 0), 1e-9)

	// exclusive rate set → overrides global
	rate := 50.0
	require.InDelta(t, 50.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &rate}, 0), 1e-9)

	// exclusive rate 0 → returns 0 (no rebate, intentional)
	zero := 0.0
	require.InDelta(t, 0.0,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &zero}, 0), 1e-9)

	// exclusive rate above max → clamped to Max
	tooHigh := 250.0
	require.InDelta(t, AffiliateRebateRateMax,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}, 0), 1e-9)

	// exclusive rate below min → clamped to Min
	tooLow := -5.0
	require.InDelta(t, AffiliateRebateRateMin,
		svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooLow}, 0), 1e-9)
}

func TestResolveRebateRatePercent_TierBoundaries(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	tests := []struct {
		active int
		want   float64
	}{
		{active: 0, want: AffiliateRebateTier0To5Default},
		{active: 5, want: AffiliateRebateTier0To5Default},
		{active: 6, want: AffiliateRebateTier6To10Default},
		{active: 10, want: AffiliateRebateTier6To10Default},
		{active: 11, want: AffiliateRebateTier11To20Default},
		{active: 20, want: AffiliateRebateTier11To20Default},
		{active: 21, want: AffiliateRebateTier21PlusDefault},
		{active: 100, want: AffiliateRebateTier21PlusDefault},
	}
	for _, tt := range tests {
		require.InDelta(t, tt.want, svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{}, tt.active), 1e-9)
	}
}

func TestAffiliateWithdrawalCashRate(t *testing.T) {
	t.Parallel()
	require.InDelta(t, 0.20, AffiliateWithdrawalCashRate, 1e-9)
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled
// safely handles a nil settingService dependency by returning the default
// (off). This protects callers from nil-pointer crashes in misconfigured
// environments.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestAffiliateInviteeStatusAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-14*24*time.Hour - 23*time.Hour)
	exactlyFifteenDays := now.Add(-15 * 24 * time.Hour)
	olderThanFifteenDays := exactlyFifteenDays.Add(-time.Second)
	recentBoundAt := recent
	oldBoundAt := olderThanFifteenDays

	tests := []struct {
		name     string
		invitee  AffiliateInvitee
		expected string
	}{
		{
			name: "paid threshold with recent use is active",
			invitee: AffiliateInvitee{
				LastUsedAt: &recent,
				TotalPaid:  AffiliateInviteePaidThresholdCNY,
			},
			expected: AffiliateInviteeStatusActive,
		},
		{
			name: "redeemed threshold with recent use is active",
			invitee: AffiliateInvitee{
				LastUsedAt:         &recent,
				TotalRedeemedQuota: AffiliateInviteeRedeemedThresholdUSD,
			},
			expected: AffiliateInviteeStatusActive,
		},
		{
			name: "below both thresholds with recent use is pending",
			invitee: AffiliateInvitee{
				LastUsedAt:         &recent,
				TotalPaid:          AffiliateInviteePaidThresholdCNY - 0.01,
				TotalRedeemedQuota: AffiliateInviteeRedeemedThresholdUSD - 0.01,
			},
			expected: AffiliateInviteeStatusPending,
		},
		{
			name: "exactly fifteen days is not yet invalid",
			invitee: AffiliateInvitee{
				LastUsedAt: &exactlyFifteenDays,
				TotalPaid:  AffiliateInviteePaidThresholdCNY,
			},
			expected: AffiliateInviteeStatusActive,
		},
		{
			name: "qualified invitee becomes invalid after fifteen days",
			invitee: AffiliateInvitee{
				LastUsedAt:         &olderThanFifteenDays,
				TotalPaid:          AffiliateInviteePaidThresholdCNY * 10,
				TotalRedeemedQuota: AffiliateInviteeRedeemedThresholdUSD * 10,
			},
			expected: AffiliateInviteeStatusInvalid,
		},
		{
			name: "recent use overrides an old binding date",
			invitee: AffiliateInvitee{
				BoundAt:    &oldBoundAt,
				LastUsedAt: &recent,
				TotalPaid:  AffiliateInviteePaidThresholdCNY,
			},
			expected: AffiliateInviteeStatusActive,
		},
		{
			name: "never used invitee falls back to recent binding date",
			invitee: AffiliateInvitee{
				BoundAt:   &recentBoundAt,
				TotalPaid: AffiliateInviteePaidThresholdCNY,
			},
			expected: AffiliateInviteeStatusActive,
		},
		{
			name: "never used invitee expires from binding date",
			invitee: AffiliateInvitee{
				BoundAt:   &oldBoundAt,
				TotalPaid: AffiliateInviteePaidThresholdCNY,
			},
			expected: AffiliateInviteeStatusInvalid,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, test.expected, affiliateInviteeStatusAt(test.invitee, now))
		})
	}
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
