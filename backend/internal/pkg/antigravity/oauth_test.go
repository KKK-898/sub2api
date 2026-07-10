//go:build unit

package antigravity

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// getClientSecret
// ---------------------------------------------------------------------------

func TestGetClientSecret_鐜鍙橀噺璁剧疆(t *testing.T) {
	old := defaultClientSecret
	defaultClientSecret = ""
	t.Cleanup(func() { defaultClientSecret = old })
	t.Setenv(AntigravityOAuthClientSecretEnv, "my-secret-value")

	// 闇€瑕侀噸鏂拌Е鍙?init 閫昏緫锛氭墜鍔ㄤ粠鐜鍙橀噺璇诲彇
	defaultClientSecret = os.Getenv(AntigravityOAuthClientSecretEnv)

	secret, err := getClientSecret()
	if err != nil {
		t.Fatalf("鑾峰彇 client_secret 澶辫触: %v", err)
	}
	if secret != "my-secret-value" {
		t.Errorf("client_secret 涓嶅尮閰? got %s, want my-secret-value", secret)
	}
}

func TestGetClientSecret_鐜鍙橀噺涓虹┖(t *testing.T) {
	old := defaultClientSecret
	defaultClientSecret = ""
	t.Cleanup(func() { defaultClientSecret = old })

	_, err := getClientSecret()
	if err == nil {
		t.Fatal("defaultClientSecret 涓虹┖鏃跺簲杩斿洖閿欒")
	}
	if !strings.Contains(err.Error(), AntigravityOAuthClientSecretEnv) {
		t.Errorf("閿欒淇℃伅搴斿寘鍚幆澧冨彉閲忓悕: got %s", err.Error())
	}
}

func TestGetClientSecret_鐜鍙橀噺鏈缃?t *testing.T) {
	old := defaultClientSecret
	defaultClientSecret = ""
	t.Cleanup(func() { defaultClientSecret = old })

	_, err := getClientSecret()
	if err == nil {
		t.Fatal("defaultClientSecret 涓虹┖鏃跺簲杩斿洖閿欒")
	}
}

func TestGetClientSecret_鐜鍙橀噺鍚┖鏍?t *testing.T) {
	old := defaultClientSecret
	defaultClientSecret = "   "
	t.Cleanup(func() { defaultClientSecret = old })

	_, err := getClientSecret()
	if err == nil {
		t.Fatal("defaultClientSecret 浠呭惈绌烘牸鏃跺簲杩斿洖閿欒")
	}
}

func TestGetClientSecret_鐜鍙橀噺鏈夊墠鍚庣┖鏍?t *testing.T) {
	old := defaultClientSecret
	defaultClientSecret = "  valid-secret  "
	t.Cleanup(func() { defaultClientSecret = old })

	secret, err := getClientSecret()
	if err != nil {
		t.Fatalf("鑾峰彇 client_secret 澶辫触: %v", err)
	}
	if secret != "valid-secret" {
		t.Errorf("搴斿幓闄ゅ墠鍚庣┖鏍? got %q, want %q", secret, "valid-secret")
	}
}

// ---------------------------------------------------------------------------
// ForwardBaseURLs
// ---------------------------------------------------------------------------

func TestForwardBaseURLs_Daily浼樺厛(t *testing.T) {
	urls := ForwardBaseURLs()
	if len(urls) == 0 {
		t.Fatal("ForwardBaseURLs 杩斿洖绌哄垪琛?)
	}

	// daily URL 搴旀帓鍦ㄧ涓€浣?	if urls[0] != antigravityDailyBaseURL {
		t.Errorf("绗竴涓?URL 搴斾负 daily: got %s, want %s", urls[0], antigravityDailyBaseURL)
	}

	// 搴斿寘鍚墍鏈?URL
	if len(urls) != len(BaseURLs) {
		t.Errorf("URL 鏁伴噺涓嶅尮閰? got %d, want %d", len(urls), len(BaseURLs))
	}

	// 楠岃瘉 prod URL 涔熷湪鍒楄〃涓?	found := false
	for _, u := range urls {
		if u == antigravityProdBaseURL {
			found = true
			break
		}
	}
	if !found {
		t.Error("ForwardBaseURLs 涓己灏?prod URL")
	}
}

