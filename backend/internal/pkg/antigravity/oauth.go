package antigravity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	// Google OAuth 缂傚倸鍊烽悞锕€螞韫囨稑鍨傞柟鎯版绾?
	AuthorizeURL = "https://accounts.google.com/o/oauth2/v2/auth"
	TokenURL     = "https://oauth2.googleapis.com/token"
	UserInfoURL  = "https://www.googleapis.com/oauth2/v2/userinfo"

	// Antigravity OAuth 闂備浇顕ф鍝ョ不瀹ュ鍨傛繛宸簻閺勩儲绻涢幋鐐寸殤闁告纰嶉妵鍕冀閵娿劌顥濆┑鐐茬墕閻栧ジ骞?
	ClientID = "1gaoge-google-oauth-client-id-placeholder"

	// AntigravityOAuthClientSecretEnv 闂?Antigravity OAuth client_secret 闂傚倷鐒﹂惇褰掑礉瀹€鈧埀顒佸嚬閸ｏ綁鎮伴鈧畷鍗炍熺紒妯煎娇闂備礁鎲￠崝鎴﹀礉瀹€鈧槐鐐寸瑹閳ь剙顫忓ú顏勭闁圭儤姊婚鍥⒑鐠団€虫灍婵炶尙鍠愭穱?
	AntigravityOAuthClientSecretEnv = "ANTIGRAVITY_OAUTH_CLIENT_SECRET"

	// AntigravityUserAgentVersionEnv 闂?Antigravity User-Agent 闂傚倷鑳剁划顖炪€冮崨瀛樺亱濠电姴鍋婇懓鍨归崗鍏肩稇闁活厽顨呴—鍐偓锝庝簻椤掋垽鏌ｉ敐澶夋喚闁哄矉绲介…銊︽償閿涘嫪鍝楁俊鐐€х徊濠氬礉閹存繄鏆﹂柨婵嗘媼濞笺劑鏌嶈閸撶喎鐣烽崘瑁佹椽顢旈崟顔ф洟鎮楅獮鍨姎闁哥喓濞€瀹?
	AntigravityUserAgentVersionEnv = "ANTIGRAVITY_USER_AGENT_VERSION"

	// DefaultUserAgentVersion 闂傚倷绀侀幖顐も偓姘卞厴瀹曞綊宕￠悜鍥偓鎸庛亜閹惧崬鐏柣鎺旂帛閹便劌螣閾忕櫢绱炵紓浣哄У閻楃娀寮婚敐鍜佺叆閹艰揪绱曟禒鈺侇渻閵堝繒绁烽柛鏃€鍨甸悾鐑芥晲閸滀焦寤洪梺閫炲苯澧寸€规洖鍟跨叅妞ゅ繐瀚娲⒑閸濆嫯鐧侀柛鏇ㄥ亐閸嬫捇鎮介崨濠勫幈濠电偛妫欓崝鏇㈩敁濠婂嫨浜滈柡鍐ｅ亾婵炲弶绮庡Σ鎰板籍閸偅鏅╁┑鐐叉閸嬫捇鎳滆ぐ鎺撯拺闁绘垟鏅滃▍鍡涙煟濡や緡娈旀い顏勫暣瀹曟帒鈽夊▎蹇ｂ偓鎾绘煟閻樺厖鑸柛鏂跨Ч瀵疇绠涘☉娆戝幗闂侀潧顭堥崕閬嶎敂椤撶姷纾兼い鏇炴噹閻忥綁鏌熷畡閭﹀剶濠碘€崇埣瀹曟儼顧侀柡鍡╁弮濮婃椽宕崟顓炩拡闂佸憡鎸婚悷銊╁Φ閹版澘閿ゆ俊銈傚亾鏉?
	DefaultUserAgentVersion = "1.23.2"

	// 闂傚倷鐒﹂幃鍫曞磿闁秴绠规い鎰堕檮閸嬧晛螖閿濆懎鏆為柛?redirect_uri闂傚倷鐒︾€笛呯矙閹达附鍋嬮煫鍥ㄧ☉閺嬩線鏌曢崼婵愭Ц缂佺媴缍侀弻锝夊箣閿濆憛鎾趁归悩绛硅€块柡宀€鍠栧Λ鍐ㄢ槈閸楃偛澹堝┑鐘愁問閸犳帡寮插鍐惧殫闁告洦鍋掗崥瀣煕濞戝崬骞橀柛?code闂?
	RedirectURI = "http://localhost:8085/callback"

	// OAuth scopes
	Scopes = "https://www.googleapis.com/auth/cloud-platform " +
		"https://www.googleapis.com/auth/userinfo.email " +
		"https://www.googleapis.com/auth/userinfo.profile " +
		"https://www.googleapis.com/auth/cclog " +
		"https://www.googleapis.com/auth/experimentsandconfigs"

	// Session 闂備礁鎼ˇ顐﹀疾濞戞◤娲晝閳ь剟鏁冮姀銈嗘櫢闁绘灏欓惈鍕⒑閸撴彃浜濇繛鍙夛耿閺?
	SessionTTL = 30 * time.Minute

	// URL 闂傚倷绀侀幉锟犳偡椤栫偛鍨傞柟鎯版閺嬩線鏌曢崼婵愭Ц缂?TTL闂傚倷鐒︾€笛呯矙閹达附鍋嬮柛娑卞灣缁犳柨顭块懜闈涘闁活厽顨嗛妵鍕冀閵娧呯厑闂?URL 闂傚倷鐒﹂惇褰掑礉瀹€鈧埀顒佸嚬閸樼晫绮嬮幒妤婃晩闁芥ê顦辩粣鐐烘倵楠炲灝鍔氭俊顐ｇ懇钘濈憸鏂款潖婵犳艾纾兼繝闈涙搐濞懷呯磽?
	URLAvailabilityTTL = 5 * time.Minute

	// Antigravity API 缂傚倸鍊烽悞锕€螞韫囨稑鍨傞柟鎯版绾?
	antigravityProdBaseURL  = "https://cloudcode-pa.googleapis.com"
	antigravityDailyBaseURL = "https://daily-cloudcode-pa.sandbox.googleapis.com"
)

var userAgentVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// UserAgentVersionResolver 闂傚倷绀佸﹢杈╁垝椤栫偛绀夐柟鐑樻⒐椤愪粙鏌涢…鎴濅簼濞存嚎鍊濋弻鐔兼焽閿曗偓閻忛亶鏌￠埀顒佺鐎ｎ偆鍘?User-Agent 闂傚倷鑳剁划顖炪€冮崨瀛樺亱濠电姴鍋婇懓鍨归崗鍏肩稇闁活厽顨婇弻锝夊Χ鎼达紕浼囧┑鐐叉▕閸撶喖寮婚敐澶婄厸闁稿本绮岄埛濠囨⒑缁嬭法绠查柨鏇樺灩閻ｅ嘲鈹戦崱蹇旑潔闂佸搫绉查崐鏇熺?
type UserAgentVersionResolver func(ctx context.Context) string

var (
	// defaultUserAgentVersion 闂傚倷绀侀幉锟犳偡椤栫偛鍨傞柛顐ｆ礀閻掑灚銇勯幒宥嗩樂濞存嚎鍨荤槐鎺旀嫚閹绘帗娈婚梺璇″枟婵炲﹪銆佸☉姗嗘僵闁稿繐鍚嬮楣冩⒒娴ｅ憡鍟為柣鐕傜畵閺佸啴濮€閵堝懘妫?ANTIGRAVITY_USER_AGENT_VERSION 闂傚倸鍊烽悞锕€顭垮Ο鑲╃煋闁割偅娲橀崑顏堟煕閳╁啰鈽夋潻?
	defaultUserAgentVersion  = DefaultUserAgentVersion
	userAgentVersionMu       sync.RWMutex
	userAgentVersionResolver UserAgentVersionResolver
)

// defaultClientSecret 闂傚倷绀侀幉锟犳偡椤栫偛鍨傞柛顐ｆ礀閻掑灚銇勯幒宥嗩樂濞存嚎鍨荤槐鎺旀嫚閹绘帗娈婚梺璇″枟婵炲﹪銆佸☉姗嗘僵闁稿繐鍚嬮楣冩⒒娴ｅ憡鍟為柣鐕傜畵閺佸啴濮€閵堝懘妫?ANTIGRAVITY_OAUTH_CLIENT_SECRET 闂傚倸鍊烽悞锕€顭垮Ο鑲╃煋闁割偅娲橀崑?
var defaultClientSecret = "gaoge-google-oauth-client-secret-placeholder"

