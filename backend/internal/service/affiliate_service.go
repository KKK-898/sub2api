package service

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrAffiliateProfileNotFound      = infraerrors.NotFound("AFFILIATE_PROFILE_NOT_FOUND", "affiliate profile not found")
	ErrAffiliateCodeInvalid          = infraerrors.BadRequest("AFFILIATE_CODE_INVALID", "invalid affiliate code")
	ErrAffiliateCodeTaken            = infraerrors.Conflict("AFFILIATE_CODE_TAKEN", "affiliate code already in use")
	ErrAffiliateAlreadyBound         = infraerrors.Conflict("AFFILIATE_ALREADY_BOUND", "affiliate inviter already bound")
	ErrAffiliateDisabled             = infraerrors.BadRequest("AFFILIATE_DISABLED", "affiliate program is disabled")
	ErrAffiliateQuotaEmpty           = infraerrors.BadRequest("AFFILIATE_QUOTA_EMPTY", "no affiliate quota available to transfer")
	ErrAffiliateWithdrawalPending    = infraerrors.Conflict("AFFILIATE_WITHDRAWAL_PENDING", "an affiliate withdrawal is already pending")
	ErrAffiliateWithdrawalNotFound   = infraerrors.NotFound("AFFILIATE_WITHDRAWAL_NOT_FOUND", "affiliate withdrawal not found")
	ErrAffiliateWithdrawalNotPending = infraerrors.Conflict("AFFILIATE_WITHDRAWAL_NOT_PENDING", "affiliate withdrawal is no longer pending")
)

const (
	affiliateInviteesLimit               = 100
	AffiliateInviteeInactiveDays         = 15
	AffiliateInviteePaidThresholdCNY     = 20.0
	AffiliateInviteeRedeemedThresholdUSD = 60.0
	AffiliateInviteeStatusActive         = "active"
	AffiliateInviteeStatusInvalid        = "invalid"
	AffiliateInviteeStatusPending        = "pending"
	// AffiliateCodeMinLength / AffiliateCodeMaxLength bound both system-generated
	// 12-char codes and admin-customized codes (e.g. "VIP2026").
	AffiliateCodeMinLength = 4
	AffiliateCodeMaxLength = 32
)

// affiliateCodeValidChar accepts uppercase letters, digits, underscore and dash.
// All input passes through strings.ToUpper before validation, so lowercase from
// users is normalized — admins may supply mixed case in their UI.
var affiliateCodeValidChar = func() [256]bool {
	var tbl [256]bool
	for c := byte('A'); c <= 'Z'; c++ {
		tbl[c] = true
	}
	for c := byte('0'); c <= '9'; c++ {
		tbl[c] = true
	}
	tbl['_'] = true
	tbl['-'] = true
	return tbl
}()

// isValidAffiliateCodeFormat validates code format for both binding (user input)
// and admin updates. Caller is expected to upper-case the input first.
func isValidAffiliateCodeFormat(code string) bool {
	if len(code) < AffiliateCodeMinLength || len(code) > AffiliateCodeMaxLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !affiliateCodeValidChar[code[i]] {
			return false
		}
	}
	return true
}

type AffiliateSummary struct {
	UserID               int64     `json:"user_id"`
	AffCode              string    `json:"aff_code"`
	AffCodeCustom        bool      `json:"aff_code_custom"`
	AffRebateRatePercent *float64  `json:"aff_rebate_rate_percent,omitempty"`
	InviterID            *int64    `json:"inviter_id,omitempty"`
	AffCount             int       `json:"aff_count"`
	AffQuota             float64   `json:"aff_quota"`
	AffFrozenQuota       float64   `json:"aff_frozen_quota"`
	AffHistoryQuota      float64   `json:"aff_history_quota"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type AffiliateInvitee struct {
	UserID               int64      `json:"user_id"`
	Email                string     `json:"email"`
	Username             string     `json:"username"`
	Status               string     `json:"status"`
	CreatedAt            *time.Time `json:"created_at,omitempty"`
	BoundAt              *time.Time `json:"bound_at,omitempty"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
	LatestContributionAt *time.Time `json:"latest_contribution_at,omitempty"`
	TotalPaid            float64    `json:"total_paid"`
	TotalRedeemedQuota   float64    `json:"total_redeemed_quota"`
	TotalRebate          float64    `json:"total_rebate"`
}

