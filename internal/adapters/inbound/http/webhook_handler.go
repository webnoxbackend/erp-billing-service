package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type WebhookHandler struct {
	subService     *application.SubscriptionService
	subRepo        *application.SubscriptionService // Wait, we can use subService directly for domain logic or repository injection
	db             *gorm.DB
	eventPublisher domain.EventPublisher
	razorpayClient domain.InvoiceRepository // Wait, let's just use raw services or repository interfaces
}

// Let's create a simpler WebhookHandler that references subService and database.
type RazorpayWebhookHandler struct {
	subService      *application.SubscriptionService
	db              *gorm.DB
	razorpayClient  *application.SubscriptionService // We'll just define the specific fields we need
	eventPublisher  domain.EventPublisher
	webhookSecret   string
	invoiceRepo     domain.InvoiceRepository
	paymentRepo     domain.PaymentRepository
	subRepo         domain.InvoiceRepository // Wait, let's pass repos and client directly!
}

type razorpayWebhookPayload struct {
	Event   string `json:"event"`
	Payload struct {
		Subscription struct {
			Entity struct {
				ID           string `json:"id"`
				PlanID       string `json:"plan_id"`
				Status       string `json:"status"`
				CurrentStart int64  `json:"current_start"`
				CurrentEnd   int64  `json:"current_end"`
			} `json:"entity"`
		} `json:"subscription"`
		Order struct {
			Entity struct {
				ID     string `json:"id"`
				Amount int64  `json:"amount"`
				Status string `json:"status"`
			} `json:"entity"`
		} `json:"order"`
	} `json:"payload"`
}

func NewRazorpayWebhookHandler(
	subService *application.SubscriptionService,
	db *gorm.DB,
	eventPublisher domain.EventPublisher,
	invoiceRepo domain.InvoiceRepository,
	paymentRepo domain.PaymentRepository,
) *RazorpayWebhookHandler {
	secret := os.Getenv("RAZORPAY_WEBHOOK_SECRET")
	if secret == "" {
		secret = "default_webhook_secret"
	}
	return &RazorpayWebhookHandler{
		subService:     subService,
		db:             db,
		eventPublisher: eventPublisher,
		webhookSecret:  secret,
		invoiceRepo:    invoiceRepo,
		paymentRepo:    paymentRepo,
	}
}