func init() {
	// 婵犵數鍋涢顓熸叏閺夋嚚褰掓倻閻ｅ苯绁﹂梺绯曞墲钃辨繛鍛█閺屾盯骞囬埡浣割瀳缂傚倸绉崇欢姘潖濞差亜绠甸柟鐑樻⒒椤旀垵顪冮妶鍡樼；闁告鍟块悾鐑芥晲婢跺﹤鍞ㄥ銈嗗姂閸╁嫰寮抽銏♀拺闁告繂瀚晶閬嶆煕閹惧鎳嗛柕鍥ㄥ姍瀹曠螖娴ｈ鐝梺璇茬箳閸嬬喖宕戦幘鎰佸殨闁瑰墽绮崑锝吤归敐鍥剁劸闁抽攱妫冮弻锝夊Χ閸涱噮妫﹂悗瑙勬礃閼归箖鍩㈡惔銏″劅闁抽敮鍋撻柡瀣箻濮婂搫煤缂佹ê鈻忛梺鍛婃⒐閸ㄥ綊宕氶幒妤婃晬婵犲﹤瀚娑㈡⒑闂堟稓澧曢柟鍐差樀瀹?
	if version := NormalizeUserAgentVersion(os.Getenv(AntigravityUserAgentVersionEnv)); version != "" {
		defaultUserAgentVersion = version
	}
	// 婵犵數鍋涢顓熸叏閺夋嚚褰掓倻閻ｅ苯绁﹂梺绯曞墲钃辨繛鍛█閺屾盯骞囬埡浣割瀳缂傚倸绉崇欢姘潖濞差亜绠甸柟鐑樻⒒椤旀垵顪冮妶鍡樼；闁告鍟块悾?client_secret闂傚倷鐒︾€笛呯矙閹达箑瀚夋い鎺嗗亾闁挎洏鍨介、姗€鎮╅崘宸偓娑㈡⒑閸涘﹥瀵欓柛娑卞枤閳ь剦鍙冨娲川婵犲倻鐟查梺鑽ゅ枂閸婃牜鎹㈠☉銏犵倞妞ゆ帊绀佹禒娲⒑闂堟单鍫ュ疾濠靛牊鏆滄繛鎴欏灪閸嬶絽霉閿濆娑ч柍褜鍓氬ú鐔煎春閳?
	if secret := os.Getenv(AntigravityOAuthClientSecretEnv); secret != "" {
		defaultClientSecret = secret
	}
}

// NormalizeUserAgentVersion 闂傚倷绀侀幖顐ょ矙閸曨厽宕叉繝闈涱儐閸嬫ɑ绻涢崱妯诲暗鐎规挷绶氶幃瑙勩偊閹稿寒浠╃紓浣割槸閹碱偊鍩ユ径鎰妞ゆ牗鐭竟鏇㈡⒒?Antigravity User-Agent 闂傚倷鑳剁划顖炪€冮崨瀛樺亱濠电姴鍋婇懓鍨归崗鍏肩稇闁活厽顨婇弻锛勨偓锝夋涧娴滄劙鏌?
func NormalizeUserAgentVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || !userAgentVersionPattern.MatchString(version) {
		return ""
	}
	return version
}

// GetDefaultUserAgentVersion 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭濞差亜鍐€妞ゆ挾鍠撻崢鎰版倵楠炲灝鍔氭繛灞傚姂瀵悂宕掗悙瀵稿幐闂佺鏈喊宥夋儗濡や胶绠?闂傚倷鑳剁划顖滃垝閻樿鍨傚ù鍏肩暘閳ь剙鍊挎俊鎼佸煛娴ｈ櫣鍘繝娈垮枟閵囨盯宕戦幘缁樼厱闁挎繂鐗婇鐘裁归悪鍛暤鐎规洘甯￠幃娆戔偓娑櫱滈埀顒€鍟村鍝勑ч崶褍顬堥柣搴㈢婢瑰棝宕氶幒妤婃晬婵犲﹤瀚娑㈡⒑闂堟稓澧曟繛灞傚€濋妴鍌涚附閸涘﹦鍘搁梺鍓插亽閸嬪嫭鏅堕鐐枑闁哄瀵ч崑銉モ攽?
func GetDefaultUserAgentVersion() string {
	return defaultUserAgentVersion
}

