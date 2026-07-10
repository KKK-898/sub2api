package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/payment"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	paymentBackupModeMain   = "main"
	paymentBackupModeBackup = "backup"
)

type softwareAdminOnlinePaymentConfig struct {
	Enabled          bool   `json:"enabled"`
	PaymentMode      string `json:"paymentMode"`
	PaymentModeSnake string `json:"payment_mode"`
}

type softwareAdminConfigResponse struct {
	Success bool                             `json:"success"`
	Data    softwareAdminOnlinePaymentConfig `json:"data"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (s *PaymentService) shouldUseBackupPaymentBridge(ctx context.Context, req CreateOrderRequest) (bool, error) {
	if req.OrderType != payment.OrderTypeBalance {
		return false, nil
	}
	if payment.GetBasePaymentType(strings.TrimSpace(req.PaymentType)) != payment.TypeAlipay {
		return false, nil
	}

	cfg, err := s.fetchSoftwareAdminOnlinePaymentConfig(ctx)
	if err != nil {
		slog.Warn("[PaymentService] backup payment mode config unavailable; falling back to main payment", "error", err)
		return false, nil
	}
	return cfg.Enabled && normalizeBackupPaymentMode(cfg) == paymentBackupModeBackup, nil
}

func normalizeBackupPaymentMode(cfg softwareAdminOnlinePaymentConfig) string {
	mode := strings.ToLower(strings.TrimSpace(cfg.PaymentMode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(cfg.PaymentModeSnake))
	}
	if mode == paymentBackupModeBackup {
		return paymentBackupModeBackup
	}
	return paymentBackupModeMain
}

func (s *PaymentService) fetchSoftwareAdminOnlinePaymentConfig(ctx context.Context) (softwareAdminOnlinePaymentConfig, error) {
	endpoint := strings.TrimSpace(os.Getenv("SOFTWARE_ADMIN_ONLINE_PAYMENT_CONFIG_URL"))
	if endpoint == "" {
		base := strings.TrimRight(strings.TrimSpace(os.Getenv("SOFTWARE_ADMIN_API_BASE_URL")), "/")
		if base == "" {
			base = strings.TrimRight(strings.TrimSpace(os.Getenv("VITE_GAOGE_SOFTWARE_ADMIN_API_BASE_URL")), "/")
		}
		if base == "" {
			if adminURL := derivedPublicURL("ADMIN_PUBLIC_URL", "ADMIN_PUBLIC_DOMAIN", "ruanjianhoutai"); adminURL != "" {
				base = strings.TrimRight(adminURL, "/") + "/api"
			}
		}
		if base != "" {
			endpoint = base + "/v1/desktop/online-payment/config"
		} else {
			return softwareAdminOnlinePaymentConfig{}, infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", "software_admin_config_url_missing")
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return softwareAdminOnlinePaymentConfig{}, infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", "backup_payment_config_error").WithCause(err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return softwareAdminOnlinePaymentConfig{}, infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", "backup_payment_config_error").WithCause(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return softwareAdminOnlinePaymentConfig{}, infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", "backup_payment_config_error").WithCause(err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return softwareAdminOnlinePaymentConfig{}, infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", "backup_payment_config_error").
			WithMetadata(map[string]string{"status": strconv.Itoa(resp.StatusCode)})
	}

	var payload softwareAdminConfigResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return softwareAdminOnlinePaymentConfig{}, infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", "backup_payment_config_error").WithCause(err)
	}
	if !payload.Success {
		msg := strings.TrimSpace(payload.Error.Message)
		if msg == "" {
			msg = "backup_payment_config_error"
		}
		return softwareAdminOnlinePaymentConfig{}, infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", msg)
	}

	return payload.Data, nil
}

func (s *PaymentService) invokeBackupPaymentBridge(ctx context.Context, order *dbent.PaymentOrder, req CreateOrderRequest, payAmount float64) (*CreateOrderResponse, error) {
	payURL, err := buildBackupPaymentBridgeURL(order, req, payAmount)
	if err != nil {
		return nil, err
	}

	_, err = s.entClient.PaymentOrder.UpdateOneID(order.ID).
		SetPayURL(payURL).
		SetProviderKey("backup_bridge").
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update order with backup bridge url: %w", err)
	}

	s.writeAuditLog(ctx, order.ID, "ORDER_CREATED", fmt.Sprintf("user:%d", req.UserID), map[string]any{
		"paymentAmount":  req.Amount,
		"creditedAmount": order.Amount,
		"payAmount":      order.PayAmount,
		"paymentType":    req.PaymentType,
		"orderType":      req.OrderType,
		"paymentMode":    "backup_bridge",
	})

	return &CreateOrderResponse{
		OrderID:     order.ID,
		Amount:      order.Amount,
		PayAmount:   payAmount,
		FeeRate:     order.FeeRate,
		Status:      order.Status,
		ResultType:  payment.CreatePaymentResultOrderCreated,
		PaymentType: req.PaymentType,
		OutTradeNo:  order.OutTradeNo,
		PayURL:      payURL,
		ExpiresAt:   order.ExpiresAt,
		PaymentMode: "backup_bridge",
	}, nil
}

func buildBackupPaymentBridgeURL(order *dbent.PaymentOrder, req CreateOrderRequest, payAmount float64) (string, error) {
	baseURL := strings.TrimSpace(os.Getenv("PAYMENT_BACKUP_BRIDGE_URL"))
	if baseURL == "" {
		if payURL := derivedPublicURL("PAY_PUBLIC_URL", "PAY_PUBLIC_DOMAIN", "pay"); payURL != "" {
			baseURL = strings.TrimRight(payURL, "/") + "/pay"
		}
	}
	u, err := url.Parse(baseURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", infraerrors.ServiceUnavailable("BACKUP_PAYMENT_CONFIG_ERROR", "backup_payment_bridge_url_invalid")
	}

	planID := backupPaymentPlanID(req.Amount)
	q := u.Query()
	q.Set("order_id", strconv.FormatInt(order.ID, 10))
	q.Set("out_trade_no", order.OutTradeNo)
	q.Set("amount", payment.FormatAmountForCurrency(payAmount, payment.DefaultPaymentCurrency))
	q.Set("plan_id", planID)
	q.Set("user_id", strconv.FormatInt(order.UserID, 10))
	q.Set("ts", strconv.FormatInt(time.Now().Unix(), 10))
	q.Set("sign", signBackupPaymentBridgeParams(q))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func derivedPublicURL(urlEnv string, domainEnv string, prefix string) string {
	if value := strings.TrimRight(strings.TrimSpace(os.Getenv(urlEnv)), "/"); value != "" {
		return value
	}

	domain := normalizePublicDomain(os.Getenv(domainEnv))
	if domain == "" {
		root := normalizePublicDomain(os.Getenv("ROOT_DOMAIN"))
		if root == "" {
			root = "localhost"
		}
		if root == "localhost" {
			domain = prefix + ".localhost"
		} else {
			domain = prefix + "." + root
		}
	}
	if domain == "" {
		return ""
	}

	scheme := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(os.Getenv("PUBLIC_SCHEME"))), ":")
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	return scheme + "://" + domain
}

func normalizePublicDomain(value string) string {
	domain := strings.TrimSpace(value)
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	if slash := strings.Index(domain, "/"); slash >= 0 {
		domain = domain[:slash]
	}
	domain = strings.TrimSuffix(domain, ":443")
	domain = strings.TrimSuffix(domain, ":80")
	domain = strings.Trim(domain, ".")
	return strings.ToLower(domain)
}

func backupPaymentPlanID(amount float64) string {
	cents := int64(amount*100 + 0.5)
	switch cents {
	case 2000:
		return "recharge-50"
	case 5000:
		return "recharge-100"
	case 10000:
		return "recharge-200"
	case 20000:
		return "recharge-500"
	case 50000:
		return "recharge-800"
	case 100000:
		return "recharge-1000"
	default:
		return "recharge-" + strconv.FormatInt(cents, 10)
	}
}

func signBackupPaymentBridgeParams(q url.Values) string {
	secret := strings.TrimSpace(os.Getenv("PAYMENT_BACKUP_BRIDGE_SECRET"))
	if secret == "" {
		secret = "gaoge-backup-payment-bridge"
	}
	keys := []string{"order_id", "out_trade_no", "amount", "plan_id", "user_id", "ts"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+q.Get(key))
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join(parts, "&")))
	return hex.EncodeToString(mac.Sum(nil))
}
