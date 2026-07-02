package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/external"
	"erp-billing-service/internal/ports/repositories"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
)

type SubscriptionService struct {
	subRepo        repositories.SubscriptionRepository
	invoiceRepo    domain.InvoiceRepository
	paymentRepo    domain.PaymentRepository
	rmRepo         domain.ReadModelRepository
	eventPublisher domain.EventPublisher
	razorpayClient external.RazorpayClient
}

func NewSubscriptionService(
	subRepo repositories.SubscriptionRepository,
	invoiceRepo domain.InvoiceRepository,
	paymentRepo domain.PaymentRepository,
	rmRepo domain.ReadModelRepository,
	eventPublisher domain.EventPublisher,
	razorpayClient external.RazorpayClient,
) *SubscriptionService {
	return &SubscriptionService{
		subRepo:        subRepo,
		invoiceRepo:    invoiceRepo,
		paymentRepo:    paymentRepo,
		rmRepo:         rmRepo,
		eventPublisher: eventPublisher,
		razorpayClient: razorpayClient,
	}
}

// CreateSubscription initiates a new subscription and generates a Razorpay mandate payment link
func (s *SubscriptionService) CreateSubscription(ctx context.Context, req dto.CreateSubscriptionRequest) (*dto.CreateSubscriptionResponse, error) {
	// 1. Prevent duplicate active subscriptions
	existing, err := s.subRepo.GetByOrganizationID(ctx, req.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing subscription: %w", err)
	}
	if existing != nil && existing.Status != domain.SubscriptionStatusCancelled {
		return nil, errors.New("organization already has an active subscription; use upgrade or downgrade APIs")
	}

	// 2. Fetch admin user email for notifications
	var adminEmail string
	if s.rmRepo != nil {
		if org, err := s.rmRepo.GetOrganization(ctx, req.OrganizationID); err == nil && org != nil {
			adminEmail = org.Email
		}
		if adminEmail == "" {
			// Fallback to query admin user
			if adminID, err := s.rmRepo.GetOrganizationAdminID(ctx, req.OrganizationID); err == nil && adminID != nil {
				if user, err := s.rmRepo.GetCustomer(ctx, *adminID); err == nil && user != nil {
					adminEmail = user.Email
				}
			}
		}
	}
	if adminEmail == "" {
		adminEmail = "billing-admin@organization.com" // System fallback
	}

	// 3. Resolve modules from the catalog
	var subItems []domain.SubscriptionItem
	var totalRecurring float64

	for _, mod := range req.Modules {
		catalogItem, err := s.subRepo.GetCatalogItem(ctx, mod.ItemCode)
		if err != nil || catalogItem == nil {
			return nil, fmt.Errorf("invalid or inactive module code: %s", mod.ItemCode)
		}

		qty := mod.Quantity
		if qty <= 0 {
			qty = 1
		}

		amount := catalogItem.UnitPrice * float64(qty)
		totalRecurring += amount

		subItems = append(subItems, domain.SubscriptionItem{
			ID:          uuid.New(),
			ItemCode:    catalogItem.Code,
			Name:        catalogItem.Name,
			Type:        catalogItem.Type,
			BillingType: catalogItem.BillingType,
			UnitPrice:   catalogItem.UnitPrice,
			Quantity:    qty,
			Amount:      amount,
			Status:      domain.SubscriptionItemActive,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		})
	}

	if len(subItems) == 0 {
		return nil, errors.New("at least one module must be selected")
	}

	// Calculate with 18% GST
	taxPercentage := 18.00
	totalRecurringWithTax := totalRecurring * (1.0 + (taxPercentage / 100.0))

	// 4. Create Plan in Razorpay
	planName := fmt.Sprintf("ERP Org Plan - %s", req.OrganizationID.String()[:8])
	razorpayPlanID, err := s.razorpayClient.CreatePlan(ctx, planName, totalRecurringWithTax, "INR")
	if err != nil {
		return nil, fmt.Errorf("failed to create Razorpay Plan: %w", err)
	}

	// 5. Create Subscription in Razorpay (defaulting to 120 cycles / 10 years for AutoPay)
	razorpaySubID, paymentLink, err := s.razorpayClient.CreateSubscription(ctx, razorpayPlanID, adminEmail, 120)
	if err != nil {
		return nil, fmt.Errorf("failed to create Razorpay Subscription: %w", err)
	}

	// 6. Save locally
	sub := &domain.Subscription{
		ID:                     uuid.New(),
		OrganizationID:         req.OrganizationID,
		RazorpaySubscriptionID: &razorpaySubID,
		RazorpayPlanID:         razorpayPlanID,
		Status:                 domain.SubscriptionStatusCreated,
		RecurringAmount:        totalRecurring,
		TaxPercentage:          taxPercentage,
		TotalRecurringAmount:   totalRecurringWithTax,
		Currency:               "INR",
		Items:                  subItems,
		CreatedAt:              time.Now().UTC(),
		UpdatedAt:              time.Now().UTC(),
	}

	if err := s.subRepo.Create(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to save subscription: %w", err)
	}

	// Write audit log
	oldBytes, _ := json.Marshal(nil)
	newBytes, _ := json.Marshal(sub)
	_ = s.subRepo.CreateAuditLog(ctx, &domain.SubscriptionAuditLog{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Action:         "create",
		OldDetails:     string(oldBytes),
		NewDetails:     string(newBytes),
		PerformedBy:    "System",
		CreatedAt:      time.Now().UTC(),
	})

	return &dto.CreateSubscriptionResponse{
		SubscriptionID:         sub.ID.String(),
		RazorpaySubscriptionID: razorpaySubID,
		PaymentLink:            paymentLink,
	}, nil
}