// SetUserAgentVersionResolver 闂備浇宕垫慨宕囩矆娴ｈ娅犲ù鐘差儐閸嬵亪鏌涢埄鍏╂垶绂嶉妶澶嬬厵闂侇叏绠戦悘閬嶆煛閳ь剚绂掔€ｎ偆鍘搁梺鍛婂姂閸斿矂銆傞崘宸唵鐟滃酣銆冩繝鍌ゅ殨闁汇垹澹婇弫鍡涙煃瑜滈崜婵嬪Φ閹邦喗宕夐柕濠忕畱閺嬪倿姊洪幐搴ｇ畵婵☆偅鐩崺鈧い鎴ｆ硶閻瑧鈧娲樻繛濠囧灳閿曞倸绠抽柡鍐ㄥ亞濡茬兘姊婚崒娆戣窗闁稿鎳橀幊婵囥偅閸愨晛浜梺鍓插亝濞叉﹢宕戦妸鈺傜厱婵°倕鍟禒锕傛煕婵犲嫭鏆柡?settings 濠电姷鏁搁崑娑⑺囬銏犵鐎光偓閸曨偆鐓戦梺绯曞墲缁嬫垼绻?
func SetUserAgentVersionResolver(resolver UserAgentVersionResolver) {
	userAgentVersionMu.Lock()
	defer userAgentVersionMu.Unlock()
	userAgentVersionResolver = resolver
}

// GetUserAgentVersionForContext 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭濞差亜鍐€妞ゆ垵褰炲Ч妤呮⒑鐟欏嫬鍔ら柛鐔锋健璺柛娑樼摠閸嬶綁鏌涢妷顔煎闁轰線浜堕弻锝夘敂閸℃鏆梺璇″灡濡啫顕ｉ鍕畾鐟滃寮歌箛娑欌拺婵炶尙绮繛鍥煕閺傚潡顎楅柍?Antigravity 闂傚倷鑳剁划顖炪€冮崨瀛樺亱濠电姴鍋婇懓鍨归崗鍏肩稇闁活厽顨婇弻锛勨偓锝夋涧娴滄劙鏌?
func GetUserAgentVersionForContext(ctx context.Context) string {
	if ctx == nil {
		ctx = context.Background()
	}
	userAgentVersionMu.RLock()
	resolver := userAgentVersionResolver
	userAgentVersionMu.RUnlock()
	if resolver != nil {
		if version := NormalizeUserAgentVersion(resolver(ctx)); version != "" {
			return version
		}
	}
	return defaultUserAgentVersion
}

// BuildUserAgent 婵犵數鍋犻幓顏嗙礊閳ь剚绻涙径瀣鐎殿噮鍋婃俊鑸靛緞婵犲嫷鍞堕梻浣规偠閸庢粓宕ㄩ鐣屾Д闂傚倷鑳剁划顖炪€冮崨瀛樺亱濠电姴鍋婇懓鍨归崗鍏肩稇闁活厽顨婂娲垂椤曞懎鍓抽梺閫炲苯澧婚柛銊ョ埣瀵?User-Agent闂傚倷鐒︾€笛呯矙閹烘梻鐭欓柟鎯х摠濞呯姵淇婇妶鍛櫣閻庢艾顦伴妵鍕箣閿濆懎濮风紓浣插亾閻庯綆鍓涚壕鑲┾偓鍏夊亾闁逞屽墴瀹曟洟濡舵径濠傜€梺闈浥堥弲婊埶夐崼銉︾厽闁归偊鍓﹂崵鐔兼煏閸℃韬柡宀嬬節瀹曟﹢顢橀悢鎭掆偓鎰攽閻愭彃绾ч柣妤冨█瀵宕ㄩ弶鎴犲姦濡炪倖甯婄欢鈥炽€掗懜鍏哥箚妞ゆ牗渚楅崕銉╂煕濮橆剚鍠橀柡灞剧⊕缁绘繈宕熼棃娑樺Υ闂?
func BuildUserAgent(version string) string {
	if normalized := NormalizeUserAgentVersion(version); normalized != "" {
		return fmt.Sprintf("antigravity/%s windows/amd64", normalized)
	}
	return fmt.Sprintf("antigravity/%s windows/amd64", defaultUserAgentVersion)
}

