package kafka

import (
	"context"
	"fmt"
	"log"
	"strings"

	"erp-billing-service/internal/domain"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EventHandler struct {
	db *gorm.DB
}

func NewEventHandler(db *gorm.DB) *EventHandler {
	return &EventHandler{db: db}
}

func (h *EventHandler) HandleMessage(ctx context.Context, topic string, key string, value []byte, headers map[string]string) error {
	return h.Handle(ctx, value)
}

func (h *EventHandler) Handle(ctx context.Context, data []byte) error {
	baseEvent, err := shared_events.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	log.Printf("Processing event: %s for aggregate: %s (%s)",
		baseEvent.Metadata.EventType,
		baseEvent.Metadata.AggregateType,
		baseEvent.Metadata.AggregateID)

	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch baseEvent.Metadata.AggregateType {
		case shared_events.AggregateCustomer:
			return h.handleCustomerEvent(tx, baseEvent)
		case shared_events.AggregateContact:
			return h.handleContactEvent(tx, baseEvent)
		case shared_events.AggregateService:
			return h.handleServiceEvent(tx, baseEvent)
		case shared_events.AggregatePart:
			return h.handlePartEvent(tx, baseEvent)
		case shared_events.AggregateItem:
			return h.handleItemEvent(tx, baseEvent)
		case shared_events.AggregateWorkOrder:
			return h.handleWorkOrderEvent(tx, baseEvent)
		case "invoice":
			return h.handleInvoiceEvent(tx, baseEvent)
		default:
			log.Printf("Ignoring unrelated aggregate type: %s", baseEvent.Metadata.AggregateType)
			return nil
		}
	})
}

