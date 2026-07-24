//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestAffiliateRepositoryListInviteesReturnsRealtimeStatusesAndFullCounts(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)

	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	now := time.Date(2026, time.July, 24, 12, 0, 0, 0, time.UTC)
	boundAt := now.Add(-30 * 24 * time.Hour)
	recentUse := now.Add(-time.Hour)
	oldUse := now.Add(-16 * 24 * time.Hour)
	latestContribution := now.Add(-2 * time.Hour)

	rows := sqlmock.NewRows([]string{
		"user_id",
		"email",
		"username",
		"bound_at",
		"total_rebate",
		"latest_contribution_at",
		"total_paid",
		"total_redeemed_quota",
		"last_used_at",
		"status",
		"total_count",
		"active_count",
		"invalid_count",
		"pending_count",
	}).
		AddRow(
			int64(101),
			"active@example.com",
			"active",
			boundAt,
			6.0,
			latestContribution,
			service.AffiliateInviteePaidThresholdCNY,
			0.0,
			recentUse,
			service.AffiliateInviteeStatusActive,
			3,
			1,
			1,
			1,
		).
		AddRow(
			int64(102),
			"invalid@example.com",
			"invalid",
			boundAt,
			20.0,
			nil,
			service.AffiliateInviteePaidThresholdCNY*10,
			service.AffiliateInviteeRedeemedThresholdUSD*10,
			oldUse,
			service.AffiliateInviteeStatusInvalid,
			3,
			1,
			1,
			1,
		)

	mock.ExpectQuery(`(?s)WITH invitee_metrics AS .*COALESCE\(last_used_at, bound_at\) < \$3.*total_paid >= \$4 OR total_redeemed_quota >= \$5`).
		WithArgs(
			int64(7),
			2,
			now.Add(-service.AffiliateInviteeInactiveDays*24*time.Hour),
			service.AffiliateInviteePaidThresholdCNY,
			service.AffiliateInviteeRedeemedThresholdUSD,
		).
		WillReturnRows(rows)

	repo := NewAffiliateRepository(client, db)
	invitees, counts, err := repo.ListInvitees(context.Background(), 7, 2, now)
	require.NoError(t, err)
	require.Len(t, invitees, 2)
	require.Equal(t, service.AffiliateInviteeCounts{
		Total:   3,
		Active:  1,
		Invalid: 1,
		Pending: 1,
	}, counts)

	require.Equal(t, service.AffiliateInviteeStatusActive, invitees[0].Status)
	require.Equal(t, recentUse, *invitees[0].LastUsedAt)
	require.Equal(t, boundAt, *invitees[0].BoundAt)
	require.Equal(t, latestContribution, *invitees[0].LatestContributionAt)
	require.Equal(t, service.AffiliateInviteeStatusInvalid, invitees[1].Status)
	require.Equal(t, service.AffiliateInviteePaidThresholdCNY*10, invitees[1].TotalPaid)

	require.NoError(t, mock.ExpectationsWereMet())
}