// GetUserAgentForContext 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭濞差亜鍐€妞ゆ垵褰炲Ч妤呮⒑鐟欏嫬鍔ら柛鐔锋健璺柛娑樼摠閸嬶綁鏌涢妷顔煎闁轰線浜堕弻锝夘敂閸℃鏆梺璇″灡濡啫顕ｉ鍕畾鐟滃寮歌箛娑欌拺婵炶尙绮繛鍥煕閺傚潡顎楅柍?User-Agent闂?
func GetUserAgentForContext(ctx context.Context) string {
	return BuildUserAgent(GetUserAgentVersionForContext(ctx))
}

// GetUserAgent 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭濞差亜鍐€妞ゆ垵褰炲Ч妤呮⒑鐟欏嫬鍔ら柛鐔锋健璺柛娑樼摠閳锋垿鏌℃径搴㈢《閼叉牠姊哄Ч鍥р偓鏇炍涘┑瀣畾?User-Agent闂?
func GetUserAgent() string {
	return GetUserAgentForContext(context.Background())
}

func getClientSecret() (string, error) {
	if v := strings.TrimSpace(defaultClientSecret); v != "" {
		return v, nil
	}
	return "", infraerrors.Newf(http.StatusBadRequest, "ANTIGRAVITY_OAUTH_CLIENT_SECRET_MISSING", "missing antigravity oauth client_secret; set %s", AntigravityOAuthClientSecretEnv)
}

// BaseURLs 闂備浇顕у锕傦綖婢舵劖鍎楁い鏂垮⒔娑?Antigravity API 缂傚倸鍊烽悞锕€螞韫囨稑鍨傞柟鎯版绾惧鏌ㄥ┑鍡╂Ч闁哄拋鍓熼弻娑㈩敃閿濆洨鐓傞梺?Antigravity-Manager 婵犵數鍎戠徊钘壝洪敂鐐床闁告劦浜栭崑鎾诲垂椤愶絺鎷圭紓浣割儏椤︾敻宕洪埀顒併亜閹烘垵顏柛銈咃躬瀵爼宕煎┑鍡忔寖缂備讲鍋?
var BaseURLs = []string{
	antigravityProdBaseURL,  // prod (婵犵數鍋炲娆撳触鐎ｎ喗鏅梻?
	antigravityDailyBaseURL, // daily sandbox (婵犵數濮伴崹鐓庘枖濞戞◤娲Ω閳轰焦鐎?
}

// BaseURL 婵犳鍠楃敮妤冪矙閹烘せ鈧箓宕奸妷顔芥櫍?URL闂傚倷鐒︾€笛呯矙閹达附鍋嬮柛娑卞灡瀹曞弶鎱ㄥΟ鎸庣【缂佺姵婢橀湁闁挎繂鎳忛崯鐐烘煕婵犲偆鐓奸柡灞剧☉椤繈顢楅崟纰樺亾濡ゅ懏鐓曟慨姗嗗墻閸庢垿鏌嶈閸撴瑥锕㈤崡鐐╂瀺闁靛繈鍨洪～?
var BaseURL = BaseURLs[0]

// ForwardBaseURLs 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭?API 闂備礁鎼ˇ閬嶅磿閹版澘绀堟繛鍡樺竾娴滃綊鏌涘畝鈧崑鐐哄磻閵娾晜鐓忓┑鐐戝啫顏柡?URL 婵犵绱曢崑鎴﹀磹閺囥垹绠规い鎰╁€涙慨铏亜閹惧崬鐏柡鍜佸墴閺屾稖顦茬弧鎼僱y 婵犵數鍋炲娆撳触鐎ｎ喗鏅梻浣告啞钃遍柣鈺婂灦瀵?
func ForwardBaseURLs() []string {
	if len(BaseURLs) == 0 {
		return nil
	}
	urls := append([]string(nil), BaseURLs...)
	dailyIndex := -1
	for i, url := range urls {
		if url == antigravityDailyBaseURL {
			dailyIndex = i
			break
		}
	}
	if dailyIndex <= 0 {
		return urls
	}
	reordered := make([]string, 0, len(urls))
	reordered = append(reordered, urls[dailyIndex])
	for i, url := range urls {
		if i == dailyIndex {
			continue
		}
		reordered = append(reordered, url)
	}
	return reordered
}