func TestForwardBaseURLs_涓嶄慨鏀瑰師鍒囩墖(t *testing.T) {
	originalFirst := BaseURLs[0]
	_ = ForwardBaseURLs()
	// 纭繚鍘熷 BaseURLs 鏈淇敼
	if BaseURLs[0] != originalFirst {
		t.Errorf("ForwardBaseURLs 涓嶅簲淇敼鍘熷 BaseURLs: got %s, want %s", BaseURLs[0], originalFirst)
	}
}

// ---------------------------------------------------------------------------
// URLAvailability
// ---------------------------------------------------------------------------

func TestNewURLAvailability(t *testing.T) {
	ua := NewURLAvailability(5 * time.Minute)
	if ua == nil {
		t.Fatal("NewURLAvailability 杩斿洖 nil")
	}
	if ua.ttl != 5*time.Minute {
		t.Errorf("TTL 涓嶅尮閰? got %v, want 5m", ua.ttl)
	}
	if ua.unavailable == nil {
		t.Error("unavailable map 涓嶅簲涓?nil")
	}
}

func TestURLAvailability_MarkUnavailable(t *testing.T) {
	ua := NewURLAvailability(5 * time.Minute)
	testURL := "https://example.com"

	ua.MarkUnavailable(testURL)

	if ua.IsAvailable(testURL) {
		t.Error("鏍囪涓轰笉鍙敤鍚?IsAvailable 搴旇繑鍥?false")
	}
}

