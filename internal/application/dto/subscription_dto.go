package dto

import (
	"time"

	"github.com/google/uuid"
)

type SubscriptionModuleInput struct {
	ItemCode string `json:"item_code"`
	Quantity int    `json:"quantity"`
}

type CreateSubscriptionRequest struct {
	OrganizationID uuid.UUID                 `json:"organization_id"`
	Modules        []SubscriptionModuleInput `json:"modules"`
}

type CreateSubscriptionResponse struct {
	SubscriptionID         string `json:"subscription_id"`
	RazorpaySubscriptionID string `json:"razorpay_subscription_id"`
	PaymentLink            string `json:"payment_link"`
}

type UpgradeSubscriptionRequest struct {
	OrganizationID uuid.UUID                 `json:"organization_id"`
	Modules        []SubscriptionModuleInput `json:"modules"`
}

type UpgradeSubscriptionResponse struct {
	RazorpayOrderID string  `json:"razorpay_order_id"`
	ProratedAmount  float64 `json:"prorated_amount"`
	TaxAmount       float64 `json:"tax_amount"`
	TotalAmount     float64 `json:"total_amount"`
}

type DowngradeSubscriptionRequest struct {
	OrganizationID uuid.UUID                 `json:"organization_id"`
	Modules        []SubscriptionModuleInput `json:"modules"`
}

type SubscriptionItemResponse struct {
	ID              string  `json:"id"`
	ItemCode        string  `json:"item_code"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	BillingType     string  `json:"billing_type"`
	UnitPrice       float64 `json:"unit_price"`
	Quantity        int     `json:"quantity"`
	Amount          float64 `json:"amount"`
	Status          string  `json:"status"`
	PendingQuantity *int    `json:"pending_quantity,omitempty"`
}

type SubscriptionResponse struct {
	ID                     string                     `json:"id"`
	OrganizationID         string                     `json:"organization_id"`
	RazorpaySubscriptionID string                     `json:"razorpay_subscription_id,omitempty"`
	RazorpayPlanID         string                     `json:"razorpay_plan_id"`
	Status                 string                     `json:"status"`
	CurrentPeriodStart     time.Time                  `json:"current_period_start"`
	CurrentPeriodEnd       time.Time                  `json:"current_period_end"`
	RenewalDate            time.Time                  `json:"renewal_date"`
	RecurringAmount        float64                    `json:"recurring_amount"`
	TaxPercentage          float64                    `json:"tax_percentage"`
	TotalRecurringAmount   float64                    `json:"total_recurring_amount"`
	Currency               string                     `json:"currency"`
	Items                  []SubscriptionItemResponse `json:"items"`
}