type AffiliateInviteeCounts struct {
	Total   int
	Active  int
	Invalid int
	Pending int
}

type AffiliateRebateTier struct {
	MinActiveCount int     `json:"min_active_count"`
	MaxActiveCount *int    `json:"max_active_count,omitempty"`
	RatePercent    float64 `json:"rate_percent"`
}

type AffiliateRewardTotals struct {
	ClaimedQuota                float64 `json:"claimed_quota"`
	WithdrawnQuota              float64 `json:"withdrawn_quota"`
	WithdrawnCashAmount         float64 `json:"withdrawn_cash_amount"`
	PendingWithdrawalQuota      float64 `json:"pending_withdrawal_quota"`
	PendingWithdrawalCashAmount float64 `json:"pending_withdrawal_cash_amount"`
}

type AffiliateWithdrawal struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	UserEmail   string     `json:"user_email,omitempty"`
	Username    string     `json:"username,omitempty"`
	QuotaAmount float64    `json:"quota_amount"`
	CashRate    float64    `json:"cash_rate"`
	CashAmount  float64    `json:"cash_amount"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	ProcessedAt *time.Time `json:"processed_at,omitempty"`
	ProcessedBy *int64     `json:"processed_by,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

type AffiliateWithdrawalFilter struct {
	Search   string
	Status   string
	Page     int
	PageSize int
}

type AffiliateDetail struct {
	UserID                      int64                 `json:"user_id"`
	AffCode                     string                `json:"aff_code"`
	InviterID                   *int64                `json:"inviter_id,omitempty"`
	AffCount                    int                   `json:"aff_count"`
	ActiveCount                 int                   `json:"active_count"`
	InvalidCount                int                   `json:"invalid_count"`
	PendingCount                int                   `json:"pending_count"`
	InactiveDays                int                   `json:"inactive_days"`
	AffQuota                    float64               `json:"aff_quota"`
	AffFrozenQuota              float64               `json:"aff_frozen_quota"`
	AffHistoryQuota             float64               `json:"aff_history_quota"`
	ClaimedQuota                float64               `json:"claimed_quota"`
	WithdrawnQuota              float64               `json:"withdrawn_quota"`
	WithdrawnCashAmount         float64               `json:"withdrawn_cash_amount"`
	PendingWithdrawalQuota      float64               `json:"pending_withdrawal_quota"`
	PendingWithdrawalCashAmount float64               `json:"pending_withdrawal_cash_amount"`
	WithdrawalCashRate          float64               `json:"withdrawal_cash_rate"`
	WithdrawableCashAmount      float64               `json:"withdrawable_cash_amount"`
	RebateTiers                 []AffiliateRebateTier `json:"rebate_tiers"`
	// EffectiveRebateRatePercent 是当前用户作为邀请人时实际生效的返利比例：
	// 优先用户自己的专属比例（aff_rebate_rate_percent），否则回退到全局比例。
	// 用于在用户的 /affiliate 页面直观展示「分享后能拿到多少」。
	EffectiveRebateRatePercent float64            `json:"effective_rebate_rate_percent"`
	Invitees                   []AffiliateInvitee `json:"invitees"`
}

type AffiliateRepository interface {
	EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error)
	GetAffiliateByUserID(ctx context.Context, userID int64) (*AffiliateSummary, error)
	GetAffiliateByCode(ctx context.Context, code string) (*AffiliateSummary, error)
	BindInviter(ctx context.Context, userID, inviterID int64) (bool, error)
	AccrueQuota(ctx context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceOrderID *int64, activeInviteeCount int, rebateRatePercent float64) (bool, error)
	GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error)
	ThawFrozenQuota(ctx context.Context, userID int64) (float64, error)
	TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error)
	GetAffiliateRewardTotals(ctx context.Context, userID int64) (AffiliateRewardTotals, error)
	CreateAffiliateWithdrawal(ctx context.Context, userID int64, cashRate float64) (*AffiliateWithdrawal, error)
	ListAffiliateWithdrawals(ctx context.Context, filter AffiliateWithdrawalFilter) ([]AffiliateWithdrawal, int64, error)
	CompleteAffiliateWithdrawal(ctx context.Context, withdrawalID, adminUserID int64, notes string) (*AffiliateWithdrawal, error)
	CancelAffiliateWithdrawal(ctx context.Context, withdrawalID, adminUserID int64, notes string) (*AffiliateWithdrawal, error)
	ListInvitees(ctx context.Context, inviterID int64, limit int, now time.Time) ([]AffiliateInvitee, AffiliateInviteeCounts, error)

	// 管理端：用户级专属配置
	UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error
	ResetUserAffCode(ctx context.Context, userID int64) (string, error)
	SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error
	BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error
	ListUsersWithCustomSettings(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error)
	ListAffiliateInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error)
	ListAffiliateRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error)
	ListAffiliateTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error)
	GetAffiliateUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error)
}

