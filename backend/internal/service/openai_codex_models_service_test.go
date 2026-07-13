package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type codexModelsSchedulerCache struct {
	snapshot []*Account
	accounts map[int64]*Account
}

func (c *codexModelsSchedulerCache) GetSnapshot(context.Context, SchedulerBucket) ([]*Account, bool, error) {
	return c.snapshot, true, nil
}

func (c *codexModelsSchedulerCache) SetSnapshot(context.Context, SchedulerBucket, []Account) error {
	return nil
}

func (c *codexModelsSchedulerCache) GetAccount(_ context.Context, accountID int64) (*Account, error) {
	return c.accounts[accountID], nil
}

func (c *codexModelsSchedulerCache) SetAccount(context.Context, *Account) error { return nil }
func (c *codexModelsSchedulerCache) DeleteAccount(context.Context, int64) error { return nil }
func (c *codexModelsSchedulerCache) UpdateLastUsed(context.Context, map[int64]time.Time) error {
	return nil
}
func (c *codexModelsSchedulerCache) TryLockBucket(context.Context, SchedulerBucket, time.Duration) (bool, error) {
	return true, nil
}
func (c *codexModelsSchedulerCache) UnlockBucket(context.Context, SchedulerBucket) error {
	return nil
}
func (c *codexModelsSchedulerCache) ListBuckets(context.Context) ([]SchedulerBucket, error) {
	return nil, nil
}
func (c *codexModelsSchedulerCache) GetOutboxWatermark(context.Context) (int64, error) {
	return 0, nil
}
func (c *codexModelsSchedulerCache) SetOutboxWatermark(context.Context, int64) error {
	return nil
}

func newCodexModelsTestAccount() *Account {
	return &Account{
		ID:       1,
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
		Credentials: map[string]any{
			"access_token":       "test-access-token",
			"chatgpt_account_id": "acc-123",
		},
	}
}

func TestSelectCodexModelsAccountSkipsAPIKeyAccount(t *testing.T) {
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Priority:    1,
			Credentials: map[string]any{"api_key": "sk-api-key"},
		},
		{
			ID:          2,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeOAuth,
			Status:      StatusActive,
			Schedulable: true,
			Priority:    2,
			Credentials: map[string]any{"access_token": "oauth-access-token"},
		},
	}}

	groupID := int64(3)
	s := &OpenAIGatewayService{accountRepo: repo}
	account, err := s.SelectCodexModelsAccount(context.Background(), &groupID)
	if err != nil {
		t.Fatalf("SelectCodexModelsAccount returned error: %v", err)
	}
	if account == nil || account.ID != 2 {
		t.Fatalf("selected account: got %#v, want OAuth account 2", account)
	}
}

func TestSelectCodexModelsAccountReloadsOAuthTokenOmittedFromSchedulerCache(t *testing.T) {
	const accountID int64 = 42
	cacheAccount := &Account{
		ID:          accountID,
		Platform:    PlatformOpenAI,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Priority:    1,
		Credentials: map[string]any{"oauth_type": "chatgpt"},
	}
	fullAccount := *cacheAccount
	fullAccount.Credentials = map[string]any{
		"oauth_type":    "chatgpt",
		"access_token":  "database-access-token",
		"refresh_token": "database-refresh-token",
	}

	repo := stubOpenAIAccountRepo{accounts: []Account{fullAccount}}
	cache := &codexModelsSchedulerCache{
		snapshot: []*Account{cacheAccount},
		accounts: map[int64]*Account{accountID: cacheAccount},
	}
	service := &OpenAIGatewayService{
		accountRepo:       repo,
		schedulerSnapshot: NewSchedulerSnapshotService(cache, nil, repo, nil, nil),
	}

	groupID := int64(3)
	account, err := service.SelectCodexModelsAccount(context.Background(), &groupID)
	if err != nil {
		t.Fatalf("SelectCodexModelsAccount returned error: %v", err)
	}
	if account == nil || account.ID != accountID {
		t.Fatalf("selected account: got %#v, want account %d", account, accountID)
	}
	if got := account.GetOpenAIAccessToken(); got != "database-access-token" {
		t.Fatalf("selected account token: got %q, want database credential", got)
	}
	if got := cacheAccount.GetOpenAIAccessToken(); got != "" {
		t.Fatalf("scheduler cache must remain credential-free, got token %q", got)
	}
}