// URLAvailability tracks temporary endpoint failures.
type URLAvailability struct {
	mu          sync.RWMutex
	unavailable map[string]time.Time
	ttl         time.Duration
	lastSuccess string
}
// DefaultURLAvailability 闂傚倷鑳堕…鍫㈡崲閸儱绀夌€光偓閸曨剙鍓?URL 闂傚倷绀侀幉锟犳偡椤栫偛鍨傞柟鎯版閺嬩線鏌曢崼婵愭Ц缂佺姵鍨归幉姝岀疀濞戣鲸鏅ｉ梺褰掓？閻掞箓宕戦悩缁樼厱闁斥晛鍠氬▓鏇㈡煟?
var DefaultURLAvailability = NewURLAvailability(URLAvailabilityTTL)

// NewURLAvailability 闂傚倷绀侀幉锛勬暜濡ゅ啰鐭欓柟瀵稿Х绾?URL 闂傚倷绀侀幉锟犳偡椤栫偛鍨傞柟鎯版閺嬩線鏌曢崼婵愭Ц缂佺姵鍨归幉姝岀疀濞戣鲸鏅ｉ梺褰掓？閻掞箓宕戦悩缁樼厱闁斥晛鍠氬▓鏇㈡煟?
func NewURLAvailability(ttl time.Duration) *URLAvailability {
	return &URLAvailability{
		unavailable: make(map[string]time.Time),
		ttl:         ttl,
	}
}

// MarkUnavailable 闂傚倷绀侀幖顐ょ矓閺夋嚚娲煛閸滀焦鏅?URL 婵犵數鍋為崹鍫曞箰閹间焦鍋ら柕濞垮労濞撳鏌涚仦鍓с€掗柍缁樻⒒閳ь剙绠嶉崕閬嶅箠鎼达絾濯奸柡灞诲劜閻?
func (u *URLAvailability) MarkUnavailable(url string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.unavailable[url] = time.Now().Add(u.ttl)
}

// MarkSuccess 闂傚倷绀侀幖顐ょ矓閺夋嚚娲煛閸滀焦鏅?URL 闂備浇宕垫慨鏉懨洪銏犵哗闂侇剙绉甸崕鎴澝归崗鍏肩稇缂佺媴缍侀弻鐔兼焽閿曗偓婢ь喗銇勯銈呪枅闁哄被鍔岄埥澶娢熼崹顕呬純闂備焦鐪归崝濠囧垂閽樺鏆︽俊銈呮噹缁犳岸姊洪銊х暠闁哄拋鍘界换娑㈠箣閻愭潙纾╁┑鈩冦仠閸斿骸危閹伴偊鏁囬柕蹇曞Х椤ρ囨⒑閸撴彃浜濈紒顔肩焸閸┿垼绠涘☉娆戝幗?
func (u *URLAvailability) MarkSuccess(url string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.lastSuccess = url
	// 闂傚倷鑳堕幊鎾绘偤閵娾晛绀夐柡鍥╁枑閸欏繑绻涢幋娆忕仼閻熸瑱绠撻獮鏍庨鈧悘顔姐亜韫囨梹鍊愭慨濠傤煼瀹曠喖鍩℃担鎻掍壕婵炴垯鍩勯弫?URL 闂傚倷鐒﹂惇褰掑礉瀹€鈧埀顒佸嚬娴滅偟鍒掗鐔风窞闁归偊鍓涢悡鎴濐渻閵堝棛澧紒顔肩Ч瀵疇绠涘☉娆戝幐閻庡箍鍎遍幊蹇曠矚閹稿簺浜?
	delete(u.unavailable, url)
}

// IsAvailable 濠电姷顣藉Σ鍛村磻閳ь剟鏌涚€ｎ偅宕岄柡?URL 闂傚倷绀侀幖顐も偓姘卞厴瀹曡瀵奸弶鎴犵暰婵炴挻鍩冮崑鎾垛偓瑙勬穿缂嶄線銆佸☉姗嗙叆闁告洦鍓氶?
func (u *URLAvailability) IsAvailable(url string) bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	expiry, exists := u.unavailable[url]
	if !exists {
		return true
	}
	return time.Now().After(expiry)
}