func TestURLAvailability_MarkSuccess(t *testing.T) {
	ua := NewURLAvailability(5 * time.Minute)
	testURL := "https://example.com"

	// 鍏堟爣璁颁负涓嶅彲鐢?	ua.MarkUnavailable(testURL)
	if ua.IsAvailable(testURL) {
		t.Error("鏍囪涓轰笉鍙敤鍚庡簲涓嶅彲鐢?)
	}

	// 鏍囪鎴愬姛鍚庡簲鎭㈠鍙敤
	ua.MarkSuccess(testURL)
	if !ua.IsAvailable(testURL) {
		t.Error("MarkSuccess 鍚庡簲鎭㈠鍙敤")
	}

	// 楠岃瘉 lastSuccess 琚缃?	ua.mu.RLock()
	if ua.lastSuccess != testURL {
		t.Errorf("lastSuccess 涓嶅尮閰? got %s, want %s", ua.lastSuccess, testURL)
	}
	ua.mu.RUnlock()
}

func TestURLAvailability_IsAvailable_TTL杩囨湡(t *testing.T) {
	// 浣跨敤鏋佺煭鐨?TTL
	ua := NewURLAvailability(1 * time.Millisecond)
	testURL := "https://example.com"

	ua.MarkUnavailable(testURL)
	// 绛夊緟 TTL 杩囨湡
	time.Sleep(5 * time.Millisecond)

	if !ua.IsAvailable(testURL) {
		t.Error("TTL 杩囨湡鍚?URL 搴旀仮澶嶅彲鐢?)
	}
}

func TestURLAvailability_IsAvailable_鏈爣璁扮殑URL(t *testing.T) {
	ua := NewURLAvailability(5 * time.Minute)
	if !ua.IsAvailable("https://never-marked.com") {
		t.Error("鏈爣璁扮殑 URL 搴旈粯璁ゅ彲鐢?)
	}
}

func TestURLAvailability_GetAvailableURLs(t *testing.T) {
	ua := NewURLAvailability(10 * time.Minute)

	// 榛樿鎵€鏈?URL 閮藉彲鐢?	urls := ua.GetAvailableURLs()
	if len(urls) != len(BaseURLs) {
		t.Errorf("鍙敤 URL 鏁伴噺涓嶅尮閰? got %d, want %d", len(urls), len(BaseURLs))
	}
}

func TestURLAvailability_GetAvailableURLs_鏍囪涓€涓笉鍙敤(t *testing.T) {
	ua := NewURLAvailability(10 * time.Minute)

	if len(BaseURLs) < 2 {
		t.Skip("BaseURLs 灏戜簬 2 涓紝璺宠繃姝ゆ祴璇?)
	}

	ua.MarkUnavailable(BaseURLs[0])
	urls := ua.GetAvailableURLs()

	// 鏍囪鐨?URL 涓嶅簲鍑虹幇鍦ㄥ彲鐢ㄥ垪琛ㄤ腑
	for _, u := range urls {
		if u == BaseURLs[0] {
			t.Errorf("琚爣璁颁笉鍙敤鐨?URL 涓嶅簲鍑虹幇鍦ㄥ彲鐢ㄥ垪琛ㄤ腑: %s", BaseURLs[0])
		}
	}
}

func TestURLAvailability_GetAvailableURLsWithBase(t *testing.T) {
	ua := NewURLAvailability(10 * time.Minute)
	customURLs := []string{"https://a.com", "https://b.com", "https://c.com"}

	urls := ua.GetAvailableURLsWithBase(customURLs)
	if len(urls) != 3 {
		t.Errorf("鍙敤 URL 鏁伴噺涓嶅尮閰? got %d, want 3", len(urls))
	}
}

func TestURLAvailability_GetAvailableURLsWithBase_LastSuccess浼樺厛(t *testing.T) {
	ua := NewURLAvailability(10 * time.Minute)
	customURLs := []string{"https://a.com", "https://b.com", "https://c.com"}

	ua.MarkSuccess("https://c.com")

	urls := ua.GetAvailableURLsWithBase(customURLs)
	if len(urls) != 3 {
		t.Fatalf("鍙敤 URL 鏁伴噺涓嶅尮閰? got %d, want 3", len(urls))
	}
	// c.com 搴旀帓鍦ㄧ涓€浣?	if urls[0] != "https://c.com" {
		t.Errorf("lastSuccess 搴旀帓鍦ㄧ涓€浣? got %s, want https://c.com", urls[0])
	}
	// 鍏朵綑鎸夊師濮嬮『搴?	if urls[1] != "https://a.com" {
		t.Errorf("绗簩涓簲涓?a.com: got %s", urls[1])
	}
	if urls[2] != "https://b.com" {
		t.Errorf("绗笁涓簲涓?b.com: got %s", urls[2])
	}
}

func TestURLAvailability_GetAvailableURLsWithBase_LastSuccess涓嶅彲鐢?t *testing.T) {
	ua := NewURLAvailability(10 * time.Minute)
	customURLs := []string{"https://a.com", "https://b.com"}

	ua.MarkSuccess("https://b.com")
	ua.MarkUnavailable("https://b.com")

	urls := ua.GetAvailableURLsWithBase(customURLs)
	// b.com 琚爣璁颁笉鍙敤锛屼笉搴斿嚭鐜?	if len(urls) != 1 {
		t.Fatalf("鍙敤 URL 鏁伴噺涓嶅尮閰? got %d, want 1", len(urls))
	}
	if urls[0] != "https://a.com" {
		t.Errorf("浠?a.com 搴斿彲鐢? got %s", urls[0])
	}
}

func TestURLAvailability_GetAvailableURLsWithBase_LastSuccess涓嶅湪鍒楄〃涓?t *testing.T) {
	ua := NewURLAvailability(10 * time.Minute)
	customURLs := []string{"https://a.com", "https://b.com"}

	ua.MarkSuccess("https://not-in-list.com")

	urls := ua.GetAvailableURLsWithBase(customURLs)
	// lastSuccess 涓嶅湪鑷畾涔夊垪琛ㄤ腑锛屼笉搴旇娣诲姞
	if len(urls) != 2 {
		t.Fatalf("鍙敤 URL 鏁伴噺涓嶅尮閰? got %d, want 2", len(urls))
	}
}