func TestSelectCodexModelsAccountRejectsAPIKeyOnlyPool(t *testing.T) {
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{
			ID:          1,
			Platform:    PlatformOpenAI,
			Type:        AccountTypeAPIKey,
			Status:      StatusActive,
			Schedulable: true,
			Credentials: map[string]any{"api_key": "sk-api-key"},
		},
	}}

	groupID := int64(3)
	s := &OpenAIGatewayService{accountRepo: repo}
	account, err := s.SelectCodexModelsAccount(context.Background(), &groupID)
	if account != nil {
		t.Fatalf("selected account: got %#v, want nil", account)
	}
	if err == nil {
		t.Fatal("expected no-available-account error, got nil")
	}
	if errors.Is(err, ErrNoAvailableAccounts) {
		return
	}
	if err.Error() != "no available OpenAI accounts" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFetchCodexModelsManifestPassthrough(t *testing.T) {
	manifestBody := `{"models":[{"slug":"gpt-5.5","display_name":"GPT-5.5"}]}`

	var gotAuth, gotAccountID, gotOriginator, gotClientVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccountID = r.Header.Get("chatgpt-account-id")
		gotOriginator = r.Header.Get("Originator")
		gotClientVersion = r.URL.Query().Get("client_version")
		w.Header().Set("ETag", `W/"abc123"`)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(manifestBody))
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	manifest, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "0.137.0", "")
	if err != nil {
		t.Fatalf("FetchCodexModelsManifest returned error: %v", err)
	}

	if string(manifest.Body) != manifestBody {
		t.Errorf("body not passed through verbatim: got %q", manifest.Body)
	}
	if manifest.ETag != `W/"abc123"` {
		t.Errorf("etag not passed through: got %q", manifest.ETag)
	}
	if gotAuth != "Bearer test-access-token" {
		t.Errorf("authorization header: got %q", gotAuth)
	}
	if gotAccountID != "acc-123" {
		t.Errorf("chatgpt-account-id header: got %q", gotAccountID)
	}
	if gotOriginator != "codex_cli_rs" {
		t.Errorf("originator header: got %q", gotOriginator)
	}
	if gotClientVersion != "0.137.0" {
		t.Errorf("client_version query: got %q", gotClientVersion)
	}
}

func TestFetchCodexModelsManifestDefaultClientVersion(t *testing.T) {
	var gotClientVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClientVersion = r.URL.Query().Get("client_version")
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	if _, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "", ""); err != nil {
		t.Fatalf("FetchCodexModelsManifest returned error: %v", err)
	}
	if gotClientVersion != openAICodexProbeVersion {
		t.Errorf("default client_version: got %q, want %q", gotClientVersion, openAICodexProbeVersion)
	}
}

func TestFetchCodexModelsManifestNotModified(t *testing.T) {
	var gotIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		w.Header().Set("ETag", `W/"abc123"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	manifest, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "0.137.0", `W/"abc123"`)
	if err != nil {
		t.Fatalf("FetchCodexModelsManifest returned error: %v", err)
	}
	if !manifest.NotModified {
		t.Error("expected NotModified to be true")
	}
	if gotIfNoneMatch != `W/"abc123"` {
		t.Errorf("if-none-match header: got %q", gotIfNoneMatch)
	}
}

func TestFetchCodexModelsManifestUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"boom"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	original := chatgptCodexModelsURL
	chatgptCodexModelsURL = server.URL
	defer func() { chatgptCodexModelsURL = original }()

	s := &OpenAIGatewayService{}
	if _, err := s.FetchCodexModelsManifest(context.Background(), newCodexModelsTestAccount(), "0.137.0", ""); err == nil {
		t.Fatal("expected error for upstream 500, got nil")
	}
}

func TestFetchCodexModelsManifestMissingToken(t *testing.T) {
	account := newCodexModelsTestAccount()
	delete(account.Credentials, "access_token")

	s := &OpenAIGatewayService{}
	if _, err := s.FetchCodexModelsManifest(context.Background(), account, "0.137.0", ""); err == nil {
		t.Fatal("expected error for missing access token, got nil")
	}
}
