package payment

import (
	"fmt"
	"strings"

	"github/master_go_cqrs/services/payment/internal/adapter/gateway/simulator"
	stripeadapter "github/master_go_cqrs/services/payment/internal/adapter/gateway/stripe"
	"github/master_go_cqrs/services/payment/internal/application/port"
	"github/master_go_cqrs/services/payment/internal/config"
)

func newPaymentProvider(cfg *config.Config) (port.PaymentProvider, error) {
	kind := strings.ToLower(strings.TrimSpace(cfg.Provider.Kind))
	if kind == "" {
		kind = "simulator"
	}
	switch kind {
	case "simulator":
		if strings.EqualFold(strings.TrimSpace(cfg.Logger.Mode), "production") {
			return nil, fmt.Errorf("payment provider simulator is forbidden in production")
		}
		return simulator.NewPaymentProvider(), nil
	case "stripe":
		return stripeadapter.NewProvider(stripeadapter.Config{
			SecretKey: cfg.Provider.Stripe.SecretKey, WebhookSecrets: cfg.Provider.Stripe.WebhookSecrets,
			BaseURL: cfg.Provider.Stripe.BaseURL, HTTPTimeout: cfg.Provider.Stripe.HTTPTimeout,
			WebhookTolerance: cfg.Provider.Stripe.WebhookTolerance,
		})
	default:
		return nil, fmt.Errorf("unsupported payment provider %q", kind)
	}
}