// GetAvailableURLs 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭濞差亜鍐€妞ゆ挾鍋熼悡鎴濐渻閵堝棛澧紒顔肩Ч瀵疇绠涘☉娆戝幗?URL 闂傚倷绀侀幉锛勬暜濡ゅ懌鈧啯寰勯幇顑?// 闂傚倷绀侀幖顐︽偋閸愵喖纾婚柟鎹愵嚙缁狙囨煃閸濆嫬鈧悂鎮橀敃鍌涚厱闁靛绠戦崫铏光偓瑙勬礃缁诲牓骞冮姀銈嗗€绘俊顖氬悑濞?URL 婵犵數鍋炲娆撳触鐎ｎ喗鏅梻浣告啞钃遍柣鈺婂灦瀵偄顓奸崶锔藉媰闂佽姤锚椤︻垳娆㈤鐔虹閻庣數顭堟牎闂佺粯顨嗛〃鍫ュ焵椤掆偓閻忔氨鍒掑澶嗏偓鏃堝焵椤掍椒绻嗘い鏍ㄤ緱閸庛儵鏌涘顒佹悙妞ゎ亜鍟存俊鍫曞川椤旇棄鍓电紓?
func (u *URLAvailability) GetAvailableURLs() []string {
	return u.GetAvailableURLsWithBase(BaseURLs)
}

// GetAvailableURLsWithBase 闂備礁鎼ˇ顐﹀疾濠婂牆钃熼柕濞垮剭濞差亜鍐€妞ゆ挾鍋熼悡鎴濐渻閵堝棛澧紒顔肩Ч瀵疇绠涘☉娆戝幗?URL 闂傚倷绀侀幉锛勬暜濡ゅ懌鈧啯寰勯幇顑┿儵鏌涢幇闈涙灍闁哄拋鍓熼弻娑㈩敃閿濆洨鐓€闂佸疇顕х换姗€寮婚敐澶涚稏妞ゆ巻鍋撳┑顔瑰亾闂備焦鎮堕崐鏍垝鎼淬劌鐒垫い鎺戝枤濞兼劙鏌ｉ褍鏋ょ紒顔碱煼閹垽骞栭悙鎵泿闂備礁鎲＄换鍌溾偓姘煎枤閸犲﹤顓兼径瀣弳?// 闂傚倷绀侀幖顐︽偋閸愵喖纾婚柟鎹愵嚙缁狙囨煃閸濆嫬鈧悂鎮橀敃鍌涚厱闁靛绠戦崫铏光偓瑙勬礃缁诲牓骞冮姀銈嗗€绘俊顖氬悑濞?URL 婵犵數鍋炲娆撳触鐎ｎ喗鏅梻浣告啞钃遍柣鈺婂灦瀵偄顓奸崶锔藉媰闂佽姤锚椤︻垳娆㈤鐔虹閻庣數顭堟牎闂佺粯顨嗛〃鍫ュ焵椤掆偓閻忔氨鍒掑鍥ｅ亾闂堟稏鍋㈤柟顔规櫊楠炴捇骞掗幋鐐剁发婵犵绱曢崑鎴﹀磹閺囥垹绠规い鎰╁€涙慨?
func (u *URLAvailability) GetAvailableURLsWithBase(baseURLs []string) []string {
	u.mu.RLock()
	defer u.mu.RUnlock()

	now := time.Now()
	result := make([]string, 0, len(baseURLs))

	// 婵犵數濮烽。浠嬪焵椤掆偓閸熷潡鍩€椤掆偓缂嶅﹪骞冨Ο璇茬窞闁归偊鍓欓悵妯荤節閵忥絾纭炬い鎴濇缁傛帒顫滈埀顒€顕ｉ崼鏇炵厸闁稿本姘ㄦ禒鎼佹⒑閹肩偛濡界€光偓缁嬭法鏆︽繝闈涱儐閸嬪嫰鏌ｉ幋鐑嗙劷闁?URL 婵犵數鍋為崹鍫曞箰婵犳艾钃熼柕濞垮妸娴滃綊鏌＄仦璇插姕闁稿鍔嶉妵鍕敇閻旈顑傜紓浣插亾濠㈣埖鍔栭悡娑㈡煃瑜滈崜鐔肩嵁閸℃凹妲奸梻濠庡墻閸撶喖寮婚敓鐘查唶婵﹩鍏涙竟鏇㈡⒒娴ｅ憡鍟為柟绋挎憸缁棃鎮烽柇锔惧姺?
	if u.lastSuccess != "" {
		found := false
		for _, url := range baseURLs {
			if url == u.lastSuccess {
				found = true
				break
			}
		}
		if found {
			expiry, exists := u.unavailable[u.lastSuccess]
			if !exists || now.After(expiry) {
				result = append(result, u.lastSuccess)
			}
		}
	}

	// 濠电姷鏁搁崕鎴犵礊閳ь剚銇勯弴鍡楀閸欏繘鏌ｉ幇顒佹儓缂佲偓閸岀偞鐓涢柛顐犲灪閺嗏晠姊洪幆褍鈧潡寮诲☉妯锋瀻婵☆垵娅ｆ禒顖炴⒑鏉炰即妾烽柛濠冪箞楠?URL闂傚倷鐒︾€笛呯矙閹达附鍋嬪┑鐘插閸嬫捇宕归銈囩厑闁诲酣娼ч妶鎼佸箖閳哄懎绠甸柟鐑樺灦鏉堝牆鈹戦敍鍕杭闁稿﹥娲熼獮鏍敃閵堝牆小濡炪倖鎸堕崹娲疾?
	for _, url := range baseURLs {
		// 闂備浇宕垫慨鎾箹椤愶附鍋柛銉㈡櫆瀹曟煡鏌涢幇鐢靛帥闁哥喎鎳橀弻娑樷枎韫囷絾笑濠电偛鍚嬮崹鍧楀蓟濞戙垹绫嶉柛灞绢嚧閵夛妇绠?lastSuccess
		if url == u.lastSuccess {
			continue
		}
		expiry, exists := u.unavailable[url]
		if !exists || now.After(expiry) {
			result = append(result, url)
		}
	}
	return result
}