// AffiliateAdminFilter 列表筛选条件
type AffiliateAdminFilter struct {
	Search   string
	Page     int
	PageSize int
}

// AffiliateAdminEntry 专属用户列表条目
type AffiliateAdminEntry struct {
	UserID               int64    `json:"user_id"`
	Email                string   `json:"email"`
	Username             string   `json:"username"`
	AffCode              string   `json:"aff_code"`
	AffCodeCustom        bool     `json:"aff_code_custom"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent,omitempty"`
	AffCount             int      `json:"aff_count"`
}

type AffiliateRecordFilter struct {
	Search   string
	Page     int
	PageSize int
	StartAt  *time.Time
	EndAt    *time.Time
	SortBy   string
	SortDesc bool
}

type AffiliateInviteRecord struct {
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	AffCode         string    `json:"aff_code"`
	TotalRebate     float64   `json:"total_rebate"`
	CreatedAt       time.Time `json:"created_at"`
}

type AffiliateRebateRecord struct {
	OrderID            int64     `json:"order_id"`
	OutTradeNo         string    `json:"out_trade_no"`
	InviterID          int64     `json:"inviter_id"`
	InviterEmail       string    `json:"inviter_email"`
	InviterUsername    string    `json:"inviter_username"`
	InviteeID          int64     `json:"invitee_id"`
	InviteeEmail       string    `json:"invitee_email"`
	InviteeUsername    string    `json:"invitee_username"`
	OrderAmount        float64   `json:"order_amount"`
	PayAmount          float64   `json:"pay_amount"`
	RebateAmount       float64   `json:"rebate_amount"`
	RebateRatePercent  *float64  `json:"rebate_rate_percent,omitempty"`
	ActiveInviteeCount *int      `json:"active_invitee_count,omitempty"`
	PaymentType        string    `json:"payment_type"`
	OrderStatus        string    `json:"order_status"`
	CreatedAt          time.Time `json:"created_at"`
}

