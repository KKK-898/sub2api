package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	PlatformSubscriptionActionSubscription = "subscription"
	PlatformSubscriptionActionBalance      = "balance"
	PlatformSubscriptionActionDeny         = "deny"
)

var ErrPlatformSubscriptionBillingDenied = infraerrors.Forbidden(
	"PLATFORM_SUBSCRIPTION_BILLING_DENIED",
	"request is not allowed by platform subscription billing rules",
)

type PlatformSubscriptionUsageCounter interface {
	GetPlatformSubscriptionUsed(ctx context.Context, userID int64, grantDate string) (float64, error)
	IncrPlatformSubscriptionUsed(ctx context.Context, userID int64, grantDate string, cost float64, ttl time.Duration) (float64, error)
	EnsurePlatformSubscriptionUsedAtLeast(ctx context.Context, userID int64, grantDate string, used float64, ttl time.Duration) error
	GetPlatformSubscriptionCache(ctx context.Context, key string) ([]byte, bool, error)
	SetPlatformSubscriptionCache(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type PlatformSubscriptionBillingRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	MatchType  string `json:"match_type"`
	MatchMode  string `json:"match_mode"`
	MatchValue string `json:"match_value"`
	Action     string `json:"action"`
	Priority   int    `json:"priority"`
}

type PlatformSubscriptionToday struct {
	HasSubscription  bool    `json:"has_subscription"`
	UserID           int64   `json:"-"`
	SubscriptionID   string  `json:"subscription_id"`
	Status           string  `json:"status"`
	GrantID          string  `json:"grant_id"`
	GrantDate        string  `json:"grant_date"`
	GrantAmountUSD   float64 `json:"grant_amount_usd"`
	UsedUSD          float64 `json:"used_usd"`
	RemainingUSD     float64 `json:"remaining_usd"`
	UsageError       string  `json:"usage_error"`
	SubscriptionEnds string  `json:"expires_at"`
}

type PlatformSubscriptionBillingInput struct {
	UserID         int64
	RequestedModel string
	GroupID        *int64
	GroupName      string
	Platform       string
}

type PlatformSubscriptionBillingDecision struct {
	Active       bool
	Action       string
	RuleID       string
	GrantID      string
	GrantDate    string
	GrantAmount  float64
	UsedAmount   float64
	Remaining    float64
	Reason       string
	MatchedBy    string
	Personal     float64
	TotalBalance float64
}

func (d *PlatformSubscriptionBillingDecision) Clone() *PlatformSubscriptionBillingDecision {
	if d == nil {
		return nil
	}
	out := *d
	return &out
}

func (d *PlatformSubscriptionBillingDecision) IsActive() bool {
	return d != nil && d.Active && d.Action != ""
}

func (d *PlatformSubscriptionBillingDecision) UseBalance(reason string, totalBalance, personal float64) {
	if d == nil {
		return
	}
	d.Action = PlatformSubscriptionActionBalance
	d.Reason = strings.TrimSpace(reason)
	d.TotalBalance = totalBalance
	d.Personal = personal
}

type platformSubscriptionContextKey struct{}

type platformSubscriptionRequestState struct {
	RequestedModel string
	Decision       *PlatformSubscriptionBillingDecision
}

func ContextWithPlatformSubscriptionRequest(ctx context.Context, requestedModel string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	requestedModel = strings.TrimSpace(requestedModel)
	if state, ok := ctx.Value(platformSubscriptionContextKey{}).(*platformSubscriptionRequestState); ok && state != nil {
		if requestedModel != "" {
			state.RequestedModel = requestedModel
		}
		return ctx
	}
	return context.WithValue(ctx, platformSubscriptionContextKey{}, &platformSubscriptionRequestState{
		RequestedModel: requestedModel,
	})
}

func PlatformSubscriptionRequestedModelFromContext(ctx context.Context) string {
	if state, ok := ctx.Value(platformSubscriptionContextKey{}).(*platformSubscriptionRequestState); ok && state != nil {
		return strings.TrimSpace(state.RequestedModel)
	}
	return ""
}

func PlatformSubscriptionDecisionFromContext(ctx context.Context) *PlatformSubscriptionBillingDecision {
	if state, ok := ctx.Value(platformSubscriptionContextKey{}).(*platformSubscriptionRequestState); ok && state != nil {
		return state.Decision
	}
	return nil
}

func setPlatformSubscriptionDecisionOnContext(ctx context.Context, decision *PlatformSubscriptionBillingDecision) {
	if state, ok := ctx.Value(platformSubscriptionContextKey{}).(*platformSubscriptionRequestState); ok && state != nil {
		state.Decision = decision
	}
}

type platformSubscriptionRulesPayload struct {
	Rules         []PlatformSubscriptionBillingRule `json:"rules"`
	DefaultAction string                            `json:"default_action"`
	UpdatedAt     string                            `json:"updated_at"`
}

type platformSubscriptionCacheEntry[T any] struct {
	Value     T
	FetchedAt time.Time
	ExpiresAt time.Time
}

type PlatformSubscriptionBillingService struct {
	cfg     config.PlatformSubscriptionBillingConfig
	client  *http.Client
	counter PlatformSubscriptionUsageCounter

	rulesMu sync.RWMutex
	rules   *platformSubscriptionCacheEntry[platformSubscriptionRulesPayload]

	userMu sync.RWMutex
	users  map[int64]*platformSubscriptionCacheEntry[PlatformSubscriptionToday]
}

type platformSubscriptionBackendHTTPError struct {
	BaseURL    string
	StatusCode int
	Body       string
}

func (e *platformSubscriptionBackendHTTPError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("backend %s returned %d: %s", e.BaseURL, e.StatusCode, strings.TrimSpace(e.Body))
}

func NewPlatformSubscriptionBillingService(cfg *config.Config, counter PlatformSubscriptionUsageCounter) *PlatformSubscriptionBillingService {
	localCfg := config.PlatformSubscriptionBillingConfig{}
	if cfg != nil {
		localCfg = cfg.PlatformSubscriptionBilling
	}
	timeout := time.Duration(localCfg.HTTPTimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &PlatformSubscriptionBillingService{
		cfg:     localCfg,
		client:  &http.Client{Timeout: timeout},
		counter: counter,
		users:   make(map[int64]*platformSubscriptionCacheEntry[PlatformSubscriptionToday]),
	}
}

func (s *PlatformSubscriptionBillingService) Enabled() bool {
	return s != nil &&
		s.cfg.Enabled &&
		strings.TrimSpace(s.cfg.BackendURL) != "" &&
		strings.TrimSpace(s.cfg.InternalToken) != ""
}

func (s *PlatformSubscriptionBillingService) Resolve(ctx context.Context, input PlatformSubscriptionBillingInput) (*PlatformSubscriptionBillingDecision, bool) {
	if !s.Enabled() || input.UserID <= 0 {
		return nil, false
	}
	today, ok := s.getUserToday(ctx, input.UserID)
	if !ok || !today.HasSubscription || strings.TrimSpace(today.GrantID) == "" || strings.TrimSpace(today.GrantDate) == "" || today.GrantAmountUSD <= 0 {
		return nil, false
	}
	if err := s.refreshTodayUsageFromCounter(ctx, &today); err != nil {
		logger.LegacyPrintf("service.platform_subscription_billing",
			"Warning: platform subscription used counter refresh failed user=%d grant_date=%s: %v",
			input.UserID, today.GrantDate, err)
	}

	rulesPayload, ok := s.getRules(ctx)
	if !ok {
		return nil, false
	}
	action := normalizePlatformSubscriptionAction(rulesPayload.DefaultAction)
	if action == "" {
		action = PlatformSubscriptionActionSubscription
	}
	decision := &PlatformSubscriptionBillingDecision{
		Active:      true,
		Action:      action,
		GrantID:     today.GrantID,
		GrantDate:   today.GrantDate,
		GrantAmount: today.GrantAmountUSD,
		UsedAmount:  today.UsedUSD,
		Remaining:   maxFloat(today.RemainingUSD, 0),
		Reason:      "default",
	}
	if rule, matched := matchPlatformSubscriptionRule(rulesPayload.Rules, input); matched {
		decision.Action = normalizePlatformSubscriptionAction(rule.Action)
		if decision.Action == "" {
			decision.Action = PlatformSubscriptionActionBalance
		}
		decision.RuleID = rule.ID
		decision.MatchedBy = rule.MatchType
		decision.Reason = "matched_rule"
	}
	return decision, true
}

func (s *PlatformSubscriptionBillingService) IncrementUserUsed(ctx context.Context, userID int64, decision *PlatformSubscriptionBillingDecision, cost float64) {
	if s == nil || s.counter == nil || userID <= 0 || decision == nil || !decision.IsActive() || decision.Action != PlatformSubscriptionActionSubscription || cost <= 0 {
		return
	}
	ttl := s.platformUsageCounterTTL()
	newUsed, err := s.counter.IncrPlatformSubscriptionUsed(ctx, userID, decision.GrantDate, cost, ttl)
	if err != nil {
		logger.LegacyPrintf("service.platform_subscription_billing",
			"ALERT: platform subscription used counter increment failed user=%d grant_date=%s cost=%f: %v",
			userID, decision.GrantDate, cost, err)
		return
	}
	decision.UsedAmount = newUsed
	decision.Remaining = maxFloat(decision.GrantAmount-newUsed, 0)
}

func (s *PlatformSubscriptionBillingService) refreshTodayUsageFromCounter(ctx context.Context, today *PlatformSubscriptionToday) error {
	if s == nil || s.counter == nil || today == nil || today.UserID <= 0 || today.GrantDate == "" {
		return nil
	}
	ttl := s.platformUsageCounterTTL()
	if today.UsedUSD > 0 {
		if err := s.counter.EnsurePlatformSubscriptionUsedAtLeast(ctx, today.UserID, today.GrantDate, today.UsedUSD, ttl); err != nil {
			return err
		}
	}
	used, err := s.counter.GetPlatformSubscriptionUsed(ctx, today.UserID, today.GrantDate)
	if err != nil {
		return err
	}
	if used > today.UsedUSD {
		today.UsedUSD = used
	}
	today.RemainingUSD = maxFloat(today.GrantAmountUSD-today.UsedUSD, 0)
	return nil
}

func (s *PlatformSubscriptionBillingService) getRules(ctx context.Context) (platformSubscriptionRulesPayload, bool) {
	if cached, ok := s.getRulesCache(false); ok {
		return cached, true
	}
	if cached, ok := s.getRulesRedisCache(ctx); ok {
		return cached, true
	}
	payload, err := s.fetchRules(ctx)
	if err == nil {
		sortPlatformSubscriptionRules(payload.Rules)
		s.setRulesCache(payload)
		s.setRulesRedisCache(ctx, payload)
		return payload, true
	}
	if cached, ok := s.getRulesCache(true); ok {
		logger.LegacyPrintf("service.platform_subscription_billing", "Warning: using stale platform subscription billing rules cache: %v", err)
		return cached, true
	}
	logger.LegacyPrintf("service.platform_subscription_billing", "Warning: platform subscription billing rules unavailable, fail-open: %v", err)
	return platformSubscriptionRulesPayload{}, false
}

func (s *PlatformSubscriptionBillingService) getUserToday(ctx context.Context, userID int64) (PlatformSubscriptionToday, bool) {
	if cached, ok := s.getUserCache(userID, false); ok {
		return cached, true
	}
	if cached, ok := s.getUserRedisCache(ctx, userID); ok {
		return cached, true
	}
	today, err := s.fetchUserToday(ctx, userID)
	if err == nil {
		s.setUserCache(userID, today)
		s.setUserRedisCache(ctx, userID, today)
		return today, true
	}
	if cached, ok := s.getUserCache(userID, true); ok {
		logger.LegacyPrintf("service.platform_subscription_billing", "Warning: using stale platform subscription user cache user=%d: %v", userID, err)
		return cached, true
	}
	logger.LegacyPrintf("service.platform_subscription_billing", "Warning: platform subscription user status unavailable, fail-open user=%d: %v", userID, err)
	return PlatformSubscriptionToday{}, false
}

func (s *PlatformSubscriptionBillingService) getRulesCache(allowStale bool) (platformSubscriptionRulesPayload, bool) {
	s.rulesMu.RLock()
	entry := s.rules
	s.rulesMu.RUnlock()
	if entry == nil {
		return platformSubscriptionRulesPayload{}, false
	}
	if allowStale || time.Now().Before(entry.ExpiresAt) {
		return entry.Value, true
	}
	return platformSubscriptionRulesPayload{}, false
}

func (s *PlatformSubscriptionBillingService) setRulesCache(value platformSubscriptionRulesPayload) {
	s.rulesMu.Lock()
	defer s.rulesMu.Unlock()
	now := time.Now()
	s.rules = &platformSubscriptionCacheEntry[platformSubscriptionRulesPayload]{
		Value:     value,
		FetchedAt: now,
		ExpiresAt: now.Add(s.rulesCacheTTL()),
	}
}

func (s *PlatformSubscriptionBillingService) getUserCache(userID int64, allowStale bool) (PlatformSubscriptionToday, bool) {
	s.userMu.RLock()
	entry := s.users[userID]
	s.userMu.RUnlock()
	if entry == nil {
		return PlatformSubscriptionToday{}, false
	}
	if allowStale || time.Now().Before(entry.ExpiresAt) {
		return entry.Value, true
	}
	return PlatformSubscriptionToday{}, false
}

func (s *PlatformSubscriptionBillingService) setUserCache(userID int64, value PlatformSubscriptionToday) {
	s.userMu.Lock()
	defer s.userMu.Unlock()
	now := time.Now()
	s.users[userID] = &platformSubscriptionCacheEntry[PlatformSubscriptionToday]{
		Value:     value,
		FetchedAt: now,
		ExpiresAt: now.Add(s.userCacheTTL()),
	}
}

func (s *PlatformSubscriptionBillingService) getRulesRedisCache(ctx context.Context) (platformSubscriptionRulesPayload, bool) {
	var payload platformSubscriptionRulesPayload
	if !s.getJSONCache(ctx, "platform_sub:rules", &payload) {
		return platformSubscriptionRulesPayload{}, false
	}
	sortPlatformSubscriptionRules(payload.Rules)
	s.setRulesCache(payload)
	return payload, true
}

func (s *PlatformSubscriptionBillingService) setRulesRedisCache(ctx context.Context, value platformSubscriptionRulesPayload) {
	s.setJSONCache(ctx, "platform_sub:rules", value, s.rulesCacheTTL())
}

func (s *PlatformSubscriptionBillingService) getUserRedisCache(ctx context.Context, userID int64) (PlatformSubscriptionToday, bool) {
	var today PlatformSubscriptionToday
	if !s.getJSONCache(ctx, platformSubscriptionUserTodayCacheKey(userID), &today) {
		return PlatformSubscriptionToday{}, false
	}
	if today.UserID == 0 {
		today.UserID = userID
	}
	s.setUserCache(userID, today)
	return today, true
}

func (s *PlatformSubscriptionBillingService) setUserRedisCache(ctx context.Context, userID int64, value PlatformSubscriptionToday) {
	s.setJSONCache(ctx, platformSubscriptionUserTodayCacheKey(userID), value, s.userCacheTTL())
}

func (s *PlatformSubscriptionBillingService) getJSONCache(ctx context.Context, key string, out any) bool {
	if s == nil || s.counter == nil || strings.TrimSpace(key) == "" {
		return false
	}
	data, ok, err := s.counter.GetPlatformSubscriptionCache(ctx, key)
	if err != nil {
		logger.LegacyPrintf("service.platform_subscription_billing", "Warning: platform subscription cache read failed key=%s: %v", key, err)
		return false
	}
	if !ok || len(data) == 0 {
		return false
	}
	if err := json.Unmarshal(data, out); err != nil {
		logger.LegacyPrintf("service.platform_subscription_billing", "Warning: platform subscription cache parse failed key=%s: %v", key, err)
		return false
	}
	return true
}

func (s *PlatformSubscriptionBillingService) setJSONCache(ctx context.Context, key string, value any, ttl time.Duration) {
	if s == nil || s.counter == nil || strings.TrimSpace(key) == "" {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		logger.LegacyPrintf("service.platform_subscription_billing", "Warning: platform subscription cache marshal failed key=%s: %v", key, err)
		return
	}
	if err := s.counter.SetPlatformSubscriptionCache(ctx, key, data, ttl); err != nil {
		logger.LegacyPrintf("service.platform_subscription_billing", "Warning: platform subscription cache write failed key=%s: %v", key, err)
	}
}

func platformSubscriptionUserTodayCacheKey(userID int64) string {
	return "platform_sub:user:" + strconv.FormatInt(userID, 10) + ":today"
}

func (s *PlatformSubscriptionBillingService) fetchRules(ctx context.Context) (platformSubscriptionRulesPayload, error) {
	var payload platformSubscriptionRulesPayload
	err := s.getJSON(ctx, "/internal/sub2/platform-subscription/billing-rules", &payload)
	return payload, err
}

func (s *PlatformSubscriptionBillingService) fetchUserToday(ctx context.Context, userID int64) (PlatformSubscriptionToday, error) {
	var today PlatformSubscriptionToday
	path := "/internal/sub2/platform-subscription/users/" + url.PathEscape(strconv.FormatInt(userID, 10)) + "/today"
	err := s.getJSON(ctx, path, &today)
	if today.UserID == 0 {
		today.UserID = userID
	}
	return today, err
}

func (s *PlatformSubscriptionBillingService) getJSON(ctx context.Context, path string, out any) error {
	bases := s.backendBaseURLs()
	if len(bases) == 0 {
		return fmt.Errorf("backend url is empty")
	}
	errs := make([]string, 0, len(bases))
	for _, base := range bases {
		err := s.getJSONFromBase(ctx, base, path, out)
		if err == nil {
			return nil
		}
		var httpErr *platformSubscriptionBackendHTTPError
		if errors.As(err, &httpErr) && (httpErr.StatusCode == http.StatusUnauthorized || httpErr.StatusCode == http.StatusForbidden) {
			return err
		}
		errs = append(errs, err.Error())
	}
	return fmt.Errorf("all backend urls failed: %s", strings.Join(errs, "; "))
}

func (s *PlatformSubscriptionBillingService) backendBaseURLs() []string {
	raw := strings.TrimSpace(s.cfg.BackendURL)
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		base := strings.TrimRight(strings.TrimSpace(part), "/")
		if base == "" {
			continue
		}
		if _, ok := seen[base]; ok {
			continue
		}
		seen[base] = struct{}{}
		out = append(out, base)
	}
	return out
}

func (s *PlatformSubscriptionBillingService) getJSONFromBase(ctx context.Context, base string, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(s.cfg.InternalToken))
	req.Header.Set("Accept", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &platformSubscriptionBackendHTTPError{
			BaseURL:    base,
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}
	var envelope struct {
		Success bool            `json:"success"`
		Data    json.RawMessage `json:"data"`
		Error   string          `json:"error"`
		Message string          `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Data) > 0 {
		if !envelope.Success {
			msg := envelope.Error
			if msg == "" {
				msg = envelope.Message
			}
			return fmt.Errorf("backend error: %s", msg)
		}
		return json.Unmarshal(envelope.Data, out)
	}
	return json.Unmarshal(body, out)
}

func (s *PlatformSubscriptionBillingService) rulesCacheTTL() time.Duration {
	seconds := s.cfg.RulesTTLSeconds
	if seconds <= 0 {
		seconds = 600
	}
	return time.Duration(seconds) * time.Second
}

func (s *PlatformSubscriptionBillingService) userCacheTTL() time.Duration {
	seconds := s.cfg.UserTTLSeconds
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

func (s *PlatformSubscriptionBillingService) platformUsageCounterTTL() time.Duration {
	ttl := 48 * time.Hour
	if userTTL := s.userCacheTTL(); userTTL > ttl {
		ttl = userTTL
	}
	return ttl
}

func matchPlatformSubscriptionRule(rules []PlatformSubscriptionBillingRule, input PlatformSubscriptionBillingInput) (PlatformSubscriptionBillingRule, bool) {
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if platformSubscriptionRuleMatches(rule, input) {
			return rule, true
		}
	}
	return PlatformSubscriptionBillingRule{}, false
}

func sortPlatformSubscriptionRules(rules []PlatformSubscriptionBillingRule) {
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return rules[i].ID < rules[j].ID
		}
		return rules[i].Priority < rules[j].Priority
	})
}

func platformSubscriptionRuleMatches(rule PlatformSubscriptionBillingRule, input PlatformSubscriptionBillingInput) bool {
	matchType := strings.ToLower(strings.TrimSpace(rule.MatchType))
	matchMode := strings.ToLower(strings.TrimSpace(rule.MatchMode))
	matchValue := strings.TrimSpace(rule.MatchValue)
	if matchMode == "" {
		matchMode = "exact"
	}
	switch matchType {
	case "model":
		return matchPlatformSubscriptionValue(input.RequestedModel, matchValue, matchMode)
	case "group":
		if input.GroupID != nil && matchPlatformSubscriptionValue(strconv.FormatInt(*input.GroupID, 10), matchValue, matchMode) {
			return true
		}
		return matchPlatformSubscriptionValue(input.GroupName, matchValue, matchMode)
	default:
		return false
	}
}

func matchPlatformSubscriptionValue(candidate, pattern, mode string) bool {
	candidate = strings.ToLower(strings.TrimSpace(candidate))
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if candidate == "" || pattern == "" {
		return false
	}
	switch mode {
	case "contains":
		return strings.Contains(candidate, pattern)
	case "wildcard":
		return wildcardMatch(pattern, candidate)
	default:
		return candidate == pattern
	}
}

func wildcardMatch(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return value == pattern
	}
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		if i == 0 && !strings.HasPrefix(pattern, "*") && idx != 0 {
			return false
		}
		pos += idx + len(part)
	}
	if !strings.HasSuffix(pattern, "*") {
		last := parts[len(parts)-1]
		return strings.HasSuffix(value, last)
	}
	return true
}

func normalizePlatformSubscriptionAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case PlatformSubscriptionActionSubscription:
		return PlatformSubscriptionActionSubscription
	case PlatformSubscriptionActionDeny:
		return PlatformSubscriptionActionDeny
	case PlatformSubscriptionActionBalance:
		return PlatformSubscriptionActionBalance
	default:
		return ""
	}
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