// UpgradeSubscription calculates prorated amounts for upgrades, generates a one-time order, and tracks it
func (s *SubscriptionService) UpgradeSubscription(ctx context.Context, req dto.UpgradeSubscriptionRequest) (*dto.UpgradeSubscriptionResponse, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, req.OrganizationID)
	if err != nil || sub == nil {
		return nil, errors.New("no active subscription found for this organization")
	}
	if sub.Status != domain.SubscriptionStatusActive {
		return nil, fmt.Errorf("subscription must be active to upgrade; current status: %s", sub.Status)
	}

	now := time.Now().UTC()

	// 1. Calculate billing days remaining in the current cycle
	totalCycleDays := sub.CurrentPeriodEnd.Sub(sub.CurrentPeriodStart).Hours() / 24
	remainingDays := sub.CurrentPeriodEnd.Sub(now).Hours() / 24

	// Clamp values to prevent negative or zero days
	if totalCycleDays <= 0 {
		totalCycleDays = 30.0
	}
	if remainingDays <= 0 {
		remainingDays = 1.0
	}
	if remainingDays > totalCycleDays {
		remainingDays = totalCycleDays
	}

	proratedRatio := remainingDays / totalCycleDays

	// Map current items for quick lookup
	currentItems := make(map[string]domain.SubscriptionItem)
	for _, item := range sub.Items {
		if item.Status == domain.SubscriptionItemActive {
			currentItems[item.ItemCode] = item
		}
	}

	var totalProratedAmount float64
	var upgradeItems []dto.SubscriptionModuleInput

	// 2. Compute prorated price differences
	for _, upgradeMod := range req.Modules {
		catalogItem, err := s.subRepo.GetCatalogItem(ctx, upgradeMod.ItemCode)
		if err != nil || catalogItem == nil {
			return nil, fmt.Errorf("invalid or inactive module code: %s", upgradeMod.ItemCode)
		}

		qty := upgradeMod.Quantity
		if qty <= 0 {
			qty = 1
		}

		currentItem, exists := currentItems[upgradeMod.ItemCode]
		var baseDiff float64

		if exists {
			// Upgrade of quantity / workforce size
			if qty <= currentItem.Quantity {
				continue // Not an upgrade
			}
			diffQty := qty - currentItem.Quantity
			baseDiff = catalogItem.UnitPrice * float64(diffQty)
		} else {
			// Brand new module addition
			baseDiff = catalogItem.UnitPrice * float64(qty)
		}

		proratedAmount := baseDiff * proratedRatio
		totalProratedAmount += proratedAmount

		upgradeItems = append(upgradeItems, dto.SubscriptionModuleInput{
			ItemCode: catalogItem.Code,
			Quantity: qty,
		})
	}

	if len(upgradeItems) == 0 {
		return nil, errors.New("no upgrade items detected (all items already exist with higher or equal quantities)")
	}

	taxAmount := totalProratedAmount * (sub.TaxPercentage / 100.0)
	totalPaid := totalProratedAmount + taxAmount

	// 3. Generate Razorpay one-time Order for the prorated amount + GST
	receiptID := fmt.Sprintf("rcpt_upg_%s_%d", sub.ID.String()[:8], now.Unix())
	orderID, err := s.razorpayClient.CreateOrder(ctx, totalPaid, "INR", receiptID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Razorpay Order: %w", err)
	}

	// 4. Save SubscriptionUpgrade metadata to be applied on payment webhook
	upgradeMetadataBytes, _ := json.Marshal(upgradeItems)
	upgrade := &domain.SubscriptionUpgrade{
		ID:              uuid.New(),
		SubscriptionID:  sub.ID,
		RazorpayOrderID: orderID,
		Status:          domain.UpgradeStatusPending,
		ProratedAmount:  totalProratedAmount,
		TaxAmount:       taxAmount,
		TotalPaid:       totalPaid,
		UpgradeItems:    upgradeMetadataBytes,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.subRepo.CreateUpgrade(ctx, upgrade); err != nil {
		return nil, fmt.Errorf("failed to track pending upgrade: %w", err)
	}

	return &dto.UpgradeSubscriptionResponse{
		RazorpayOrderID: orderID,
		ProratedAmount:  math.Round(totalProratedAmount*100) / 100,
		TaxAmount:       math.Round(taxAmount*100) / 100,
		TotalAmount:     math.Round(totalPaid*100) / 100,
	}, nil
}