type AffiliateTransferRecord struct {
	LedgerID            int64     `json:"ledger_id"`
	UserID              int64     `json:"user_id"`
	UserEmail           string    `json:"user_email"`
	Username            string    `json:"username"`
	Amount              float64   `json:"amount"`
	BalanceAfter        *float64  `json:"balance_after,omitempty"`
	AvailableQuotaAfter *float64  `json:"available_quota_after,omitempty"`
	FrozenQuotaAfter    *float64  `json:"frozen_quota_after,omitempty"`
	HistoryQuotaAfter   *float64  `json:"history_quota_after,omitempty"`
	SnapshotAvailable   bool      `json:"snapshot_available"`
	CurrentBalance      float64   `json:"-"`
	RemainingQuota      float64   `json:"-"`
	FrozenQuota         float64   `json:"-"`
	HistoryQuota        float64   `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
}

type AffiliateUserOverview struct {
	UserID              int64   `json:"user_id"`
	Email               string  `json:"email"`
	Username            string  `json:"username"`
	AffCode             string  `json:"aff_code"`
	RebateRatePercent   float64 `json:"rebate_rate_percent"`
	RebateRateCustom    bool    `json:"-"`
	InvitedCount        int     `json:"invited_count"`
	RebatedInviteeCount int     `json:"rebated_invitee_count"`
	AvailableQuota      float64 `json:"available_quota"`
	HistoryQuota        float64 `json:"history_quota"`
}

type AffiliateService struct {
	repo                 AffiliateRepository
	settingService       *SettingService
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  *BillingCacheService
}

func NewAffiliateService(repo AffiliateRepository, settingService *SettingService, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *AffiliateService {
	return &AffiliateService{
		repo:                 repo,
		settingService:       settingService,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
	}
}

// IsEnabled reports whether the affiliate (邀请返利) feature is turned on.
func (s *AffiliateService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return AffiliateEnabledDefault
	}
	return s.settingService.IsAffiliateEnabled(ctx)
}

func (s *AffiliateService) EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.EnsureUserAffiliate(ctx, userID)
}

func (s *AffiliateService) GetAffiliateDetail(ctx context.Context, userID int64) (*AffiliateDetail, error) {
	// Lazy thaw: move any matured frozen quota to available before reading.
	if s != nil && s.repo != nil {
		// best-effort: thaw failure is non-fatal
		_, _ = s.repo.ThawFrozenQuota(ctx, userID)
	}

	summary, err := s.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	invitees, counts, err := s.listInvitees(ctx, userID)
	if err != nil {
		return nil, err
	}
	totals, err := s.repo.GetAffiliateRewardTotals(ctx, userID)
	if err != nil {
		return nil, err
	}
	tiers := s.rebateTiers(ctx)
	return &AffiliateDetail{
		UserID:                      summary.UserID,
		AffCode:                     summary.AffCode,
		InviterID:                   summary.InviterID,
		AffCount:                    counts.Total,
		ActiveCount:                 counts.Active,
		InvalidCount:                counts.Invalid,
		PendingCount:                counts.Pending,
		InactiveDays:                AffiliateInviteeInactiveDays,
		AffQuota:                    summary.AffQuota,
		AffFrozenQuota:              summary.AffFrozenQuota,
		AffHistoryQuota:             summary.AffHistoryQuota,
		ClaimedQuota:                totals.ClaimedQuota,
		WithdrawnQuota:              totals.WithdrawnQuota,
		WithdrawnCashAmount:         totals.WithdrawnCashAmount,
		PendingWithdrawalQuota:      totals.PendingWithdrawalQuota,
		PendingWithdrawalCashAmount: totals.PendingWithdrawalCashAmount,
		WithdrawalCashRate:          AffiliateWithdrawalCashRate,
		WithdrawableCashAmount:      roundTo(summary.AffQuota*AffiliateWithdrawalCashRate, 8),
		RebateTiers:                 tiers,
		EffectiveRebateRatePercent:  s.resolveRebateRatePercent(ctx, summary, counts.Active),
		Invitees:                    invitees,
	}, nil
}

func (s *AffiliateService) BindInviterByCode(ctx context.Context, userID int64, rawCode string) error {
	return s.bindInviterByIdentifier(ctx, userID, rawCode, false)
}

// BindInviter binds an existing account from the desktop client. Numeric user
// IDs and affiliate codes resolve to the same immutable inviter relationship.
func (s *AffiliateService) BindInviter(ctx context.Context, userID int64, rawIdentifier string) error {
	return s.bindInviterByIdentifier(ctx, userID, rawIdentifier, true)
}

func (s *AffiliateService) bindInviterByIdentifier(ctx context.Context, userID int64, rawIdentifier string, requireEnabled bool) error {
	identifier := strings.TrimSpace(rawIdentifier)
	if identifier == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if !s.IsEnabled(ctx) {
		if requireEnabled {
			return ErrAffiliateDisabled
		}
		return nil
	}

	selfSummary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return err
	}
	if selfSummary.InviterID != nil {
		return nil
	}

	inviterSummary, err := s.resolveInviter(ctx, identifier)
	if err != nil {
		if errors.Is(err, ErrAffiliateProfileNotFound) {
			return ErrAffiliateCodeInvalid
		}
		return err
	}
	if inviterSummary == nil || inviterSummary.UserID <= 0 || inviterSummary.UserID == userID {
		return ErrAffiliateCodeInvalid
	}

	bound, err := s.repo.BindInviter(ctx, userID, inviterSummary.UserID)
	if err != nil {
		return err
	}
	if !bound {
		return ErrAffiliateAlreadyBound
	}
	return nil
}

func (s *AffiliateService) resolveInviter(ctx context.Context, rawIdentifier string) (*AffiliateSummary, error) {
	identifier := strings.TrimSpace(rawIdentifier)
	if numericID, err := strconv.ParseInt(identifier, 10, 64); err == nil && numericID > 0 {
		summary, lookupErr := s.repo.GetAffiliateByUserID(ctx, numericID)
		if lookupErr == nil {
			return summary, nil
		}
		if !errors.Is(lookupErr, ErrAffiliateProfileNotFound) && !errors.Is(lookupErr, ErrUserNotFound) {
			return nil, lookupErr
		}
	}

	code := strings.ToUpper(identifier)
	if !isValidAffiliateCodeFormat(code) {
		return nil, ErrAffiliateCodeInvalid
	}
	return s.repo.GetAffiliateByCode(ctx, code)
}

func (s *AffiliateService) AccrueInviteRebate(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64) (float64, error) {
	return s.AccrueInviteRebateForOrder(ctx, inviteeUserID, baseRechargeAmount, nil)
}

func (s *AffiliateService) AccrueInviteRebateForOrder(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64, sourceOrderID *int64) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if inviteeUserID <= 0 || baseRechargeAmount <= 0 || math.IsNaN(baseRechargeAmount) || math.IsInf(baseRechargeAmount, 0) {
		return 0, nil
	}
	// 总开关关闭时，新充值不再产生返利
	if !s.IsEnabled(ctx) {
		return 0, nil
	}

	inviteeSummary, err := s.repo.EnsureUserAffiliate(ctx, inviteeUserID)
	if err != nil {
		return 0, err
	}
	if inviteeSummary.InviterID == nil || *inviteeSummary.InviterID <= 0 {
		return 0, nil
	}

	// 加载邀请人 profile，优先使用专属比例（覆盖全局）
	inviterSummary, err := s.repo.EnsureUserAffiliate(ctx, *inviteeSummary.InviterID)
	if err != nil {
		return 0, err
	}
	// 有效期检查：超过返利有效期后不再产生返利
	if s.settingService != nil {
		if durationDays := s.settingService.GetAffiliateRebateDurationDays(ctx); durationDays > 0 {
			if time.Now().After(inviteeSummary.CreatedAt.AddDate(0, 0, durationDays)) {
				return 0, nil
			}
		}
	}

	_, counts, err := s.repo.ListInvitees(ctx, inviterSummary.UserID, 1, time.Now().UTC())
	if err != nil {
		return 0, err
	}
	rebateRatePercent := s.resolveRebateRatePercent(ctx, inviterSummary, counts.Active)
	rebate := roundTo(baseRechargeAmount*(rebateRatePercent/100), 8)
	if rebate <= 0 {
		return 0, nil
	}

	// 单人上限检查：精确截断到剩余额度
	if s.settingService != nil {
		if perInviteeCap := s.settingService.GetAffiliateRebatePerInviteeCap(ctx); perInviteeCap > 0 {
			existing, err := s.repo.GetAccruedRebateFromInvitee(ctx, *inviteeSummary.InviterID, inviteeUserID)
			if err != nil {
				return 0, err
			}
			if existing >= perInviteeCap {
				return 0, nil
			}
			if remaining := perInviteeCap - existing; rebate > remaining {
				rebate = roundTo(remaining, 8)
			}
		}
	}

	var freezeHours int
	if s.settingService != nil {
		freezeHours = s.settingService.GetAffiliateRebateFreezeHours(ctx)
	}

	applied, err := s.repo.AccrueQuota(
		ctx,
		*inviteeSummary.InviterID,
		inviteeUserID,
		rebate,
		freezeHours,
		sourceOrderID,
		counts.Active,
		rebateRatePercent,
	)
	if err != nil {
		return 0, err
	}
	if !applied {
		return 0, nil
	}
	return rebate, nil
}

// resolveRebateRatePercent returns the inviter's exclusive rate when set,
// otherwise the global setting value (clamped to [Min, Max]).
func (s *AffiliateService) resolveRebateRatePercent(ctx context.Context, inviter *AffiliateSummary, activeInviteeCount int) float64 {
	if inviter != nil && inviter.AffRebateRatePercent != nil {
		v := *inviter.AffRebateRatePercent
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return s.globalRebateRatePercent(ctx)
		}
		return clampAffiliateRebateRate(v)
	}
	tiers := s.rebateTiers(ctx)
	for _, tier := range tiers {
		if activeInviteeCount < tier.MinActiveCount {
			continue
		}
		if tier.MaxActiveCount == nil || activeInviteeCount <= *tier.MaxActiveCount {
			return tier.RatePercent
		}
	}
	return s.globalRebateRatePercent(ctx)
}

func (s *AffiliateService) rebateTiers(ctx context.Context) []AffiliateRebateTier {
	rates := [4]float64{
		AffiliateRebateTier0To5Default,
		AffiliateRebateTier6To10Default,
		AffiliateRebateTier11To20Default,
		AffiliateRebateTier21PlusDefault,
	}
	if s != nil && s.settingService != nil {
		rates = s.settingService.GetAffiliateRebateTierRates(ctx)
	}
	max5, max10, max20 := 5, 10, 20
	return []AffiliateRebateTier{
		{MinActiveCount: 0, MaxActiveCount: &max5, RatePercent: rates[0]},
		{MinActiveCount: 6, MaxActiveCount: &max10, RatePercent: rates[1]},
		{MinActiveCount: 11, MaxActiveCount: &max20, RatePercent: rates[2]},
		{MinActiveCount: 21, MaxActiveCount: nil, RatePercent: rates[3]},
	}
}

// globalRebateRatePercent reads the system-wide rebate rate via SettingService,
// returning the documented default when SettingService is unavailable.
func (s *AffiliateService) globalRebateRatePercent(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return AffiliateRebateRateDefault
	}
	return s.settingService.GetAffiliateRebateRatePercent(ctx)
}

func (s *AffiliateService) TransferAffiliateQuota(ctx context.Context, userID int64) (float64, float64, error) {
	if s == nil || s.repo == nil {
		return 0, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}

	transferred, balance, err := s.repo.TransferQuotaToBalance(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if transferred > 0 {
		s.invalidateAffiliateCaches(ctx, userID)
	}
	return transferred, balance, nil
}

func (s *AffiliateService) RequestAffiliateWithdrawal(ctx context.Context, userID int64) (*AffiliateWithdrawal, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	withdrawal, err := s.repo.CreateAffiliateWithdrawal(ctx, userID, AffiliateWithdrawalCashRate)
	if err != nil {
		return nil, err
	}
	s.invalidateAffiliateCaches(ctx, userID)
	return withdrawal, nil
}

func (s *AffiliateService) listInvitees(ctx context.Context, inviterID int64) ([]AffiliateInvitee, AffiliateInviteeCounts, error) {
	if s == nil || s.repo == nil {
		return nil, AffiliateInviteeCounts{}, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	now := time.Now().UTC()
	invitees, counts, err := s.repo.ListInvitees(ctx, inviterID, affiliateInviteesLimit, now)
	if err != nil {
		return nil, AffiliateInviteeCounts{}, err
	}
	for i := range invitees {
		invitees[i].Status = affiliateInviteeStatusAt(invitees[i], now)
		invitees[i].Email = maskEmail(invitees[i].Email)
	}
	return invitees, counts, nil
}

func affiliateInviteeStatusAt(invitee AffiliateInvitee, now time.Time) string {
	lastActivityAt := invitee.LastUsedAt
	if lastActivityAt == nil {
		lastActivityAt = invitee.BoundAt
	}
	if lastActivityAt == nil {
		lastActivityAt = invitee.CreatedAt
	}
	if lastActivityAt != nil && now.After(lastActivityAt.Add(AffiliateInviteeInactiveDays*24*time.Hour)) {
		return AffiliateInviteeStatusInvalid
	}
	if invitee.TotalPaid >= AffiliateInviteePaidThresholdCNY ||
		invitee.TotalRedeemedQuota >= AffiliateInviteeRedeemedThresholdUSD {
		return AffiliateInviteeStatusActive
	}
	return AffiliateInviteeStatusPending
}

func roundTo(v float64, scale int) float64 {
	factor := math.Pow10(scale)
	return math.Round(v*factor) / factor
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return "***"
	}

	local := email[:at]
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")

	maskedLocal := maskSegment(local)
	if dot <= 0 || dot >= len(domain)-1 {
		return maskedLocal + "@" + maskSegment(domain)
	}

	domainName := domain[:dot]
	tld := domain[dot:]
	return maskedLocal + "@" + maskSegment(domainName) + tld
}

func maskSegment(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "***"
	}
	if len(r) == 1 {
		return string(r[0]) + "***"
	}
	return string(r[0]) + "***"
}

func (s *AffiliateService) invalidateAffiliateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.affiliate", "[Affiliate] Failed to invalidate billing cache for user %d: %v", userID, err)
		}
	}
}

// =========================
// Admin: 专属配置管理
// =========================

// validateExclusiveRate ensures a per-user override is finite and within
// [Min, Max]. nil is always valid (means "clear / fall back to global").
func validateExclusiveRate(ratePercent *float64) error {
	if ratePercent == nil {
		return nil
	}
	v := *ratePercent
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return infraerrors.BadRequest("INVALID_RATE", "invalid rebate rate")
	}
	if v < AffiliateRebateRateMin || v > AffiliateRebateRateMax {
		return infraerrors.BadRequest("INVALID_RATE", "rebate rate out of range")
	}
	return nil
}

// AdminUpdateUserAffCode 管理员改写用户的邀请码（专属邀请码）。
func (s *AffiliateService) AdminUpdateUserAffCode(ctx context.Context, userID int64, rawCode string) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}
	return s.repo.UpdateUserAffCode(ctx, userID, code)
}

// AdminResetUserAffCode 重置用户邀请码为系统随机码。
func (s *AffiliateService) AdminResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ResetUserAffCode(ctx, userID)
}

// AdminSetUserRebateRate 设置/清除用户专属返利比例。ratePercent==nil 表示清除。
func (s *AffiliateService) AdminSetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	return s.repo.SetUserRebateRate(ctx, userID, ratePercent)
}

// AdminBatchSetUserRebateRate 批量设置/清除用户专属返利比例。
func (s *AffiliateService) AdminBatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.BatchSetUserRebateRate(ctx, cleaned, ratePercent)
}

// AdminListCustomUsers 列出有专属配置的用户。
func (s *AffiliateService) AdminListCustomUsers(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListUsersWithCustomSettings(ctx, filter)
}

func (s *AffiliateService) AdminListInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateInviteRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateRebateRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateTransferRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListWithdrawals(ctx context.Context, filter AffiliateWithdrawalFilter) ([]AffiliateWithdrawal, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.Status = strings.ToLower(strings.TrimSpace(filter.Status))
	return s.repo.ListAffiliateWithdrawals(ctx, filter)
}

func (s *AffiliateService) AdminCompleteWithdrawal(ctx context.Context, withdrawalID, adminUserID int64, notes string) (*AffiliateWithdrawal, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	withdrawal, err := s.repo.CompleteAffiliateWithdrawal(ctx, withdrawalID, adminUserID, strings.TrimSpace(notes))
	if err != nil {
		return nil, err
	}
	s.invalidateAffiliateCaches(ctx, withdrawal.UserID)
	return withdrawal, nil
}

func (s *AffiliateService) AdminCancelWithdrawal(ctx context.Context, withdrawalID, adminUserID int64, notes string) (*AffiliateWithdrawal, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	withdrawal, err := s.repo.CancelAffiliateWithdrawal(ctx, withdrawalID, adminUserID, strings.TrimSpace(notes))
	if err != nil {
		return nil, err
	}
	s.invalidateAffiliateCaches(ctx, withdrawal.UserID)
	return withdrawal, nil
}

func (s *AffiliateService) AdminGetUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	overview, err := s.repo.GetAffiliateUserOverview(ctx, userID)
	if err != nil {
		return nil, err
	}
	if overview != nil {
		if !overview.RebateRateCustom {
			summary, summaryErr := s.repo.GetAffiliateByUserID(ctx, userID)
			if summaryErr != nil {
				return nil, summaryErr
			}
			_, counts, countsErr := s.repo.ListInvitees(ctx, userID, 1, time.Now().UTC())
			if countsErr != nil {
				return nil, countsErr
			}
			overview.RebateRatePercent = s.resolveRebateRatePercent(ctx, summary, counts.Active)
		}
		overview.RebateRatePercent = clampAffiliateRebateRate(overview.RebateRatePercent)
	}
	return overview, nil
}

func normalizeAffiliateRecordFilter(filter AffiliateRecordFilter) AffiliateRecordFilter {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.SortBy = strings.TrimSpace(filter.SortBy)
	return filter
}