func (h *RazorpayWebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Verify webhook signature
	signature := r.Header.Get("X-Razorpay-Signature")
	if signature == "" {
		log.Println("[Razorpay Webhook] Missing X-Razorpay-Signature header")
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	// Signature verification logic (re-implemented here using the secret)
	// We can use standard library hmac comparison
	mac := hmacSHA256(body, h.webhookSecret)
	if mac != signature {
		log.Println("[Razorpay Webhook] Invalid webhook signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	var payload razorpayWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[Razorpay Webhook] Failed to unmarshal body: %v", err)
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	log.Printf("[Razorpay Webhook] Received event: %s", payload.Event)

	ctx := r.Context()
	switch payload.Event {
	case "subscription.activated":
		err = h.handleSubscriptionActivated(ctx, payload)
	case "subscription.charged":
		err = h.handleSubscriptionCharged(ctx, payload)
	case "order.paid":
		err = h.handleOrderPaid(ctx, payload)
	case "subscription.cancelled":
		err = h.handleSubscriptionCancelled(ctx, payload)
	default:
		log.Printf("[Razorpay Webhook] Unhandled event type: %s", payload.Event)
	}

	if err != nil {
		log.Printf("[Razorpay Webhook] Error handling event %s: %v", payload.Event, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func hmacSHA256(body []byte, secret string) string {
	h := sha256.New
	mac := hmac.New(h, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (h *RazorpayWebhookHandler) handleSubscriptionActivated(ctx context.Context, payload razorpayWebhookPayload) error {
	var sub domain.Subscription
	if err := h.db.WithContext(ctx).Preload("Items").First(&sub, "razorpay_subscription_id = ?", payload.Payload.Subscription.Entity.ID).Error; err != nil {
		return fmt.Errorf("subscription not found locally: %w", err)
	}

	oldBytes, _ := json.Marshal(sub)

	// Update subscription details
	sub.Status = domain.SubscriptionStatusActive
	sub.CurrentPeriodStart = time.Unix(payload.Payload.Subscription.Entity.CurrentStart, 0).UTC()
	sub.CurrentPeriodEnd = time.Unix(payload.Payload.Subscription.Entity.CurrentEnd, 0).UTC()
	sub.RenewalDate = sub.CurrentPeriodEnd
	sub.UpdatedAt = time.Now().UTC()

	if err := h.db.WithContext(ctx).Save(&sub).Error; err != nil {
		return err
	}

	// Generate Paid Invoice & Payment Record
	if err := h.createInvoiceAndPayment(ctx, &sub, "AutoPay Subscription Activation Invoice"); err != nil {
		log.Printf("Warning: failed to generate activation invoice: %v", err)
	}

	// Write Audit Log
	newBytes, _ := json.Marshal(sub)
	_ = h.db.WithContext(ctx).Create(&domain.SubscriptionAuditLog{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Action:         "activate",
		OldDetails:     string(oldBytes),
		NewDetails:     string(newBytes),
		PerformedBy:    "Razorpay Webhook",
		CreatedAt:      time.Now().UTC(),
	})

	// Emit Event to Kafka to enable features immediately
	return h.publishEvent(ctx, &sub, shared_events.OrganizationSubscribed)
}

func (h *RazorpayWebhookHandler) handleSubscriptionCharged(ctx context.Context, payload razorpayWebhookPayload) error {
	var sub domain.Subscription
	if err := h.db.WithContext(ctx).Preload("Items").First(&sub, "razorpay_subscription_id = ?", payload.Payload.Subscription.Entity.ID).Error; err != nil {
		return fmt.Errorf("subscription not found locally: %w", err)
	}

	oldBytes, _ := json.Marshal(sub)

	// Apply any pending downgrades before processing charged renewal
	err := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range sub.Items {
			if sub.Items[i].Status == domain.SubscriptionItemPendingRemoval {
				if err := tx.Delete(&sub.Items[i]).Error; err != nil {
					return err
				}
			} else if sub.Items[i].PendingQuantity != nil {
				sub.Items[i].Quantity = *sub.Items[i].PendingQuantity
				sub.Items[i].PendingQuantity = nil
				sub.Items[i].Amount = sub.Items[i].UnitPrice * float64(sub.Items[i].Quantity)
				if err := tx.Save(&sub.Items[i]).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// Reload updated items
	if err := h.db.WithContext(ctx).Preload("Items").First(&sub, "id = ?", sub.ID).Error; err != nil {
		return err
	}

	sub.RecalculateTotals()
	sub.CurrentPeriodStart = time.Unix(payload.Payload.Subscription.Entity.CurrentStart, 0).UTC()
	sub.CurrentPeriodEnd = time.Unix(payload.Payload.Subscription.Entity.CurrentEnd, 0).UTC()
	sub.RenewalDate = sub.CurrentPeriodEnd
	sub.UpdatedAt = time.Now().UTC()

	if err := h.db.WithContext(ctx).Save(&sub).Error; err != nil {
		return err
	}

	// Generate Paid Invoice & Payment Record
	if err := h.createInvoiceAndPayment(ctx, &sub, "AutoPay Subscription Renewal Invoice"); err != nil {
		log.Printf("Warning: failed to generate renewal invoice: %v", err)
	}

	// Write Audit Log
	newBytes, _ := json.Marshal(sub)
	_ = h.db.WithContext(ctx).Create(&domain.SubscriptionAuditLog{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Action:         "renew",
		OldDetails:     string(oldBytes),
		NewDetails:     string(newBytes),
		PerformedBy:    "Razorpay Webhook",
		CreatedAt:      time.Now().UTC(),
	})

	// Emit Event to Kafka
	return h.publishEvent(ctx, &sub, shared_events.OrganizationSubscribed)
}

func (h *RazorpayWebhookHandler) handleOrderPaid(ctx context.Context, payload razorpayWebhookPayload) error {
	var upgrade domain.SubscriptionUpgrade
	err := h.db.WithContext(ctx).First(&upgrade, "razorpay_order_id = ? AND status = ?", payload.Payload.Order.Entity.ID, domain.UpgradeStatusPending).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[Razorpay Webhook] No pending upgrade found for order: %s", payload.Payload.Order.Entity.ID)
			return nil // Already processed or not related to upgrade
		}
		return err
	}

	var sub domain.Subscription
	if err := h.db.WithContext(ctx).Preload("Items").First(&sub, "id = ?", upgrade.SubscriptionID).Error; err != nil {
		return err
	}

	oldBytes, _ := json.Marshal(sub)

	// Deserialize target upgrades
	var upgradeItems []dto.SubscriptionModuleInput
	if err := json.Unmarshal(upgrade.UpgradeItems, &upgradeItems); err != nil {
		return err
	}

	// Apply items within GORM transaction
	err = h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		itemMap := make(map[string]*domain.SubscriptionItem)
		for i := range sub.Items {
			itemMap[sub.Items[i].ItemCode] = &sub.Items[i]
		}

		for _, up := range upgradeItems {
			item, exists := itemMap[up.ItemCode]
			if exists {
				// Update quantity
				item.Quantity = up.Quantity
				item.Amount = item.UnitPrice * float64(item.Quantity)
				item.UpdatedAt = time.Now().UTC()
				if err := tx.Save(item).Error; err != nil {
					return err
				}
			} else {
				// Get catalog info to create new item
				var catalog domain.BillingItemCatalog
				if err := tx.First(&catalog, "code = ?", up.ItemCode).Error; err != nil {
					return err
				}

				newItem := domain.SubscriptionItem{
					ID:             uuid.New(),
					SubscriptionID: sub.ID,
					ItemCode:       catalog.Code,
					Name:           catalog.Name,
					Type:           catalog.Type,
					BillingType:    catalog.BillingType,
					UnitPrice:      catalog.UnitPrice,
					Quantity:       up.Quantity,
					Amount:         catalog.UnitPrice * float64(up.Quantity),
					Status:         domain.SubscriptionItemActive,
					CreatedAt:      time.Now().UTC(),
					UpdatedAt:      time.Now().UTC(),
				}
				if err := tx.Create(&newItem).Error; err != nil {
					return err
				}
			}
		}

		upgrade.Status = domain.UpgradeStatusCompleted
		upgrade.UpdatedAt = time.Now().UTC()
		if err := tx.Save(&upgrade).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Reload updated sub
	if err := h.db.WithContext(ctx).Preload("Items").First(&sub, "id = ?", sub.ID).Error; err != nil {
		return err
	}

	sub.RecalculateTotals()
	sub.UpdatedAt = time.Now().UTC()
	if err := h.db.WithContext(ctx).Save(&sub).Error; err != nil {
		return err
	}

	// Create prorated invoice and record payment
	if err := h.createInvoiceAndPaymentForUpgrade(ctx, &sub, &upgrade, upgradeItems); err != nil {
		log.Printf("Warning: failed to generate upgrade invoice: %v", err)
	}

	// Write Audit Log
	newBytes, _ := json.Marshal(sub)
	_ = h.db.WithContext(ctx).Create(&domain.SubscriptionAuditLog{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Action:         "upgrade",
		OldDetails:     string(oldBytes),
		NewDetails:     string(newBytes),
		PerformedBy:    "Razorpay Webhook",
		CreatedAt:      time.Now().UTC(),
	})

	// Emit Event to Kafka to enable features immediately
	return h.publishEvent(ctx, &sub, shared_events.OrganizationSubscribed)
}

func (h *RazorpayWebhookHandler) handleSubscriptionCancelled(ctx context.Context, payload razorpayWebhookPayload) error {
	var sub domain.Subscription
	if err := h.db.WithContext(ctx).Preload("Items").First(&sub, "razorpay_subscription_id = ?", payload.Payload.Subscription.Entity.ID).Error; err != nil {
		return fmt.Errorf("subscription not found locally: %w", err)
	}

	oldBytes, _ := json.Marshal(sub)

	sub.Status = domain.SubscriptionStatusCancelled
	sub.UpdatedAt = time.Now().UTC()

	if err := h.db.WithContext(ctx).Save(&sub).Error; err != nil {
		return err
	}

	// Write Audit Log
	newBytes, _ := json.Marshal(sub)
	_ = h.db.WithContext(ctx).Create(&domain.SubscriptionAuditLog{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Action:         "cancel",
		OldDetails:     string(oldBytes),
		NewDetails:     string(newBytes),
		PerformedBy:    "Razorpay Webhook",
		CreatedAt:      time.Now().UTC(),
	})

	// Emit Unsubscribed Event to Kafka to disable features
	return h.publishEvent(ctx, &sub, shared_events.OrganizationUnsubscribed)
}

// Helpers

func (h *RazorpayWebhookHandler) createInvoiceAndPayment(ctx context.Context, sub *domain.Subscription, subject string) error {
	invoiceID := uuid.New()
	invoiceNumber := fmt.Sprintf("INV-SUB-%s-%d", sub.OrganizationID.String()[:8], time.Now().Unix())

	var items []domain.InvoiceItem
	for _, it := range sub.Items {
		if it.Status == domain.SubscriptionItemActive {
			items = append(items, domain.InvoiceItem{
				ID:          uuid.New(),
				InvoiceID:   invoiceID,
				Name:        it.Name,
				Description: fmt.Sprintf("AutoPay Subscription line item: %s", it.Name),
				Quantity:    float64(it.Quantity),
				UnitPrice:   it.UnitPrice,
				Tax:         it.UnitPrice * float64(it.Quantity) * 0.18,
				Total:       it.UnitPrice * float64(it.Quantity) * 1.18,
				ItemType:    "service",
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			})
		}
	}

	invoice := &domain.Invoice{
		ID:                invoiceID,
		OrganizationID:    sub.OrganizationID,
		Subject:           subject,
		InvoiceNumber:     &invoiceNumber,
		SourceSystem:      domain.SourceSystemManual,
		InvoiceDate:       time.Now().UTC(),
		DueDate:           time.Now().UTC(),
		Status:            domain.InvoiceStatusPaid,
		SubTotal:          sub.RecurringAmount,
		TaxTotal:          sub.RecurringAmount * 0.18,
		TotalAmount:       sub.TotalRecurringAmount,
		PaidAmount:        sub.TotalRecurringAmount,
		BalanceAmount:     0.00,
		Currency:          "INR",
		Notes:             "Automated AutoPay subscription billing",
		Items:             items,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	if err := h.invoiceRepo.Create(ctx, invoice); err != nil {
		return err
	}

	payment := &domain.Payment{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		InvoiceID:      invoiceID,
		Amount:         sub.TotalRecurringAmount,
		PaymentDate:    time.Now().UTC(),
		Method:         domain.PaymentMethodBank,
		Reference:      *sub.RazorpaySubscriptionID,
		Status:         domain.PaymentStatusCompleted,
		PaymentType:    domain.PaymentTypePayment,
		Notes:          "AutoPay transaction",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	return h.paymentRepo.Create(ctx, payment)
}

func (h *RazorpayWebhookHandler) createInvoiceAndPaymentForUpgrade(ctx context.Context, sub *domain.Subscription, upgrade *domain.SubscriptionUpgrade, upgradeItems []dto.SubscriptionModuleInput) error {
	invoiceID := uuid.New()
	invoiceNumber := fmt.Sprintf("INV-UPG-%s-%d", sub.OrganizationID.String()[:8], time.Now().Unix())

	var items []domain.InvoiceItem
	for _, it := range upgradeItems {
		items = append(items, domain.InvoiceItem{
			ID:          uuid.New(),
			InvoiceID:   invoiceID,
			Name:        "Upgrade: " + it.ItemCode,
			Description: fmt.Sprintf("Mid-cycle prorated upgrade to quantity %d", it.Quantity),
			Quantity:    float64(it.Quantity),
			UnitPrice:   upgrade.ProratedAmount, // Just represent the net sum
			Tax:         upgrade.TaxAmount,
			Total:       upgrade.TotalPaid,
			ItemType:    "service",
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		})
	}

	invoice := &domain.Invoice{
		ID:                invoiceID,
		OrganizationID:    sub.OrganizationID,
		Subject:           "Subscription Upgrade Invoice (Prorated)",
		InvoiceNumber:     &invoiceNumber,
		SourceSystem:      domain.SourceSystemManual,
		InvoiceDate:       time.Now().UTC(),
		DueDate:           time.Now().UTC(),
		Status:            domain.InvoiceStatusPaid,
		SubTotal:          upgrade.ProratedAmount,
		TaxTotal:          upgrade.TaxAmount,
		TotalAmount:       upgrade.TotalPaid,
		PaidAmount:        upgrade.TotalPaid,
		BalanceAmount:     0.00,
		Currency:          "INR",
		Notes:             fmt.Sprintf("Prorated upgrade payment for order %s", upgrade.RazorpayOrderID),
		Items:             items,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	if err := h.invoiceRepo.Create(ctx, invoice); err != nil {
		return err
	}

	payment := &domain.Payment{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		InvoiceID:      invoiceID,
		Amount:         upgrade.TotalPaid,
		PaymentDate:    time.Now().UTC(),
		Method:         domain.PaymentMethodBank,
		Reference:      upgrade.RazorpayOrderID,
		Status:         domain.PaymentStatusCompleted,
		PaymentType:    domain.PaymentTypePayment,
		Notes:          "One-time upgrade payment",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	return h.paymentRepo.Create(ctx, payment)
}

func (h *RazorpayWebhookHandler) publishEvent(ctx context.Context, sub *domain.Subscription, eventType shared_events.EventType) error {
	var itemPayloads []domain.SubscriptionItemPayload
	for _, it := range sub.Items {
		itemPayloads = append(itemPayloads, domain.SubscriptionItemPayload{
			ItemCode:    it.ItemCode,
			Name:        it.Name,
			Type:        it.Type,
			BillingType: it.BillingType,
			UnitPrice:   it.UnitPrice,
			Quantity:    it.Quantity,
			Amount:      it.Amount,
			Status:      string(it.Status),
		})
	}

	payload := domain.SubscriptionActivatedEvent{
		SubscriptionID:     sub.ID.String(),
		OrganizationID:     sub.OrganizationID.String(),
		RazorpaySubID:      *sub.RazorpaySubscriptionID,
		Status:             string(sub.Status),
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   sub.CurrentPeriodEnd,
		RecurringAmount:    sub.RecurringAmount,
		Items:              itemPayloads,
		Timestamp:          time.Now().UTC(),
	}

	meta := shared_events.NewEventMetadata(eventType, shared_events.AggregateOrganizationSubscription, sub.ID.String())
	return h.eventPublisher.Publish(ctx, meta, payload)
}