// DowngradeSubscription schedules downgrades to take effect on the next renewal date
func (s *SubscriptionService) DowngradeSubscription(ctx context.Context, req dto.DowngradeSubscriptionRequest) (*dto.SubscriptionResponse, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, req.OrganizationID)
	if err != nil || sub == nil {
		return nil, errors.New("no active subscription found for this organization")
	}
	if sub.Status != domain.SubscriptionStatusActive {
		return nil, fmt.Errorf("subscription must be active to downgrade; current status: %s", sub.Status)
	}

	oldBytes, _ := json.Marshal(sub)

	// Map current items for lookup
	itemMap := make(map[string]*domain.SubscriptionItem)
	for i := range sub.Items {
		itemMap[sub.Items[i].ItemCode] = &sub.Items[i]
	}

	hasChanges := false

	// Apply pending removal / quantity decreases
	for _, downMod := range req.Modules {
		item, exists := itemMap[downMod.ItemCode]
		if !exists || item.Status == domain.SubscriptionItemPendingRemoval {
			continue
		}

		if downMod.Quantity <= 0 {
			// Mark complete removal
			item.Status = domain.SubscriptionItemPendingRemoval
			hasChanges = true
		} else if downMod.Quantity < item.Quantity {
			// Mark workforce decrease
			targetQty := downMod.Quantity
			item.PendingQuantity = &targetQty
			hasChanges = true
		}
	}

	if !hasChanges {
		return nil, errors.New("no downgrades detected")
	}

	// 1. Calculate what the new recurring amount *will* be at next cycle
	sub.RecalculateTotals()

	// 2. Create new Plan in Razorpay for the lower amount
	planName := fmt.Sprintf("ERP Org Plan - %s", sub.OrganizationID.String()[:8])
	newPlanID, err := s.razorpayClient.CreatePlan(ctx, planName, sub.TotalRecurringAmount, "INR")
	if err != nil {
		return nil, fmt.Errorf("failed to create Razorpay Plan for downgrade: %w", err)
	}

	// 3. Update subscription on Razorpay to point to new Plan at cycle_end
	err = s.razorpayClient.UpdateSubscriptionPlan(ctx, *sub.RazorpaySubscriptionID, newPlanID, "cycle_end")
	if err != nil {
		return nil, fmt.Errorf("failed to update Razorpay Subscription plan: %w", err)
	}

	// 4. Persist updated details (retaining features till cycle ends)
	sub.RazorpayPlanID = newPlanID
	sub.UpdatedAt = time.Now().UTC()

	if err := s.subRepo.Update(ctx, sub); err != nil {
		return nil, fmt.Errorf("failed to save subscription downgrades: %w", err)
	}

	// Write audit log
	newBytes, _ := json.Marshal(sub)
	_ = s.subRepo.CreateAuditLog(ctx, &domain.SubscriptionAuditLog{
		ID:             uuid.New(),
		OrganizationID: sub.OrganizationID,
		SubscriptionID: sub.ID,
		Action:         "downgrade",
		OldDetails:     string(oldBytes),
		NewDetails:     string(newBytes),
		PerformedBy:    "System",
		CreatedAt:      time.Now().UTC(),
	})

	// Publish DowngradedEvent to Kafka to notify microservices of scheduled changes (if relevant)
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
	meta := shared_events.NewEventMetadata(shared_events.OrganizationSubscribed, shared_events.AggregateOrganizationSubscription, sub.ID.String())
	if s.eventPublisher != nil {
		_ = s.eventPublisher.Publish(ctx, meta, domain.SubscriptionDowngradedEvent{
			SubscriptionID:  sub.ID.String(),
			OrganizationID:  sub.OrganizationID.String(),
			RecurringAmount: sub.RecurringAmount,
			Items:           itemPayloads,
			Timestamp:       time.Now().UTC(),
		})
	}

	return s.MapToResponse(sub), nil
}