// ---------------------------------------------------------------------------
// SessionStore
// ---------------------------------------------------------------------------

func TestNewSessionStore(t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	if store == nil {
		t.Fatal("NewSessionStore 杩斿洖 nil")
	}
	if store.sessions == nil {
		t.Error("sessions map 涓嶅簲涓?nil")
	}
}

func TestSessionStore_SetAndGet(t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	session := &OAuthSession{
		State:        "test-state",
		CodeVerifier: "test-verifier",
		ProxyURL:     "http://proxy.example.com",
		CreatedAt:    time.Now(),
	}

	store.Set("session-1", session)

	got, ok := store.Get("session-1")
	if !ok {
		t.Fatal("Get 搴旇繑鍥?true")
	}
	if got.State != "test-state" {
		t.Errorf("State 涓嶅尮閰? got %s", got.State)
	}
	if got.CodeVerifier != "test-verifier" {
		t.Errorf("CodeVerifier 涓嶅尮閰? got %s", got.CodeVerifier)
	}
	if got.ProxyURL != "http://proxy.example.com" {
		t.Errorf("ProxyURL 涓嶅尮閰? got %s", got.ProxyURL)
	}
}

func TestSessionStore_Get_涓嶅瓨鍦?t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("涓嶅瓨鍦ㄧ殑 session 搴旇繑鍥?false")
	}
}

func TestSessionStore_Get_杩囨湡(t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	session := &OAuthSession{
		State:     "expired-state",
		CreatedAt: time.Now().Add(-SessionTTL - time.Minute), // 宸茶繃鏈?	}

	store.Set("expired-session", session)

	_, ok := store.Get("expired-session")
	if ok {
		t.Error("杩囨湡鐨?session 搴旇繑鍥?false")
	}
}

func TestSessionStore_Delete(t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	session := &OAuthSession{
		State:     "to-delete",
		CreatedAt: time.Now(),
	}

	store.Set("del-session", session)
	store.Delete("del-session")

	_, ok := store.Get("del-session")
	if ok {
		t.Error("鍒犻櫎鍚?Get 搴旇繑鍥?false")
	}
}

func TestSessionStore_Delete_涓嶅瓨鍦?t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	// 鍒犻櫎涓嶅瓨鍦ㄧ殑 session 涓嶅簲 panic
	store.Delete("nonexistent")
}

func TestSessionStore_Stop(t *testing.T) {
	store := NewSessionStore()
	store.Stop()

	// 澶氭 Stop 涓嶅簲 panic
	store.Stop()
}

func TestSessionStore_澶氫釜Session(t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	for i := 0; i < 10; i++ {
		session := &OAuthSession{
			State:     "state-" + string(rune('0'+i)),
			CreatedAt: time.Now(),
		}
		store.Set("session-"+string(rune('0'+i)), session)
	}

	// 楠岃瘉閮借兘鍙栧埌
	for i := 0; i < 10; i++ {
		_, ok := store.Get("session-" + string(rune('0'+i)))
		if !ok {
			t.Errorf("session-%d 搴斿瓨鍦?, i)
		}
	}
}

// ---------------------------------------------------------------------------
// GenerateRandomBytes
// ---------------------------------------------------------------------------

func TestGenerateRandomBytes_闀垮害姝ｇ‘(t *testing.T) {
	sizes := []int{0, 1, 16, 32, 64, 128}
	for _, size := range sizes {
		b, err := GenerateRandomBytes(size)
		if err != nil {
			t.Fatalf("GenerateRandomBytes(%d) 澶辫触: %v", size, err)
		}
		if len(b) != size {
			t.Errorf("闀垮害涓嶅尮閰? got %d, want %d", len(b), size)
		}
	}
}

func TestGenerateRandomBytes_涓嶅悓璋冪敤浜х敓涓嶅悓缁撴灉(t *testing.T) {
	b1, err := GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("绗竴娆¤皟鐢ㄥけ璐? %v", err)
	}
	b2, err := GenerateRandomBytes(32)
	if err != nil {
		t.Fatalf("绗簩娆¤皟鐢ㄥけ璐? %v", err)
	}
	// 涓ゆ鐢熸垚鐨勯殢鏈哄瓧鑺傚簲璇ヤ笉鍚岋紙姒傜巼涓婂嚑涔庝笉鍙兘鐩稿悓锛?	if string(b1) == string(b2) {
		t.Error("涓ゆ鐢熸垚鐨勯殢鏈哄瓧鑺傜浉鍚岋紝姒傜巼鏋佷綆锛屽彲鑳芥湁闂")
	}
}