// OAuthSession 婵犵數鍎戠徊钘壝洪敂鐐床闁稿瞼鍋為崑?OAuth 闂傚倷鑳堕幊鎾诲箹椤愶附鍋嬪┑鐘插亞閻掍粙鏌涢锝囪穿閻熸瑥瀚刊鎾偠濞戞帒澧查柣鎾跺█濮婂搫效閸パ冾瀳闁诲孩鍑规禍婵嗩嚗閸曨偒鍚嬪璺猴功閻嫰姊洪崨濠冨矮缂佲偓娴ｈ櫣鎳呴梻?
type OAuthSession struct {
	State        string    `json:"state"`
	CodeVerifier string    `json:"code_verifier"`
	ProxyURL     string    `json:"proxy_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// SessionStore OAuth session 闂備浇顕х€涒晝绮欓幒妤佹櫔闂?
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*OAuthSession
	stopCh   chan struct{}
}

func NewSessionStore() *SessionStore {
	store := &SessionStore{
		sessions: make(map[string]*OAuthSession),
		stopCh:   make(chan struct{}),
	}
	go store.cleanup()
	return store
}

func (s *SessionStore) Set(sessionID string, session *OAuthSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
}

func (s *SessionStore) Get(sessionID string) (*OAuthSession, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if time.Since(session.CreatedAt) > SessionTTL {
		return nil, false
	}
	return session, true
}

func (s *SessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *SessionStore) Stop() {
	select {
	case <-s.stopCh:
		return
	default:
		close(s.stopCh)
	}
}

func (s *SessionStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, session := range s.sessions {
				if time.Since(session.CreatedAt) > SessionTTL {
					delete(s.sessions, id)
				}
			}
			s.mu.Unlock()
		}
	}
}

func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func GenerateState() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64URLEncode(bytes), nil
}

func GenerateSessionID() (string, error) {
	bytes, err := GenerateRandomBytes(16)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateCodeVerifier() (string, error) {
	bytes, err := GenerateRandomBytes(32)
	if err != nil {
		return "", err
	}
	return base64URLEncode(bytes), nil
}

func GenerateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64URLEncode(hash[:])
}

func base64URLEncode(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// BuildAuthorizationURL 闂傚倷绀侀幖顐︻敄閸涱垪鍋撳鐓庡缂?Google OAuth 闂傚倷鑳堕幊鎾诲箹椤愶附鍋嬪┑鐘插亞閻?URL
func BuildAuthorizationURL(state, codeChallenge string) string {
	params := url.Values{}
	params.Set("client_id", ClientID)
	params.Set("redirect_uri", RedirectURI)
	params.Set("response_type", "code")
	params.Set("scope", Scopes)
	params.Set("state", state)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	params.Set("access_type", "offline")
	params.Set("prompt", "consent")
	params.Set("include_granted_scopes", "true")

	return fmt.Sprintf("%s?%s", AuthorizeURL, params.Encode())
}
