package external

import (
	"context"
)

type RazorpayClient interface {
	CreatePlan(ctx context.Context, name string, amount float64, currency string) (string, error)
	CreateSubscription(ctx context.Context, planID string, customerEmail string, totalCycles int) (string, string, error) // returns subID, paymentLink
	UpdateSubscriptionPlan(ctx context.Context, subID string, newPlanID string, scheduleChangeAt string) error
	CreateOrder(ctx context.Context, amount float64, currency string, receipt string) (string, error)
	VerifyWebhookSignature(body []byte, signature string, secret string) bool
}