func (h *EventHandler) handleCustomerEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.CustomerCreated:
		var payload shared_events.CustomerCreatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		customerID, _ := uuid.Parse(payload.CustomerID)
		orgID, _ := uuid.Parse(payload.OrganizationID)

		displayName := payload.DisplayName
		if displayName == "" {
			displayName = payload.CompanyName
			if payload.FirstName != "" || payload.LastName != "" {
				displayName = strings.TrimSpace(fmt.Sprintf("%s %s", payload.FirstName, payload.LastName))
			}
		}

		if displayName == "" {
			displayName = "Unknown Customer"
		}

		rm := domain.CustomerRM{
			ID:             customerID,
			OrganizationID: orgID,
			DisplayName:    displayName,
			CompanyName:    payload.CompanyName,
			Email:          payload.Email,
			Phone:          payload.Phone,
			BillingStreet:  payload.Street1,
			BillingCity:    payload.City,
			BillingState:   payload.State,
			BillingCode:    payload.ZipCode,
			BillingCountry: payload.Country,
			UpdatedAt:      event.Metadata.OccurredAt,
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.CustomerUpdated:
		var payload shared_events.CustomerUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		customerID, _ := uuid.Parse(payload.CustomerID)

		updates := make(map[string]interface{})

		// Helper to check if a field is updated
		isUpdated := func(field string) bool {
			for _, f := range payload.UpdatedFields {
				if f == field {
					return true
				}
			}
			return false
		}

		// Update fields if they are present in UpdatedFields or non-empty (as fallback)
		if payload.CompanyName != "" || isUpdated("company_name") {
			updates["company_name"] = payload.CompanyName
		}
		if payload.Email != "" || isUpdated("email") {
			updates["email"] = payload.Email
		}
		if payload.Phone != "" || isUpdated("phone") {
			updates["phone"] = payload.Phone
		}

		// Address fields
		if payload.Street1 != "" || isUpdated("street1") {
			updates["billing_street"] = payload.Street1
		}
		if payload.City != "" || isUpdated("city") {
			updates["billing_city"] = payload.City
		}
		if payload.State != "" || isUpdated("state") {
			updates["billing_state"] = payload.State
		}
		if payload.ZipCode != "" || isUpdated("zip_code") {
			updates["billing_code"] = payload.ZipCode
		}
		if payload.Country != "" || isUpdated("country") {
			updates["billing_country"] = payload.Country
		}

		if payload.DisplayName != "" || isUpdated("display_name") {
			updates["display_name"] = payload.DisplayName
		} else {
			var firstName = payload.FirstName
			var lastName = payload.LastName

			if firstName != "" || lastName != "" {
				// If we have at least one name part in the payload
				if firstName != "" && lastName != "" {
					updates["display_name"] = strings.TrimSpace(fmt.Sprintf("%s %s", firstName, lastName))
				} else {
					// We have only one part. We need to fetch current values to be safe.
					var current domain.CustomerRM
					if err := tx.First(&current, "id = ?", customerID).Error; err == nil {
						// This is still a bit simplified but better than nothing
						if firstName == "" {
							updates["display_name"] = strings.TrimSpace(fmt.Sprintf("%s %s", current.DisplayName, lastName))
						} else {
							updates["display_name"] = strings.TrimSpace(fmt.Sprintf("%s %s", firstName, current.DisplayName))
						}
						// Note: The above is still brittle because we don't know if current.DisplayName
						// is already a combination or just a company name.
						// But with the new DisplayName field, this fallback will be used less often.
					}
				}
			} else if payload.CompanyName != "" || isUpdated("company_name") {
				// If only company name is updated, we might want to update display name if it was company name before.
				// For now, let's just trust the explicit DisplayName update.
			}
		}

		updates["updated_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.CustomerRM{}).Where("id = ?", customerID).Updates(updates).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleContactEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.ContactCreated:
		var payload shared_events.ContactCreatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		contactID, _ := uuid.Parse(payload.ContactID)
		orgID, _ := uuid.Parse(payload.OrganizationID)
		var customerID uuid.UUID
		if payload.CompanyID != nil {
			customerID, _ = uuid.Parse(*payload.CompanyID)
		}

		rm := domain.ContactRM{
			ID:             contactID,
			OrganizationID: orgID,
			CustomerID:     customerID,
			FirstName:      payload.FirstName,
			LastName:       payload.LastName,
			Email:          payload.Email,
			Phone:          payload.Phone,
			Mobile:         payload.Mobile,
			IsPrimary:      payload.IsPrimary,
			UpdatedAt:      event.Metadata.OccurredAt,
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.ContactUpdated:
		var payload shared_events.ContactUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		contactID, _ := uuid.Parse(payload.ContactID)

		updates := make(map[string]interface{})

		isUpdated := func(field string) bool {
			for _, f := range payload.UpdatedFields {
				if f == field {
					return true
				}
			}
			return false
		}

		if payload.FirstName != "" || isUpdated("first_name") {
			updates["first_name"] = payload.FirstName
		}
		if payload.LastName != "" || isUpdated("last_name") {
			updates["last_name"] = payload.LastName
		}
		if payload.Email != "" || isUpdated("email") {
			updates["email"] = payload.Email
		}
		if payload.Phone != "" || isUpdated("phone") {
			updates["phone"] = payload.Phone
		}
		if payload.Mobile != "" || isUpdated("mobile") {
			updates["mobile"] = payload.Mobile
		}
		if payload.IsPrimary != nil || isUpdated("is_primary") {
			updates["is_primary"] = payload.IsPrimary
		}
		if payload.CompanyID != nil {
			custID, _ := uuid.Parse(*payload.CompanyID)
			updates["customer_id"] = custID
		}

		updates["updated_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.ContactRM{}).Where("id = ?", contactID).Updates(updates).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleServiceEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	var payload shared_events.ServiceCreatedPayload
	if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
		return err
	}

	serviceID, _ := uuid.Parse(payload.ServiceID)
	orgID, _ := uuid.Parse(payload.OrganizationID)

	rm := domain.ItemRM{
		ID:             serviceID,
		OrganizationID: orgID,
		Name:           payload.Name,
		Description:    payload.Description,
		ItemType:       "service",
		Status:         payload.Status,
		SellingPrice:   payload.BasePrice,
		UpdatedAt:      event.Metadata.OccurredAt,
	}

	return tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&rm).Error
}

func (h *EventHandler) handlePartEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	var payload shared_events.PartCreatedPayload
	if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
		return err
	}

	partID, _ := uuid.Parse(payload.PartID)
	orgID, _ := uuid.Parse(payload.OrganizationID)

	rm := domain.ItemRM{
		ID:             partID,
		OrganizationID: orgID,
		SKU:            payload.PartNumber,
		Name:           payload.Name,
		Description:    payload.Description,
		ItemType:       "part",
		Status:         payload.Status,
		SellingPrice:   payload.UnitPrice,
		UpdatedAt:      event.Metadata.OccurredAt,
	}

	return tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&rm).Error
}