// ---------------------------------------------------------------------------
// GenerateState
// ---------------------------------------------------------------------------

func TestGenerateState_杩斿洖鍊兼牸寮?t *testing.T) {
	state, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState 澶辫触: %v", err)
	}
	if state == "" {
		t.Error("GenerateState 杩斿洖绌哄瓧绗︿覆")
	}
	// base64url 缂栫爜涓嶅簲鍖呭惈 +, /, =
	if strings.ContainsAny(state, "+/=") {
		t.Errorf("GenerateState 杩斿洖鍊煎寘鍚潪 base64url 瀛楃: %s", state)
	}
	// 32 瀛楄妭鐨?base64url 缂栫爜闀垮害搴斾负 43锛堝幓鎺変簡灏鹃儴 = 濉厖锛?	if len(state) != 43 {
		t.Errorf("GenerateState 杩斿洖鍊奸暱搴︿笉鍖归厤: got %d, want 43", len(state))
	}
}

func TestGenerateState_鍞竴鎬?t *testing.T) {
	s1, _ := GenerateState()
	s2, _ := GenerateState()
	if s1 == s2 {
		t.Error("涓ゆ GenerateState 缁撴灉鐩稿悓")
	}
}

// ---------------------------------------------------------------------------
// GenerateSessionID
// ---------------------------------------------------------------------------

func TestGenerateSessionID_杩斿洖鍊兼牸寮?t *testing.T) {
	id, err := GenerateSessionID()
	if err != nil {
		t.Fatalf("GenerateSessionID 澶辫触: %v", err)
	}
	if id == "" {
		t.Error("GenerateSessionID 杩斿洖绌哄瓧绗︿覆")
	}
	// 16 瀛楄妭鐨?hex 缂栫爜闀垮害搴斾负 32
	if len(id) != 32 {
		t.Errorf("GenerateSessionID 杩斿洖鍊奸暱搴︿笉鍖归厤: got %d, want 32", len(id))
	}
	// 楠岃瘉鏄悎娉曠殑 hex 瀛楃涓?	if _, err := hex.DecodeString(id); err != nil {
		t.Errorf("GenerateSessionID 杩斿洖鍊间笉鏄悎娉曠殑 hex 瀛楃涓? %s, err: %v", id, err)
	}
}

func TestGenerateSessionID_鍞竴鎬?t *testing.T) {
	id1, _ := GenerateSessionID()
	id2, _ := GenerateSessionID()
	if id1 == id2 {
		t.Error("涓ゆ GenerateSessionID 缁撴灉鐩稿悓")
	}
}

// ---------------------------------------------------------------------------
// GenerateCodeVerifier
// ---------------------------------------------------------------------------

func TestGenerateCodeVerifier_杩斿洖鍊兼牸寮?t *testing.T) {
	verifier, err := GenerateCodeVerifier()
	if err != nil {
		t.Fatalf("GenerateCodeVerifier 澶辫触: %v", err)
	}
	if verifier == "" {
		t.Error("GenerateCodeVerifier 杩斿洖绌哄瓧绗︿覆")
	}
	// base64url 缂栫爜涓嶅簲鍖呭惈 +, /, =
	if strings.ContainsAny(verifier, "+/=") {
		t.Errorf("GenerateCodeVerifier 杩斿洖鍊煎寘鍚潪 base64url 瀛楃: %s", verifier)
	}
	// 32 瀛楄妭鐨?base64url 缂栫爜闀垮害搴斾负 43
	if len(verifier) != 43 {
		t.Errorf("GenerateCodeVerifier 杩斿洖鍊奸暱搴︿笉鍖归厤: got %d, want 43", len(verifier))
	}
}

