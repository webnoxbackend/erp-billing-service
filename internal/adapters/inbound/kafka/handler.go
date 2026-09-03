package kafka

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EventHandler struct {
	db             *gorm.DB
	invoiceService *application.InvoiceService
}

func NewEventHandler(db *gorm.DB, invoiceService *application.InvoiceService) *EventHandler {
	return &EventHandler{
		db:             db,
		invoiceService: invoiceService,
	}
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

	txErr := h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
		case shared_events.AggregateOrganization:
			return h.handleOrganizationEvent(tx, baseEvent)
		case shared_events.AggregateAddress:
			return h.handleAddressEvent(tx, baseEvent)
		case shared_events.AggregateServiceCategory:
			return h.handleServiceCategoryEvent(tx, baseEvent)
		case "invoice":
			return h.handleInvoiceEvent(tx, baseEvent)
		case shared_events.AggregateAppointment:
			return h.handleAppointmentEvent(tx, baseEvent)
		case shared_events.AggregateUser:
			return h.handleUserEvent(tx, baseEvent)
		default:
			log.Printf("Ignoring unrelated aggregate type: %s", baseEvent.Metadata.AggregateType)
			return nil
		}
	})

	if txErr != nil {
		return txErr
	}

	// Trigger auto-invoice generation in the background upon appointment completion
	if baseEvent.Metadata.AggregateType == shared_events.AggregateAppointment &&
		baseEvent.Metadata.EventType == shared_events.AppointmentCompleted {
		
		var payload struct {
			shared_events.AppointmentPayload
		}
		if err := shared_events.UnmarshalPayload(baseEvent, &payload); err == nil {
			if payload.WorkOrderID != "" {
				go func() {
					log.Printf("[AutoInvoice] Detected completed appointment %s. Triggering invoice auto-generation for work order %s...", 
						payload.AppointmentID, payload.WorkOrderID)
					bgCtx := context.Background()
					if err := h.autoGenerateInvoice(bgCtx, payload.WorkOrderID, payload.OrganizationID); err != nil {
						log.Printf("[AutoInvoice] [ERROR] Failed to auto-generate invoice for work order %s: %v", payload.WorkOrderID, err)
					}
				}()
			}
		} else {
			log.Printf("[AutoInvoice] [ERROR] Failed to unmarshal appointment payload: %v", err)
		}
	}

	return nil
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

		var externalKey *string
		if payload.ExternalKey != "" {
			externalKey = &payload.ExternalKey
		}

		var serviceAddressID, billingAddressID, shippingAddressID, parentAccountID *uuid.UUID
		if payload.ServiceAddressID != "" {
			if parsed, err := uuid.Parse(payload.ServiceAddressID); err == nil {
				serviceAddressID = &parsed
			}
		}
		if payload.BillingAddressID != "" {
			if parsed, err := uuid.Parse(payload.BillingAddressID); err == nil {
				billingAddressID = &parsed
			}
		}
		if payload.ShippingAddressID != "" {
			if parsed, err := uuid.Parse(payload.ShippingAddressID); err == nil {
				shippingAddressID = &parsed
			}
		}
		if payload.ParentAccountID != "" {
			if parsed, err := uuid.Parse(payload.ParentAccountID); err == nil {
				parentAccountID = &parsed
			}
		}

		rm := domain.CustomerRM{
			ID:                customerID,
			OrganizationID:    orgID,
			ExternalKey:       externalKey,
			CustomerType:      payload.CustomerType,
			DisplayName:       displayName,
			CompanyName:       payload.CompanyName,
			FirstName:         payload.FirstName,
			LastName:          payload.LastName,
			Salutation:        payload.Salutation,
			Email:             payload.Email,
			PhoneWork:         payload.PhoneWork,
			PhoneMobile:       payload.PhoneMobile,
			WebsiteURL:        payload.WebsiteURL,
			TaxNumber:         payload.TaxNumber,
			CurrencyCode:      payload.CurrencyCode,
			PaymentTerms:      payload.PaymentTerms,
			IsTaxable:         payload.IsTaxable,
			Industry:          payload.Industry,
			Rating:            payload.Rating,
			Ownership:         payload.Ownership,
			AnnualRevenue:     payload.AnnualRevenue,
			PortalEnabled:     payload.PortalEnabled,
			Status:            payload.Status,
			SourceSystem:      payload.SourceSystem,
			SourceID:          payload.SourceID,
			ServiceAddressID:  serviceAddressID,
			BillingAddressID:  billingAddressID,
			ShippingAddressID: shippingAddressID,
			AccountOwner:      payload.AccountOwner,
			AccountSite:       payload.AccountSite,
			ParentAccountID:   parentAccountID,
			CustomerLanguage:  payload.CustomerLanguage,
			CreatedAt:         event.Metadata.OccurredAt,
			UpdatedAt:         event.Metadata.OccurredAt,

			// Ignored DB fields filled for compatibility
			Phone:            payload.Phone,
			BillingStreet:    payload.Street1,
			BillingCity:      payload.City,
			BillingState:     payload.State,
			BillingCode:      payload.ZipCode,
			BillingCountry:   payload.Country,
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

		if payload.ExternalKey != "" || isUpdated("external_key") {
			if payload.ExternalKey == "" {
				updates["external_key"] = nil
			} else {
				updates["external_key"] = &payload.ExternalKey
			}
		}
		if payload.CustomerType != "" || isUpdated("customer_type") {
			updates["customer_type"] = payload.CustomerType
		}
		if payload.CompanyName != "" || isUpdated("company_name") {
			updates["company_name"] = payload.CompanyName
		}
		if payload.FirstName != "" || isUpdated("first_name") {
			updates["first_name"] = payload.FirstName
		}
		if payload.LastName != "" || isUpdated("last_name") {
			updates["last_name"] = payload.LastName
		}
		if payload.Salutation != "" || isUpdated("salutation") {
			updates["salutation"] = payload.Salutation
		}
		if payload.Email != "" || isUpdated("email") {
			updates["email"] = payload.Email
		}
		if payload.PhoneWork != "" || isUpdated("phone_work") {
			updates["phone_work"] = payload.PhoneWork
		}
		if payload.PhoneMobile != "" || isUpdated("phone_mobile") {
			updates["phone_mobile"] = payload.PhoneMobile
		}
		if payload.WebsiteURL != "" || isUpdated("website_url") {
			updates["website_url"] = payload.WebsiteURL
		}
		if payload.TaxNumber != "" || isUpdated("tax_number") {
			updates["tax_number"] = payload.TaxNumber
		}
		if payload.CurrencyCode != "" || isUpdated("currency_code") {
			updates["currency_code"] = payload.CurrencyCode
		}
		if payload.PaymentTerms != "" || isUpdated("payment_terms") {
			updates["payment_terms"] = payload.PaymentTerms
		}
		if payload.IsTaxable != nil || isUpdated("is_taxable") {
			updates["is_taxable"] = payload.IsTaxable
		}
		if payload.Industry != "" || isUpdated("industry") {
			updates["industry"] = payload.Industry
		}
		if payload.Rating != "" || isUpdated("rating") {
			updates["rating"] = payload.Rating
		}
		if payload.Ownership != "" || isUpdated("ownership") {
			updates["ownership"] = payload.Ownership
		}
		if payload.AnnualRevenue != nil || isUpdated("annual_revenue") {
			updates["annual_revenue"] = payload.AnnualRevenue
		}
		if payload.PortalEnabled != nil || isUpdated("portal_enabled") {
			updates["portal_enabled"] = payload.PortalEnabled
		}
		if payload.Status != "" || isUpdated("status") {
			updates["status"] = payload.Status
		}
		if payload.SourceSystem != "" || isUpdated("source_system") {
			updates["source_system"] = payload.SourceSystem
		}
		if payload.SourceID != "" || isUpdated("source_id") {
			updates["source_id"] = payload.SourceID
		}
		if payload.ServiceAddressID != "" || isUpdated("service_address_id") {
			if payload.ServiceAddressID == "" {
				updates["service_address_id"] = nil
			} else if parsed, err := uuid.Parse(payload.ServiceAddressID); err == nil {
				updates["service_address_id"] = &parsed
			}
		}
		if payload.BillingAddressID != "" || isUpdated("billing_address_id") {
			if payload.BillingAddressID == "" {
				updates["billing_address_id"] = nil
			} else if parsed, err := uuid.Parse(payload.BillingAddressID); err == nil {
				updates["billing_address_id"] = &parsed
			}
		}
		if payload.ShippingAddressID != "" || isUpdated("shipping_address_id") {
			if payload.ShippingAddressID == "" {
				updates["shipping_address_id"] = nil
			} else if parsed, err := uuid.Parse(payload.ShippingAddressID); err == nil {
				updates["shipping_address_id"] = &parsed
			}
		}
		if payload.AccountOwner != "" || isUpdated("account_owner") {
			updates["account_owner"] = payload.AccountOwner
		}
		if payload.AccountSite != "" || isUpdated("account_site") {
			updates["account_site"] = payload.AccountSite
		}
		if payload.ParentAccountID != "" || isUpdated("parent_account_id") {
			if payload.ParentAccountID == "" {
				updates["parent_account_id"] = nil
			} else if parsed, err := uuid.Parse(payload.ParentAccountID); err == nil {
				updates["parent_account_id"] = &parsed
			}
		}
		if payload.CustomerLanguage != "" || isUpdated("customer_language") {
			updates["customer_language"] = payload.CustomerLanguage
		}

		// Also update the compatibility flat fields in updates
		if payload.Phone != "" || isUpdated("phone") {
			updates["phone"] = payload.Phone
		}
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
				if firstName != "" && lastName != "" {
					updates["display_name"] = strings.TrimSpace(fmt.Sprintf("%s %s", firstName, lastName))
				} else {
					var current domain.CustomerRM
					if err := tx.First(&current, "id = ?", customerID).Error; err == nil {
						if firstName == "" {
							updates["display_name"] = strings.TrimSpace(fmt.Sprintf("%s %s", current.DisplayName, lastName))
						} else {
							updates["display_name"] = strings.TrimSpace(fmt.Sprintf("%s %s", firstName, current.DisplayName))
						}
					}
				}
			}
		}

		updates["updated_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.CustomerRM{}).Where("id = ?", customerID).Updates(updates).Error

	case shared_events.CustomerDeleted:
		var payload shared_events.CustomerDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		customerID, _ := uuid.Parse(payload.CustomerID)
		return tx.Where("id = ?", customerID).Delete(&domain.CustomerRM{}).Error

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

	salesInfo := domain.JSONB{
		"selling_price": payload.BasePrice,
		"selling_currency": "INR",
		"sellable": true,
		"taxable": false,
		"discount_allowed": false,
	}

	rm := domain.ItemRM{
		ID:             serviceID,
		OrganizationID: orgID,
		Name:           payload.Name,
		Description:    payload.Description,
		Type:           "service",
		Status:         payload.Status,
		SalesInfo:      salesInfo,
		UpdatedAt:      event.Metadata.OccurredAt,
		CreatedAt:      event.Metadata.OccurredAt,
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

	salesInfo := domain.JSONB{
		"selling_price": payload.UnitPrice,
		"selling_currency": "INR",
		"sellable": true,
		"taxable": false,
		"discount_allowed": false,
	}

	rm := domain.ItemRM{
		ID:             partID,
		OrganizationID: orgID,
		SKU:            payload.PartNumber,
		Name:           payload.Name,
		Description:    payload.Description,
		Type:           "goods",
		Status:         payload.Status,
		SalesInfo:      salesInfo,
		UpdatedAt:      event.Metadata.OccurredAt,
		CreatedAt:      event.Metadata.OccurredAt,
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

		var requestID, estimateID, assetID, requiredSkillID, serviceAddressID, billingAddressID *uuid.UUID
		if payload.RequestID != "" {
			if parsed, err := uuid.Parse(payload.RequestID); err == nil {
				requestID = &parsed
			}
		}
		if payload.EstimateID != "" {
			if parsed, err := uuid.Parse(payload.EstimateID); err == nil {
				estimateID = &parsed
			}
		}
		if payload.AssetID != "" {
			if parsed, err := uuid.Parse(payload.AssetID); err == nil {
				assetID = &parsed
			}
		}
		if payload.RequiredSkillID != "" {
			if parsed, err := uuid.Parse(payload.RequiredSkillID); err == nil {
				requiredSkillID = &parsed
			}
		}
		if payload.ServiceAddressID != "" {
			if parsed, err := uuid.Parse(payload.ServiceAddressID); err == nil {
				serviceAddressID = &parsed
			}
		}
		if payload.BillingAddressID != "" {
			if parsed, err := uuid.Parse(payload.BillingAddressID); err == nil {
				billingAddressID = &parsed
			}
		}

		var custID, contID *uuid.UUID
		if payload.CustomerID != "" {
			if parsed, err := uuid.Parse(payload.CustomerID); err == nil {
				custID = &parsed
			}
		}
		if payload.ContactID != "" {
			if parsed, err := uuid.Parse(payload.ContactID); err == nil {
				contID = &parsed
			}
		}

		var serviceCategoryID *uuid.UUID
		if payload.ServiceCategoryID != "" {
			parsed, err := uuid.Parse(payload.ServiceCategoryID)
			if err == nil {
				serviceCategoryID = &parsed
			}
		}

		rm := domain.WorkOrderRM{
			ID:                id,
			OrganizationID:    orgID,
			RequestID:         requestID,
			EstimateID:        estimateID,
			ServiceCategoryID: serviceCategoryID,
			Summary:           payload.Summary,
			Priority:          payload.Priority,
			Type:              payload.Type,
			DueDate:           payload.DueDate,
			Status:            payload.Status,
			BillingStatus:     payload.BillingStatus,
			AssetID:           assetID,
			RequiredSkillID:   requiredSkillID,
			CustomerID:        custID,
			ContactID:         contID,
			ServiceAddressID:  serviceAddressID,
			BillingAddressID:  billingAddressID,
			PreferredDate1:    payload.PreferredDate1,
			PreferredDate2:    payload.PreferredDate2,
			PreferredTime:     payload.PreferredTime,
			PreferenceNote:    payload.PreferenceNote,
			SubTotal:          payload.SubTotal,
			Discount:          payload.Discount,
			Adjustment:        payload.Adjustment,
			GrandTotal:        payload.GrandTotal,
			CreatedAt:         event.Metadata.OccurredAt,
			UpdatedAt:         event.Metadata.OccurredAt,
			SyncedAt:          event.Metadata.OccurredAt,
		}

		if err := tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error; err != nil {
			return err
		}

		// Handle service lines
		for _, line := range payload.ServiceLines {
			lineID, _ := uuid.Parse(line.ID)
			var serviceID *uuid.UUID
			if line.ServiceID != nil {
				parsed, _ := uuid.Parse(*line.ServiceID)
				serviceID = &parsed
			}
			sl := domain.WorkOrderServiceLineRM{
				ID:          lineID,
				WorkOrderID: id,
				ServiceID:   serviceID,
				Description: line.Description,
				Quantity:    line.Quantity,
				Unit:        line.Unit,
				ListPrice:   line.ListPrice,
				LineAmount:  line.LineAmount,
				CreatedAt:   event.Metadata.OccurredAt,
				UpdatedAt:   event.Metadata.OccurredAt,
				SyncedAt:    event.Metadata.OccurredAt,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&sl).Error; err != nil {
				return err
			}
		}

		// Handle part lines
		for _, line := range payload.PartLines {
			lineID, _ := uuid.Parse(line.ID)
			var partID *uuid.UUID
			if line.PartID != nil {
				parsed, _ := uuid.Parse(*line.PartID)
				partID = &parsed
			}
			pl := domain.WorkOrderPartLineRM{
				ID:          lineID,
				WorkOrderID: id,
				PartID:      partID,
				Description: line.Description,
				Quantity:    line.Quantity,
				Unit:        line.Unit,
				ListPrice:   line.ListPrice,
				LineAmount:  line.LineAmount,
				CreatedAt:   event.Metadata.OccurredAt,
				UpdatedAt:   event.Metadata.OccurredAt,
				SyncedAt:    event.Metadata.OccurredAt,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&pl).Error; err != nil {
				return err
			}
		}

		return nil

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
		if payload.Priority != "" {
			updates["priority"] = payload.Priority
		}
		if payload.Type != "" {
			updates["type"] = payload.Type
		}
		if payload.DueDate != nil {
			updates["due_date"] = payload.DueDate
		}
		if payload.Status != "" {
			updates["status"] = payload.Status
		}
		if payload.BillingStatus != "" {
			updates["billing_status"] = payload.BillingStatus
		}
		if payload.AssetID != "" {
			if parsed, err := uuid.Parse(payload.AssetID); err == nil {
				updates["asset_id"] = &parsed
			}
		}
		if payload.RequiredSkillID != "" {
			if parsed, err := uuid.Parse(payload.RequiredSkillID); err == nil {
				updates["required_skill_id"] = &parsed
			}
		}
		if payload.CustomerID != "" {
			if parsed, err := uuid.Parse(payload.CustomerID); err == nil {
				updates["customer_id"] = &parsed
			}
		}
		if payload.ContactID != "" {
			if parsed, err := uuid.Parse(payload.ContactID); err == nil {
				updates["contact_id"] = &parsed
			}
		}
		if payload.ServiceAddressID != "" {
			if parsed, err := uuid.Parse(payload.ServiceAddressID); err == nil {
				updates["service_address_id"] = &parsed
			}
		}
		if payload.BillingAddressID != "" {
			if parsed, err := uuid.Parse(payload.BillingAddressID); err == nil {
				updates["billing_address_id"] = &parsed
			}
		}
		if payload.PreferredDate1 != nil {
			updates["preferred_date1"] = payload.PreferredDate1
		}
		if payload.PreferredDate2 != nil {
			updates["preferred_date2"] = payload.PreferredDate2
		}
		if payload.PreferredTime != "" {
			updates["preferred_time"] = payload.PreferredTime
		}
		if payload.PreferenceNote != "" {
			updates["preference_note"] = payload.PreferenceNote
		}
		if payload.SubTotal > 0 {
			updates["sub_total"] = payload.SubTotal
		}
		if payload.Discount > 0 {
			updates["discount"] = payload.Discount
		}
		if payload.Adjustment > 0 {
			updates["adjustment"] = payload.Adjustment
		}
		if payload.GrandTotal > 0 {
			updates["grand_total"] = payload.GrandTotal
		}
		if payload.ServiceCategoryID != "" {
			parsed, err := uuid.Parse(payload.ServiceCategoryID)
			if err == nil {
				updates["service_category_id"] = &parsed
			}
		}
		updates["updated_at"] = event.Metadata.OccurredAt
		updates["synced_at"] = event.Metadata.OccurredAt

		if err := tx.Model(&domain.WorkOrderRM{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}

		// If line items are provided, replace existing ones
		if len(payload.ServiceLines) > 0 {
			if err := tx.Where("work_order_id = ?", id).Delete(&domain.WorkOrderServiceLineRM{}).Error; err != nil {
				return err
			}
			for _, line := range payload.ServiceLines {
				lineID, _ := uuid.Parse(line.ID)
				var serviceID *uuid.UUID
				if line.ServiceID != nil {
					parsed, _ := uuid.Parse(*line.ServiceID)
					serviceID = &parsed
				}
				sl := domain.WorkOrderServiceLineRM{
					ID:          lineID,
					WorkOrderID: id,
					ServiceID:   serviceID,
					Description: line.Description,
					Quantity:    line.Quantity,
					Unit:        line.Unit,
					ListPrice:   line.ListPrice,
					LineAmount:  line.LineAmount,
					CreatedAt:   event.Metadata.OccurredAt,
					UpdatedAt:   event.Metadata.OccurredAt,
					SyncedAt:    event.Metadata.OccurredAt,
				}
				if err := tx.Create(&sl).Error; err != nil {
					return err
				}
			}
		}

		if len(payload.PartLines) > 0 {
			if err := tx.Where("work_order_id = ?", id).Delete(&domain.WorkOrderPartLineRM{}).Error; err != nil {
				return err
			}
			for _, line := range payload.PartLines {
				lineID, _ := uuid.Parse(line.ID)
				var partID *uuid.UUID
				if line.PartID != nil {
					parsed, _ := uuid.Parse(*line.PartID)
					partID = &parsed
				}
				pl := domain.WorkOrderPartLineRM{
					ID:          lineID,
					WorkOrderID: id,
					PartID:      partID,
					Description: line.Description,
					Quantity:    line.Quantity,
					Unit:        line.Unit,
					ListPrice:   line.ListPrice,
					LineAmount:  line.LineAmount,
					CreatedAt:   event.Metadata.OccurredAt,
					UpdatedAt:   event.Metadata.OccurredAt,
					SyncedAt:    event.Metadata.OccurredAt,
				}
				if err := tx.Create(&pl).Error; err != nil {
					return err
				}
			}
		}

		return nil

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

		var brandID, categoryID, createdBy *string
		if payload.BrandID != nil && *payload.BrandID != "" {
			brandID = payload.BrandID
		}
		if payload.CategoryID != "" {
			categoryID = &payload.CategoryID
		}
		if payload.CreatedBy != "" {
			createdBy = &payload.CreatedBy
		}

		rm := domain.ItemRM{
			ID:               itemID,
			OrganizationID:   orgID,
			SKU:              payload.SKU,
			Name:             payload.Name,
			Description:      payload.Description,
			Type:             payload.Type,
			Status:           payload.Status,
			UnitID:           payload.UnitID,
			Dimensions:       domain.JSONB(payload.Dimensions),
			Weight:           domain.JSONB(payload.Weight),
			ManufacturerID:   payload.ManufacturerID,
			BrandID:          brandID,
			BarCode:          payload.BarCode,
			UPC:              payload.UPC,
			EAN:              payload.EAN,
			ISBN:             payload.ISBN,
			MPN:              payload.MPN,
			SalesInfo:        domain.JSONB(payload.SalesInfo),
			PurchaseInfo:     domain.JSONB(payload.PurchaseInfo),
			InventoryInfo:    domain.JSONB(payload.InventoryInfo),
			ServiceInfo:      domain.JSONB(payload.ServiceInfo),
			CRMProductInfo:   domain.JSONB(payload.CRMProductInfo),
			CRMFields:        domain.JSONB(payload.CRMFields),
			CRMServiceFields: domain.JSONB(payload.CRMServiceFields),
			CreatedBy:        createdBy,
			CreatedAt:        event.Metadata.OccurredAt,
			UpdatedAt:        event.Metadata.OccurredAt,
			CategoryID:       categoryID,
			SyncedAt:         event.Metadata.OccurredAt,
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

		if payload.SKU != "" || isUpdated("sku") {
			updates["sku"] = payload.SKU
		}
		if payload.Name != "" || isUpdated("name") {
			updates["name"] = payload.Name
		}
		if payload.Description != "" || isUpdated("description") {
			updates["description"] = payload.Description
		}
		if payload.Type != "" || isUpdated("type") {
			updates["type"] = payload.Type
		}
		if payload.Status != "" || isUpdated("status") {
			updates["status"] = payload.Status
		}
		if payload.UnitID != nil || isUpdated("unit_id") {
			updates["unit_id"] = payload.UnitID
		}
		if payload.Dimensions != nil || isUpdated("dimensions") {
			updates["dimensions"] = domain.JSONB(payload.Dimensions)
		}
		if payload.Weight != nil || isUpdated("weight") {
			updates["weight"] = domain.JSONB(payload.Weight)
		}
		if payload.ManufacturerID != nil || isUpdated("manufacturer_id") {
			updates["manufacturer_id"] = payload.ManufacturerID
		}
		if payload.BrandID != nil || isUpdated("brand_id") {
			if payload.BrandID != nil && *payload.BrandID != "" {
				updates["brand_id"] = payload.BrandID
			} else {
				updates["brand_id"] = nil
			}
		}
		if payload.BarCode != "" || isUpdated("bar_code") {
			updates["bar_code"] = payload.BarCode
		}
		if payload.UPC != "" || isUpdated("upc") {
			updates["upc"] = payload.UPC
		}
		if payload.EAN != "" || isUpdated("ean") {
			updates["ean"] = payload.EAN
		}
		if payload.ISBN != "" || isUpdated("isbn") {
			updates["isbn"] = payload.ISBN
		}
		if payload.MPN != "" || isUpdated("mpn") {
			updates["mpn"] = payload.MPN
		}
		if payload.SalesInfo != nil || isUpdated("sales_info") {
			updates["sales_info"] = domain.JSONB(payload.SalesInfo)
		}
		if payload.PurchaseInfo != nil || isUpdated("purchase_info") {
			updates["purchase_info"] = domain.JSONB(payload.PurchaseInfo)
		}
		if payload.InventoryInfo != nil || isUpdated("inventory_info") {
			updates["inventory_info"] = domain.JSONB(payload.InventoryInfo)
		}
		if payload.ServiceInfo != nil || isUpdated("service_info") {
			updates["service_info"] = domain.JSONB(payload.ServiceInfo)
		}
		if payload.CRMProductInfo != nil || isUpdated("crm_product_info") {
			updates["crm_product_info"] = domain.JSONB(payload.CRMProductInfo)
		}
		if payload.CRMFields != nil || isUpdated("crm_fields") {
			updates["crm_fields"] = domain.JSONB(payload.CRMFields)
		}
		if payload.CRMServiceFields != nil || isUpdated("crm_service_fields") {
			updates["crm_service_fields"] = domain.JSONB(payload.CRMServiceFields)
		}
		if payload.CategoryID != "" || isUpdated("category_id") {
			if payload.CategoryID != "" {
				updates["category_id"] = &payload.CategoryID
			} else {
				updates["category_id"] = nil
			}
		}
		if payload.UpdatedBy != "" {
			updates["updated_by"] = &payload.UpdatedBy
		}

		updates["updated_at"] = event.Metadata.OccurredAt
		updates["synced_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.ItemRM{}).Where("id = ?", itemID).Updates(updates).Error

	case shared_events.ItemDeleted:
		var payload shared_events.ItemDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}
		id, _ := uuid.Parse(payload.ItemID)
		now := event.Metadata.OccurredAt
		return tx.Model(&domain.ItemRM{}).Where("id = ?", id).Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": now,
			"synced_at":  now,
		}).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleOrganizationEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.OrganizationCreated:
		var payload shared_events.OrganizationCreatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, _ := uuid.Parse(payload.OrganizationID)

		rm := domain.OrganizationRM{
			ID:               id,
			ProfileID:        payload.ProfileID,
			OrganizationName: payload.Name,
			OrganizationType: payload.OrganizationType,
			Address:          payload.Address,
			City:             payload.City,
			State:            payload.State,
			ZipCode:          payload.ZipCode,
			Country:          payload.Country,
			Phone:            payload.Phone,
			Website:          payload.Domain,
			Currency:         payload.Currency,
			Timezone:         payload.Timezone,
			BusinessCategory: payload.BusinessCategory,
			Language:         payload.Language,
			CollectsTax:      payload.CollectsTax,
			GSTIN:            payload.GSTIN,
			IsActive:         payload.Status == "active",
			CreatedAt:        event.Metadata.OccurredAt,
			UpdatedAt:        event.Metadata.OccurredAt,
			SyncedAt:         event.Metadata.OccurredAt,
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.OrganizationUpdated:
		var payload shared_events.OrganizationUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, _ := uuid.Parse(payload.OrganizationID)

		updates := make(map[string]interface{})

		isUpdated := func(field string) bool {
			for _, f := range payload.UpdatedFields {
				if f == field {
					return true
				}
			}
			return false
		}

		if payload.Name != "" || isUpdated("name") {
			updates["organization_name"] = payload.Name
		}
		if payload.ProfileID != "" || isUpdated("profile_id") {
			updates["profile_id"] = payload.ProfileID
		}
		if payload.OrganizationType != "" || isUpdated("organization_type") {
			updates["organization_type"] = payload.OrganizationType
		}
		if payload.BusinessCategory != "" || isUpdated("business_category") {
			updates["business_category"] = payload.BusinessCategory
		}
		if payload.Language != "" || isUpdated("language") {
			updates["language"] = payload.Language
		}
		updates["collects_tax"] = payload.CollectsTax
		if payload.GSTIN != "" || isUpdated("gstin") {
			updates["gstin"] = payload.GSTIN
		}
		if payload.Address != "" || isUpdated("address") {
			updates["address"] = payload.Address
		}
		if payload.City != "" || isUpdated("city") {
			updates["city"] = payload.City
		}
		if payload.State != "" || isUpdated("state") {
			updates["state"] = payload.State
		}
		if payload.ZipCode != "" || isUpdated("zip_code") {
			updates["zip_code"] = payload.ZipCode
		}
		if payload.Country != "" || isUpdated("country") {
			updates["country"] = payload.Country
		}
		if payload.Phone != "" || isUpdated("phone") {
			updates["phone"] = payload.Phone
		}
		if payload.Domain != "" || isUpdated("domain") {
			updates["website"] = payload.Domain
		}
		if payload.Currency != "" || isUpdated("currency") {
			updates["currency"] = payload.Currency
		}
		if payload.Timezone != "" || isUpdated("timezone") {
			updates["timezone"] = payload.Timezone
		}
		if payload.Status != "" || isUpdated("status") {
			updates["is_active"] = payload.Status == "active"
		}

		updates["updated_at"] = event.Metadata.OccurredAt
		updates["synced_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.OrganizationRM{}).Where("id = ?", id).Updates(updates).Error

	case shared_events.OrganizationDeleted:
		var payload shared_events.OrganizationDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}
		id, _ := uuid.Parse(payload.OrganizationID)
		return tx.Delete(&domain.OrganizationRM{}, "id = ?", id).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleAddressEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.AddressCreated:
		var payload shared_events.AddressCreatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		addressID, _ := uuid.Parse(payload.AddressID)
		orgID, _ := uuid.Parse(payload.OrganizationID)
		
		var customerID *uuid.UUID
		if payload.CompanyID != nil && *payload.CompanyID != "" {
			if parsed, err := uuid.Parse(*payload.CompanyID); err == nil {
				customerID = &parsed
			}
		}

		var contactID *uuid.UUID
		if payload.ContactID != nil && *payload.ContactID != "" {
			if parsed, err := uuid.Parse(*payload.ContactID); err == nil {
				contactID = &parsed
			}
		}

		attention := payload.Attention
		if attention == "" && payload.Name != "" {
			attention = payload.Name
		}

		rm := domain.AddressReadOnly{
			ID:              addressID,
			OrganizationID:  orgID,
			CustomerID:      customerID,
			ContactID:       contactID,
			Attention:       attention,
			Type:            payload.AddressType,
			Street1:         payload.Street1,
			Street2:         payload.Street2,
			City:            payload.City,
			State:           payload.State,
			PostalCode:      payload.PostalCode,
			Country:         payload.Country,
			Phone:           payload.Phone,
			Fax:             payload.Fax,
			IsDefault:       payload.IsDefault,
			IsPrimary:       payload.IsDefault,
			Territory:       payload.Territory,
			Latitude:        payload.Latitude,
			Longitude:       payload.Longitude,
			NormalizedHash:  payload.NormalizedHash,
			GeocodingStatus: payload.GeocodingStatus,
			Status:          "active",
			SyncedAt:        event.Metadata.OccurredAt,
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.AddressUpdated:
		var payload shared_events.AddressUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		addressID, _ := uuid.Parse(payload.AddressID)

		updates := make(map[string]interface{})

		isUpdated := func(field string) bool {
			for _, f := range payload.UpdatedFields {
				if f == field {
					return true
				}
			}
			return false
		}

		if payload.AddressType != "" || isUpdated("address_type") {
			updates["type"] = payload.AddressType
		}
		
		attention := payload.Attention
		if attention == "" && payload.Name != "" {
			attention = payload.Name
		}
		if attention != "" || isUpdated("attention") || isUpdated("name") {
			updates["attention"] = attention
		}

		if payload.Street1 != "" || isUpdated("street1") {
			updates["street1"] = payload.Street1
		}
		if payload.Street2 != "" || isUpdated("street2") {
			updates["street2"] = payload.Street2
		}
		if payload.City != "" || isUpdated("city") {
			updates["city"] = payload.City
		}
		if payload.State != "" || isUpdated("state") {
			updates["state"] = payload.State
		}
		if payload.PostalCode != "" || isUpdated("postal_code") {
			updates["postal_code"] = payload.PostalCode
		}
		if payload.Country != "" || isUpdated("country") {
			updates["country"] = payload.Country
		}
		if payload.Phone != "" || isUpdated("phone") {
			updates["phone"] = payload.Phone
		}
		if payload.Fax != "" || isUpdated("fax") {
			updates["fax"] = payload.Fax
		}
		if payload.Territory != "" || isUpdated("territory") {
			updates["territory"] = payload.Territory
		}
		if payload.Latitude != nil || isUpdated("latitude") {
			updates["latitude"] = payload.Latitude
		}
		if payload.Longitude != nil || isUpdated("longitude") {
			updates["longitude"] = payload.Longitude
		}
		if payload.NormalizedHash != "" || isUpdated("normalized_hash") {
			updates["normalized_hash"] = payload.NormalizedHash
		}
		if payload.GeocodingStatus != "" || isUpdated("geocoding_status") {
			updates["geocoding_status"] = payload.GeocodingStatus
		}

		if payload.CompanyID != nil {
			if *payload.CompanyID == "" {
				updates["customer_id"] = nil
			} else if parsed, err := uuid.Parse(*payload.CompanyID); err == nil {
				updates["customer_id"] = &parsed
			}
		}
		if payload.ContactID != nil {
			if *payload.ContactID == "" {
				updates["contact_id"] = nil
			} else if parsed, err := uuid.Parse(*payload.ContactID); err == nil {
				updates["contact_id"] = &parsed
			}
		}
		if payload.IsDefault != nil {
			updates["is_default"] = *payload.IsDefault
			updates["is_primary"] = *payload.IsDefault
		}

		updates["synced_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.AddressReadOnly{}).Where("id = ?", addressID).Updates(updates).Error

	case shared_events.AddressDeleted:
		var payload shared_events.AddressDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		addressID, _ := uuid.Parse(payload.AddressID)
		return tx.Delete(&domain.AddressReadOnly{}, "id = ?", addressID).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleServiceCategoryEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.ServiceCategoryCreated:
		var payload shared_events.ServiceCategoryCreatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, _ := uuid.Parse(payload.ID)
		orgID, _ := uuid.Parse(payload.OrganizationID)

		createdAt := payload.CreatedAt
		if createdAt.IsZero() {
			createdAt = event.Metadata.OccurredAt
		}

		rm := domain.ServiceCategoryReadOnly{
			ID:             id,
			OrganizationID: orgID,
			CategoryName:   payload.CategoryName,
			CategoryCode:   payload.CategoryCode,
			Description:    payload.Description,
			Type:           payload.Type,
			ImagePath:      payload.ImagePath,
			Status:         payload.Status,
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt,
			SyncedAt:       event.Metadata.OccurredAt,
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.ServiceCategoryUpdated:
		var payload shared_events.ServiceCategoryUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, _ := uuid.Parse(payload.ID)

		updates := make(map[string]interface{})

		isUpdated := func(field string) bool {
			for _, f := range payload.UpdatedFields {
				if f == field {
					return true
				}
			}
			return false
		}

		if payload.CategoryName != "" || isUpdated("category_name") {
			updates["category_name"] = payload.CategoryName
		}
		if payload.CategoryCode != "" || isUpdated("category_code") {
			updates["category_code"] = payload.CategoryCode
		}
		if payload.Description != "" || isUpdated("description") {
			updates["description"] = payload.Description
		}
		if payload.Type != "" || isUpdated("type") {
			updates["type"] = payload.Type
		}
		if payload.ImagePath != "" || isUpdated("image_path") {
			updates["image_path"] = payload.ImagePath
		}
		if payload.Status != "" || isUpdated("status") {
			updates["status"] = payload.Status
		}

		updatedAt := payload.UpdatedAt
		if updatedAt.IsZero() {
			updatedAt = event.Metadata.OccurredAt
		}
		updates["updated_at"] = updatedAt
		updates["synced_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.ServiceCategoryReadOnly{}).Where("id = ?", id).Updates(updates).Error

	case shared_events.ServiceCategoryDeleted:
		var payload shared_events.ServiceCategoryDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, _ := uuid.Parse(payload.ID)
		now := event.Metadata.OccurredAt
		return tx.Model(&domain.ServiceCategoryReadOnly{}).Where("id = ?", id).Updates(map[string]interface{}{
			"deleted_at": &now,
			"synced_at":  now,
		}).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleAppointmentEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.AppointmentCreated,
		shared_events.AppointmentUpdated,
		shared_events.AppointmentAssigned,
		shared_events.AppointmentStarted,
		shared_events.AppointmentCompleted,
		shared_events.AppointmentCancelled,
		shared_events.AppointmentTerminated,
		shared_events.AppointmentRescheduled:

		var payload struct {
			shared_events.AppointmentPayload
		}
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, err := uuid.Parse(payload.AppointmentID)
		if err != nil {
			return fmt.Errorf("invalid appointment ID %s: %w", payload.AppointmentID, err)
		}
		orgID, err := uuid.Parse(payload.OrganizationID)
		if err != nil {
			return fmt.Errorf("invalid organization ID %s: %w", payload.OrganizationID, err)
		}
		woID, err := uuid.Parse(payload.WorkOrderID)
		if err != nil {
			return fmt.Errorf("invalid work order ID %s: %w", payload.WorkOrderID, err)
		}

		var customerID *uuid.UUID // not in Kafka payload

		var assignedTechnicianID *uuid.UUID
		if payload.AssignedTechnicianID != nil && *payload.AssignedTechnicianID != "" {
			if parsed, err := uuid.Parse(*payload.AssignedTechnicianID); err == nil {
				assignedTechnicianID = &parsed
			}
		}

		var assignedCrewID *uuid.UUID
		if payload.AssignedCrewID != nil && *payload.AssignedCrewID != "" {
			if parsed, err := uuid.Parse(*payload.AssignedCrewID); err == nil {
				assignedCrewID = &parsed
			}
		}

		var serviceAddressID *uuid.UUID
		if payload.ServiceAddressID != nil && *payload.ServiceAddressID != "" {
			if parsed, err := uuid.Parse(*payload.ServiceAddressID); err == nil {
				serviceAddressID = &parsed
			}
		}

		var billingAddressID *uuid.UUID
		if payload.BillingAddressID != nil && *payload.BillingAddressID != "" {
			if parsed, err := uuid.Parse(*payload.BillingAddressID); err == nil {
				billingAddressID = &parsed
			}
		}

		rm := domain.ServiceAppointmentRM{
			ID:                   id,
			OrganizationID:       orgID,
			WorkOrderID:          woID,
			CustomerID:           customerID,
			AppointmentNumber:    payload.AppointmentNumber,
			Subject:              "", // not in Kafka payload
			ScheduledDate:        payload.ScheduledDate,
			ScheduledTime:        "", // not in Kafka payload
			ScheduledStartTime:   payload.ScheduledStartTime,
			ScheduledEndTime:     payload.ScheduledEndTime,
			Duration:             payload.Duration,
			Status:               payload.Status,
			ActualStartTime:      payload.ActualStartTime,
			ActualEndTime:        payload.ActualEndTime,
			StartLatitude:        nil, // not in Kafka payload
			StartLongitude:       nil, // not in Kafka payload
			EndLatitude:          nil, // not in Kafka payload
			EndLongitude:         nil, // not in Kafka payload
			AssignedTechnicianID: assignedTechnicianID,
			AssignedCrewID:       assignedCrewID,
			ServiceAddressID:     serviceAddressID,
			BillingAddressID:     billingAddressID,
			Notes:                payload.Notes,
			CreatedAt:            event.Metadata.OccurredAt,
			UpdatedAt:            event.Metadata.OccurredAt,
			SyncedAt:             event.Metadata.OccurredAt,
		}

		if err := tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error; err != nil {
			return err
		}

		// Delete existing resources first (hard delete to cleanly recreate)
		if err := tx.Where("service_appointment_id = ?", id).Delete(&domain.ServiceAppointmentResourceRM{}).Error; err != nil {
			return err
		}

		// Insert new resources if any
		if len(payload.Resources) > 0 {
			for _, res := range payload.Resources {
				resID, err := uuid.Parse(res.ID)
				if err != nil {
					continue
				}
				resOrgID, _ := uuid.Parse(res.OrganizationID)
				resApptID, _ := uuid.Parse(res.ServiceAppointmentID)
				resourceID, err := uuid.Parse(res.ResourceID)
				if err != nil {
					continue
				}

				sar := domain.ServiceAppointmentResourceRM{
					ID:                   resID,
					OrganizationID:       resOrgID,
					ServiceAppointmentID: resApptID,
					ResourceType:         res.ResourceType,
					ResourceID:           resourceID,
					StartTime:            res.StartTime,
					EndTime:              res.EndTime,
					CreatedAt:            event.Metadata.OccurredAt,
					UpdatedAt:            event.Metadata.OccurredAt,
					SyncedAt:             event.Metadata.OccurredAt,
				}
				if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&sar).Error; err != nil {
					return err
				}
			}
		}

		return nil

	case shared_events.AppointmentDeleted:
		var payload shared_events.AppointmentDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		id, err := uuid.Parse(payload.AppointmentID)
		if err != nil {
			return fmt.Errorf("invalid deleted appointment ID %s: %w", payload.AppointmentID, err)
		}

		// Delete resources first
		if err := tx.Where("service_appointment_id = ?", id).Delete(&domain.ServiceAppointmentResourceRM{}).Error; err != nil {
			return err
		}

		// Soft delete appointment
		now := event.Metadata.OccurredAt
		return tx.Model(&domain.ServiceAppointmentRM{}).Where("id = ?", id).Updates(map[string]interface{}{
			"deleted_at": &now,
			"synced_at":  now,
		}).Error

	default:
		return nil
	}
}

func (h *EventHandler) handleUserEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case shared_events.UserRegistered:
		var payload shared_events.UserRegisteredPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		var emailVerified bool
		if payload.IsActive {
			emailVerified = true
		}

		rm := domain.UserReadOnly{
			ID:               payload.UserID,
			Email:            payload.Email,
			FirstName:        payload.FirstName,
			LastName:         payload.LastName,
			FullName:         payload.FullName,
			Role:             payload.Role,
			OrganizationName: payload.OrganizationName,
			UserType:         payload.UserType,
			ProfilePhotoURL:  payload.ProfilePhotoURL,
			EmailVerified:    &emailVerified,
			IsActive:         &payload.IsActive,
			CreatedAt:        payload.CreatedAt,
			UpdatedAt:        payload.UpdatedAt,
			SyncedAt:         event.Metadata.OccurredAt,
		}

		if payload.OrganizationID != "" {
			rm.OrganizationID = &payload.OrganizationID
		}
		if payload.ProfileID != "" {
			rm.ProfileID = &payload.ProfileID
		}
		if payload.EmployeeID != "" {
			rm.EmployeeID = &payload.EmployeeID
		}
		if payload.WorkforceUserID != "" {
			rm.WorkforceUserID = &payload.WorkforceUserID
		}
		if payload.CustomerID != "" {
			rm.CustomerID = &payload.CustomerID
		}
		if payload.PhoneNumber != "" {
			rm.PhoneNumber = &payload.PhoneNumber
		}

		return tx.Clauses(clause.OnConflict{
			UpdateAll: true,
		}).Create(&rm).Error

	case shared_events.UserUpdated:
		var payload shared_events.UserUpdatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		updates := make(map[string]interface{})
		isUpdated := func(field string) bool {
			for _, f := range payload.UpdatedFields {
				if f == field {
					return true
				}
			}
			return false
		}

		if payload.Email != "" || isUpdated("email") {
			updates["email"] = payload.Email
		}
		if payload.FirstName != "" || isUpdated("first_name") {
			updates["first_name"] = payload.FirstName
		}
		if payload.LastName != "" || isUpdated("last_name") {
			updates["last_name"] = payload.LastName
		}
		if payload.FullName != "" || isUpdated("full_name") {
			updates["full_name"] = payload.FullName
		}
		if payload.Role != "" || isUpdated("role") {
			updates["role"] = payload.Role
		}
		if payload.OrganizationID != "" || isUpdated("organization_id") {
			updates["organization_id"] = &payload.OrganizationID
		}
		if payload.OrganizationName != "" || isUpdated("organization_name") {
			updates["organization_name"] = payload.OrganizationName
		}
		if payload.ProfileID != "" || isUpdated("profile_id") {
			updates["profile_id"] = &payload.ProfileID
		}
		if payload.EmployeeID != "" || isUpdated("employee_id") {
			updates["employee_id"] = &payload.EmployeeID
		}
		if payload.WorkforceUserID != "" || isUpdated("workforce_user_id") {
			updates["workforce_user_id"] = &payload.WorkforceUserID
		}
		if payload.CustomerID != "" || isUpdated("customer_id") {
			updates["customer_id"] = &payload.CustomerID
		}
		if payload.UserType != "" || isUpdated("user_type") {
			updates["user_type"] = payload.UserType
		}
		if payload.ProfilePhotoURL != "" || isUpdated("profile_photo_url") {
			updates["profile_photo_url"] = payload.ProfilePhotoURL
		}
		if payload.PhoneNumber != "" || isUpdated("phone_number") {
			updates["phone_number"] = &payload.PhoneNumber
		}
		if payload.IsActive != nil || isUpdated("is_active") {
			updates["is_active"] = payload.IsActive
		}

		updates["updated_at"] = event.Metadata.OccurredAt
		updates["synced_at"] = event.Metadata.OccurredAt

		return tx.Model(&domain.UserReadOnly{}).Where("id = ?", payload.UserID).Updates(updates).Error

	case shared_events.UserActivated:
		var payload shared_events.UserActivatedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		active := true
		updates := map[string]interface{}{
			"is_active":  &active,
			"updated_at": event.Metadata.OccurredAt,
			"synced_at":  event.Metadata.OccurredAt,
		}
		return tx.Model(&domain.UserReadOnly{}).Where("id = ?", payload.UserID).Updates(updates).Error

	case shared_events.UserDeactivated:
		var payload struct {
			UserID string `json:"user_id"`
		}
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		active := false
		updates := map[string]interface{}{
			"is_active":  &active,
			"updated_at": event.Metadata.OccurredAt,
			"synced_at":  event.Metadata.OccurredAt,
		}
		return tx.Model(&domain.UserReadOnly{}).Where("id = ?", payload.UserID).Updates(updates).Error

	case shared_events.UserDeleted:
		var payload shared_events.UserDeletedPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		now := event.Metadata.OccurredAt
		return tx.Model(&domain.UserReadOnly{}).Where("id = ?", payload.UserID).Updates(map[string]interface{}{
			"deleted_at": &now,
			"synced_at":  now,
			"updated_at": now,
		}).Error

	default:
		return nil
	}
}

func (h *EventHandler) autoGenerateInvoice(ctx context.Context, workOrderIDStr string, orgIDStr string) error {
	woID, err := uuid.Parse(workOrderIDStr)
	if err != nil {
		return fmt.Errorf("invalid work order UUID %s: %w", workOrderIDStr, err)
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return fmt.Errorf("invalid organization UUID %s: %w", orgIDStr, err)
	}

	// 1. Check if invoice already exists to prevent double invoicing
	var count int64
	err = h.db.WithContext(ctx).Model(&domain.Invoice{}).
		Where("organization_id = ? AND source_system = ? AND source_reference_id = ?", orgID, domain.SourceSystemFSM, workOrderIDStr).
		Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to check existing invoice: %w", err)
	}

	if count > 0 {
		log.Printf("[AutoInvoice] Invoice already exists for work order %s. Skipping auto-generation.", workOrderIDStr)
		return nil
	}

	// 2. Fetch work order details from read model (replicated data)
	var wo domain.WorkOrderRM
	err = h.db.WithContext(ctx).
		Preload("Customer").
		Preload("ServiceLines").
		Preload("PartLines").
		First(&wo, "id = ?", woID).Error
	if err != nil {
		return fmt.Errorf("failed to fetch work order read model %s: %w", woID, err)
	}

	// Debug: confirm service_category_id is loaded from DB
	if wo.ServiceCategoryID != nil {
		log.Printf("[AutoInvoice] wo.ServiceCategoryID = %s for work order %s", wo.ServiceCategoryID.String(), workOrderIDStr)
	} else {
		log.Printf("[AutoInvoice] wo.ServiceCategoryID is NIL for work order %s", workOrderIDStr)
	}

	// Check if customer ID is present
	if wo.CustomerID == nil {
		return fmt.Errorf("work order %s has no customer ID", woID)
	}

	// Fetch customer if not preloaded (fallback)
	if wo.Customer == nil {
		var customer domain.CustomerRM
		if err := h.db.WithContext(ctx).First(&customer, "id = ?", *wo.CustomerID).Error; err == nil {
			wo.Customer = &customer
		}
	}

	// 3. Set customer-specific currency and payment terms
	currency := "INR"
	paymentTerms := "Due on Receipt"
	if wo.Customer != nil {
		if wo.Customer.CurrencyCode != "" {
			currency = wo.Customer.CurrencyCode
		}
		if wo.Customer.PaymentTerms != "" {
			paymentTerms = wo.Customer.PaymentTerms
		}
	}

	// 4. Map work order details to CreateInvoiceRequest DTO
	req := dto.CreateInvoiceRequest{
		SourceSystem:      string(domain.SourceSystemFSM),
		SourceReferenceID: &workOrderIDStr,
		Subject:           "Invoice for Work Order: " + wo.Summary,
		CustomerID:        *wo.CustomerID,
		ContactID:         wo.ContactID,
		ServiceAddressID:  wo.ServiceAddressID,
		BillingAddressID:  wo.BillingAddressID,
		ServiceCategoryID: wo.ServiceCategoryID,
		InvoiceDate:       time.Now().UTC(),
		DueDate:           time.Now().UTC().AddDate(0, 0, 30), // Default Terms: Net 30
		Currency:          currency,
		PaymentTerms:      paymentTerms,
		Adjustment:        wo.Adjustment - wo.Discount,
		Items:             make([]dto.CreateInvoiceItem, 0),
	}

	// Override due date if specified in work order
	if wo.DueDate != nil {
		req.DueDate = *wo.DueDate
	}

	// Add service lines
	for _, line := range wo.ServiceLines {
		itemID := uuid.Nil
		if line.ServiceID != nil {
			itemID = *line.ServiceID
		} else {
			itemID = line.ID
		}

		req.Items = append(req.Items, dto.CreateInvoiceItem{
			ItemID:            itemID,
			ItemType:          "service",
			Name:              line.Description,
			Description:       line.Description,
			Quantity:          line.Quantity,
			UnitPrice:         line.ListPrice,
			ServiceCategoryID: wo.ServiceCategoryID,
		})
	}

	// Add part lines
	for _, line := range wo.PartLines {
		itemID := uuid.Nil
		if line.PartID != nil {
			itemID = *line.PartID
		} else {
			itemID = line.ID
		}

		req.Items = append(req.Items, dto.CreateInvoiceItem{
			ItemID:            itemID,
			ItemType:          "part",
			Name:              line.Description,
			Description:       line.Description,
			Quantity:          line.Quantity,
			UnitPrice:         line.ListPrice,
			ServiceCategoryID: wo.ServiceCategoryID,
		})
	}

	// 5. Fallback check: If work order contains no items, create a generic item representing work order total
	if len(req.Items) == 0 {
		req.Items = append(req.Items, dto.CreateInvoiceItem{
			ItemID:            uuid.New(),
			ItemType:          "service",
			Name:              "Service Charge (" + wo.Summary + ")",
			Description:       "Service rendered under Work Order: " + wo.Summary,
			Quantity:          1,
			UnitPrice:         wo.GrandTotal,
			ServiceCategoryID: wo.ServiceCategoryID,
		})
		// Clear adjustment since it's already captured in the unit price of fallback item
		req.Adjustment = 0
	}

	// 6. Call invoiceService to create draft invoice
	if req.ServiceCategoryID != nil {
		log.Printf("[AutoInvoice] req.ServiceCategoryID = %s before CreateInvoice", req.ServiceCategoryID.String())
	} else {
		log.Printf("[AutoInvoice] req.ServiceCategoryID is NIL before CreateInvoice")
	}
	invoiceResp, err := h.invoiceService.CreateInvoice(ctx, orgID, req)
	if err != nil {
		return fmt.Errorf("failed to create draft invoice: %w", err)
	}
	log.Printf("[AutoInvoice] Created draft invoice %s for work order %s", invoiceResp.ID, workOrderIDStr)

	// 7. Call invoiceService to send invoice (transitions to SENT, generates number, builds PDF, and emails customer)
	_, err = h.invoiceService.SendInvoice(ctx, invoiceResp.ID, dto.SendInvoiceRequest{})
	if err != nil {
		return fmt.Errorf("failed to send/email invoice %s: %w", invoiceResp.ID, err)
	}
	log.Printf("[AutoInvoice] Successfully generated, finalized, and emailed invoice %s for work order %s", 
		invoiceResp.ID, workOrderIDStr)

	return nil
}