func (h *EventHandler) handleWorkOrderEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.WorkOrderCreated:
		var payload shared_events.WorkOrderCreatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, _ := uuid.Parse(payload.WorkOrderID)
		orgID, _ := uuid.Parse(payload.OrganizationID)
		custID, _ := uuid.Parse(payload.CustomerID)
		contID, _ := uuid.Parse(payload.ContactID)

		rm := domain.WorkOrderRM{
			ID:             id,
			OrganizationID: orgID,
			Summary:        payload.Summary,
			Status:         payload.Status,
			BillingStatus:  payload.BillingStatus,
			CustomerID:     &custID,
			ContactID:      &contID,
			GrandTotal:     payload.GrandTotal,
			UpdatedAt:      event.Metadata.OccurredAt,
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.WorkOrderUpdated:
		var payload shared_events.WorkOrderUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, _ := uuid.Parse(payload.WorkOrderID)

		updates := make(map[string]interface{})
		if payload.Summary != "" {
			updates["summary"] = payload.Summary
		}
		if payload.Status != "" {
			updates["status"] = payload.Status
		}
		if payload.BillingStatus != "" {
			updates["billing_status"] = payload.BillingStatus
		}
		if payload.GrandTotal > 0 {
			updates["grand_total"] = payload.GrandTotal
		}
		updates["updated_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.WorkOrderRM{}).Where("id = ?", id).Updates(updates).Error

	case shared_events.WorkOrderDeleted:
		var payload shared_events.WorkOrderDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}
		id, _ := uuid.Parse(payload.WorkOrderID)
		return tx.Delete(&domain.WorkOrderRM{}, "id = ?", id).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleItemEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.ItemCreated:
		var payload shared_events.ItemCreatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		itemID, _ := uuid.Parse(payload.ItemID)
		orgID, _ := uuid.Parse(payload.OrganizationID)

		// Determine item type
		itemType := "part"
		if payload.Type == "service" {
			itemType = "service"
		} else if payload.Type == "goods" {
			itemType = "goods"
		}

		// Extract pricing information
		var sellingPrice, costPrice float64
		var currency, unit string
		var taxable bool
		var taxRate float64

		if payload.SalesInfo != nil {
			if v, ok := payload.SalesInfo["selling_price"].(float64); ok {
				sellingPrice = v
			}
			if v, ok := payload.SalesInfo["rate"].(float64); ok && sellingPrice == 0 {
				sellingPrice = v
			}
			if v, ok := payload.SalesInfo["selling_currency"].(string); ok {
				currency = v
			}
			if v, ok := payload.SalesInfo["taxable"].(bool); ok {
				taxable = v
			}
			if v, ok := payload.SalesInfo["tax_rate"].(float64); ok {
				taxRate = v
			}
		}

		if payload.PurchaseInfo != nil {
			if v, ok := payload.PurchaseInfo["cost_price"].(float64); ok {
				costPrice = v
			}
			if currency == "" {
				if v, ok := payload.PurchaseInfo["cost_currency"].(string); ok {
					currency = v
				}
			}
		}

		// Extract inventory information
		var qtyOnHand, qtyAvailable, qtyReserved, qtyDamaged float64
		var reorderLevel, reorderQty float64
		var trackInventory bool

		if payload.InventoryInfo != nil {
			if v, ok := payload.InventoryInfo["quantity_on_hand"].(float64); ok {
				qtyOnHand = v
			}
			if v, ok := payload.InventoryInfo["quantity_available"].(float64); ok {
				qtyAvailable = v
			}
			if v, ok := payload.InventoryInfo["quantity_reserved"].(float64); ok {
				qtyReserved = v
			}
			if v, ok := payload.InventoryInfo["quantity_damaged"].(float64); ok {
				qtyDamaged = v
			}
			if v, ok := payload.InventoryInfo["reorder_level"].(float64); ok {
				reorderLevel = v
			}
			if v, ok := payload.InventoryInfo["reorder_quantity"].(float64); ok {
				reorderQty = v
			}
			if v, ok := payload.InventoryInfo["track_inventory"].(bool); ok {
				trackInventory = v
			}
		}

		// Extract description
		var description string
		if payload.SalesInfo != nil {
			if v, ok := payload.SalesInfo["description"].(string); ok {
				description = v
			}
		}

		rm := domain.ItemRM{
			ID:                itemID,
			OrganizationID:    orgID,
			SKU:               payload.SKU,
			Name:              payload.Name,
			Description:       description,
			ItemType:          itemType,
			Status:            payload.Status,
			SellingPrice:      sellingPrice,
			CostPrice:         costPrice,
			Currency:          currency,
			Unit:              unit,
			QuantityOnHand:    qtyOnHand,
			QuantityAvailable: qtyAvailable,
			QuantityReserved:  qtyReserved,
			QuantityDamaged:   qtyDamaged,
			ReorderLevel:      reorderLevel,
			ReorderQuantity:   reorderQty,
			TrackInventory:    trackInventory,
			Taxable:           taxable,
			TaxRate:           taxRate,
			UpdatedAt:         event.Metadata.OccurredAt,
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.ItemUpdated:
		var payload shared_events.ItemUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		itemID, _ := uuid.Parse(payload.ItemID)

		updates := make(map[string]interface{})

		isUpdated := func(field string) bool {
			for _, f := range payload.UpdatedFields {
				if f == field {
					return true
				}
			}
			return false
		}

		// Basic fields
		if payload.Name != "" || isUpdated("name") {
			updates["name"] = payload.Name
		}
		if payload.SKU != "" || isUpdated("sku") {
			updates["sku"] = payload.SKU
		}
		if payload.Status != "" || isUpdated("status") {
			updates["status"] = payload.Status
		}

		// Item type
		if payload.Type != "" || isUpdated("type") {
			if payload.Type == "service" {
				updates["item_type"] = "service"
			} else if payload.Type == "goods" {
				updates["item_type"] = "goods"
			} else {
				updates["item_type"] = "part"
			}
		}

		// Sales info (pricing and tax)
		if payload.SalesInfo != nil || isUpdated("sales_info") {
			if payload.SalesInfo != nil {
				if v, ok := payload.SalesInfo["selling_price"].(float64); ok {
					updates["selling_price"] = v
				}
				if v, ok := payload.SalesInfo["rate"].(float64); ok {
					if _, exists := updates["selling_price"]; !exists {
						updates["selling_price"] = v
					}
				}
				if v, ok := payload.SalesInfo["selling_currency"].(string); ok {
					updates["currency"] = v
				}
				if v, ok := payload.SalesInfo["description"].(string); ok {
					updates["description"] = v
				}
				if v, ok := payload.SalesInfo["taxable"].(bool); ok {
					updates["taxable"] = v
				}
				if v, ok := payload.SalesInfo["tax_rate"].(float64); ok {
					updates["tax_rate"] = v
				}
			}
		}

		// Purchase info
		if payload.PurchaseInfo != nil || isUpdated("purchase_info") {
			if payload.PurchaseInfo != nil {
				if v, ok := payload.PurchaseInfo["cost_price"].(float64); ok {
					updates["cost_price"] = v
				}
				if v, ok := payload.PurchaseInfo["cost_currency"].(string); ok {
					if _, exists := updates["currency"]; !exists {
						updates["currency"] = v
					}
				}
			}
		}

		// Inventory info
		if payload.InventoryInfo != nil || isUpdated("inventory_info") {
			if payload.InventoryInfo != nil {
				if v, ok := payload.InventoryInfo["quantity_on_hand"].(float64); ok {
					updates["quantity_on_hand"] = v
				}
				if v, ok := payload.InventoryInfo["quantity_available"].(float64); ok {
					updates["quantity_available"] = v
				}
				if v, ok := payload.InventoryInfo["quantity_reserved"].(float64); ok {
					updates["quantity_reserved"] = v
				}
				if v, ok := payload.InventoryInfo["quantity_damaged"].(float64); ok {
					updates["quantity_damaged"] = v
				}
				if v, ok := payload.InventoryInfo["reorder_level"].(float64); ok {
					updates["reorder_level"] = v
				}
				if v, ok := payload.InventoryInfo["reorder_quantity"].(float64); ok {
					updates["reorder_quantity"] = v
				}
				if v, ok := payload.InventoryInfo["track_inventory"].(bool); ok {
					updates["track_inventory"] = v
				}
			}
		}

		updates["updated_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.ItemRM{}).Where("id = ?", itemID).Updates(updates).Error

	case shared_events.ItemDeleted:
		var payload shared_events.ItemDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}
		id, _ := uuid.Parse(payload.ItemID)
		return tx.Delete(&domain.ItemRM{}, "id = ?", id).Error

	default:
		return nil
	}
}