func TestGenerateCodeVerifier_鍞竴鎬?t *testing.T) {
	v1, _ := GenerateCodeVerifier()
	v2, _ := GenerateCodeVerifier()
	if v1 == v2 {
		t.Error("涓ゆ GenerateCodeVerifier 缁撴灉鐩稿悓")
	}
}

// ---------------------------------------------------------------------------
// GenerateCodeChallenge
// ---------------------------------------------------------------------------

func TestGenerateCodeChallenge_SHA256_Base64URL(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"

	challenge := GenerateCodeChallenge(verifier)

	// 鎵嬪姩璁＄畻棰勬湡鍊?	hash := sha256.Sum256([]byte(verifier))
	expected := strings.TrimRight(base64.URLEncoding.EncodeToString(hash[:]), "=")

	if challenge != expected {
		t.Errorf("CodeChallenge 涓嶅尮閰? got %s, want %s", challenge, expected)
	}
}

func TestGenerateCodeChallenge_涓嶅惈濉厖瀛楃(t *testing.T) {
	challenge := GenerateCodeChallenge("test-verifier")
	if strings.Contains(challenge, "=") {
		t.Errorf("CodeChallenge 涓嶅簲鍖呭惈 = 濉厖瀛楃: %s", challenge)
	}
}

func TestGenerateCodeChallenge_涓嶅惈闈濽RL瀹夊叏瀛楃(t *testing.T) {
	challenge := GenerateCodeChallenge("another-verifier")
	if strings.ContainsAny(challenge, "+/") {
		t.Errorf("CodeChallenge 涓嶅簲鍖呭惈 + 鎴?/ 瀛楃: %s", challenge)
	}
}

func TestGenerateCodeChallenge_鐩稿悓杈撳叆鐩稿悓杈撳嚭(t *testing.T) {
	c1 := GenerateCodeChallenge("same-verifier")
	c2 := GenerateCodeChallenge("same-verifier")
	if c1 != c2 {
		t.Errorf("鐩稿悓杈撳叆搴斾骇鐢熺浉鍚岃緭鍑? got %s and %s", c1, c2)
	}
}

func TestGenerateCodeChallenge_涓嶅悓杈撳叆涓嶅悓杈撳嚭(t *testing.T) {
	c1 := GenerateCodeChallenge("verifier-1")
	c2 := GenerateCodeChallenge("verifier-2")
	if c1 == c2 {
		t.Error("涓嶅悓杈撳叆搴斾骇鐢熶笉鍚岃緭鍑?)
	}
}

// ---------------------------------------------------------------------------
// BuildAuthorizationURL
// ---------------------------------------------------------------------------

func TestBuildAuthorizationURL_鍙傛暟楠岃瘉(t *testing.T) {
	state := "test-state-123"
	codeChallenge := "test-challenge-abc"

	authURL := BuildAuthorizationURL(state, codeChallenge)

	// 楠岃瘉浠?AuthorizeURL 寮€澶?	if !strings.HasPrefix(authURL, AuthorizeURL+"?") {
		t.Errorf("URL 搴斾互 %s? 寮€澶? got %s", AuthorizeURL, authURL)
	}

	// 瑙ｆ瀽 URL 骞堕獙璇佸弬鏁?	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("瑙ｆ瀽 URL 澶辫触: %v", err)
	}

	params := parsed.Query()

	expectedParams := map[string]string{
		"client_id":              ClientID,
		"redirect_uri":           RedirectURI,
		"response_type":          "code",
		"scope":                  Scopes,
		"state":                  state,
		"code_challenge":         codeChallenge,
		"code_challenge_method":  "S256",
		"access_type":            "offline",
		"prompt":                 "consent",
		"include_granted_scopes": "true",
	}

	for key, want := range expectedParams {
		got := params.Get(key)
		if got != want {
			t.Errorf("鍙傛暟 %s 涓嶅尮閰? got %q, want %q", key, got, want)
		}
	}
}