// GetStatus returns the current subscription response
func (s *SubscriptionService) GetStatus(ctx context.Context, orgID uuid.UUID) (*dto.SubscriptionResponse, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, errors.New("no subscription found for this organization")
	}
	return s.MapToResponse(sub), nil
}

// GetHistory returns audit log history
func (s *SubscriptionService) GetHistory(ctx context.Context, orgID uuid.UUID) ([]domain.SubscriptionAuditLog, error) {
	sub, err := s.subRepo.GetByOrganizationID(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, errors.New("no subscription found for this organization")
	}
	return s.subRepo.ListAuditLogs(ctx, sub.ID)
}

// MapToResponse converts domain.Subscription into DTO response
func (s *SubscriptionService) MapToResponse(sub *domain.Subscription) *dto.SubscriptionResponse {
	var items []dto.SubscriptionItemResponse
	for _, it := range sub.Items {
		var pendingQty *int
		if it.PendingQuantity != nil {
			val := *it.PendingQuantity
			pendingQty = &val
		}
		items = append(items, dto.SubscriptionItemResponse{
			ID:              it.ID.String(),
			ItemCode:        it.ItemCode,
			Name:            it.Name,
			Type:            it.Type,
			BillingType:     it.BillingType,
			UnitPrice:       it.UnitPrice,
			Quantity:        it.Quantity,
			Amount:          it.Amount,
			Status:          string(it.Status),
			PendingQuantity: pendingQty,
		})
	}

	razorpaySubID := ""
	if sub.RazorpaySubscriptionID != nil {
		razorpaySubID = *sub.RazorpaySubscriptionID
	}

	return &dto.SubscriptionResponse{
		ID:                     sub.ID.String(),
		OrganizationID:         sub.OrganizationID.String(),
		RazorpaySubscriptionID: razorpaySubID,
		RazorpayPlanID:         sub.RazorpayPlanID,
		Status:                 string(sub.Status),
		CurrentPeriodStart:     sub.CurrentPeriodStart,
		CurrentPeriodEnd:       sub.CurrentPeriodEnd,
		RenewalDate:            sub.RenewalDate,
		RecurringAmount:        sub.RecurringAmount,
		TaxPercentage:          sub.TaxPercentage,
		TotalRecurringAmount:   sub.TotalRecurringAmount,
		Currency:               sub.Currency,
		Items:                  items,
	}
}