func TestBuildAuthorizationURL_鍙傛暟鏁伴噺(t *testing.T) {
	authURL := BuildAuthorizationURL("s", "c")
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("瑙ｆ瀽 URL 澶辫触: %v", err)
	}

	params := parsed.Query()
	// 搴斿寘鍚?10 涓弬鏁?	expectedCount := 10
	if len(params) != expectedCount {
		t.Errorf("鍙傛暟鏁伴噺涓嶅尮閰? got %d, want %d", len(params), expectedCount)
	}
}

func TestBuildAuthorizationURL_鐗规畩瀛楃缂栫爜(t *testing.T) {
	state := "state+with/special=chars"
	codeChallenge := "challenge+value"

	authURL := BuildAuthorizationURL(state, codeChallenge)

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("瑙ｆ瀽 URL 澶辫触: %v", err)
	}

	// 瑙ｆ瀽鍚庡簲姝ｇ‘杩樺師鐗规畩瀛楃
	if got := parsed.Query().Get("state"); got != state {
		t.Errorf("state 鍙傛暟缂栫爜/瑙ｇ爜涓嶅尮閰? got %q, want %q", got, state)
	}
}

// ---------------------------------------------------------------------------
// 甯搁噺鍊奸獙璇?// ---------------------------------------------------------------------------

func TestConstants_鍊兼纭?t *testing.T) {
	if AuthorizeURL != "https://accounts.google.com/o/oauth2/v2/auth" {
		t.Errorf("AuthorizeURL 涓嶅尮閰? got %s", AuthorizeURL)
	}
	if TokenURL != "https://oauth2.googleapis.com/token" {
		t.Errorf("TokenURL 涓嶅尮閰? got %s", TokenURL)
	}
	if UserInfoURL != "https://www.googleapis.com/oauth2/v2/userinfo" {
		t.Errorf("UserInfoURL 涓嶅尮閰? got %s", UserInfoURL)
	}
	if ClientID != "1gaoge-google-oauth-client-id-placeholder" {
		t.Errorf("ClientID 涓嶅尮閰? got %s", ClientID)
	}
	secret, err := getClientSecret()
	if err != nil {
		t.Fatalf("getClientSecret 搴旇繑鍥為粯璁ゅ€硷紝浣嗘姤閿? %v", err)
	}
	if secret != "gaoge-google-oauth-client-secret-placeholder" {
		t.Errorf("榛樿 client_secret 涓嶅尮閰? got %s", secret)
	}
	if RedirectURI != "http://localhost:8085/callback" {
		t.Errorf("RedirectURI 涓嶅尮閰? got %s", RedirectURI)
	}
	if GetUserAgent() != "antigravity/1.23.2 windows/amd64" {
		t.Errorf("UserAgent 涓嶅尮閰? got %s", GetUserAgent())
	}
	if SessionTTL != 30*time.Minute {
		t.Errorf("SessionTTL 涓嶅尮閰? got %v", SessionTTL)
	}
	if URLAvailabilityTTL != 5*time.Minute {
		t.Errorf("URLAvailabilityTTL 涓嶅尮閰? got %v", URLAvailabilityTTL)
	}
}

func TestScopes_鍖呭惈蹇呰鑼冨洿(t *testing.T) {
	expectedScopes := []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
		"https://www.googleapis.com/auth/cclog",
		"https://www.googleapis.com/auth/experimentsandconfigs",
	}

	for _, scope := range expectedScopes {
		if !strings.Contains(Scopes, scope) {
			t.Errorf("Scopes 缂哄皯 %s", scope)
		}
	}
}
