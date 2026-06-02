package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/outbound"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InvoiceService struct {
	invoiceRepo     domain.InvoiceRepository
	rmRepo          domain.ReadModelRepository
	auditRepo       domain.AuditLogRepository
	eventPublisher  domain.EventPublisher
	pdfService      *PDFService // Added for PDF generation
	inventoryClient outbound.InventoryClient
	customerClient  outbound.CustomerClient
}

func NewInvoiceService(
	invoiceRepo domain.InvoiceRepository,
	rmRepo domain.ReadModelRepository,
	auditRepo domain.AuditLogRepository,
	eventPublisher domain.EventPublisher,
	pdfService *PDFService,
	inventoryClient outbound.InventoryClient,
	customerClient outbound.CustomerClient,
) *InvoiceService {
	return &InvoiceService{
		invoiceRepo:     invoiceRepo,
		rmRepo:          rmRepo,
		auditRepo:       auditRepo,
		eventPublisher:  eventPublisher,
		pdfService:      pdfService,
		inventoryClient: inventoryClient,
		customerClient:  customerClient,
	}
}

// CreateInvoice creates a new invoice in DRAFT status
// Invoice number is NOT generated here - it's generated when invoice is SENT
func (s *InvoiceService) CreateInvoice(ctx context.Context, orgID uuid.UUID, req dto.CreateInvoiceRequest) (*dto.InvoiceResponse, error) {
	// Parse source system
	sourceSystem := domain.SourceSystemManual
	if req.SourceSystem != "" {
		sourceSystem = domain.SourceSystem(req.SourceSystem)
	}

	ownerID := req.OwnerID
	if ownerID == nil {
		if adminUUID, err := s.rmRepo.GetOrganizationAdminID(ctx, orgID); err == nil {
			ownerID = adminUUID
		}
	}

	billingAddressID := req.BillingAddressID
	shippingAddressID := req.ShippingAddressID
	serviceAddressID := req.ServiceAddressID

	shippingExplicitlyBlank := req.ShippingStreet == "" && req.ShippingCity == "" && req.ShippingState == "" && req.ShippingCode == "" && req.ShippingCountry == ""

	var customerRM *domain.CustomerRM
	if billingAddressID == nil || shippingAddressID == nil || serviceAddressID == nil {
		if customer, err := s.rmRepo.GetCustomer(ctx, req.CustomerID); err == nil && customer != nil {
			customerRM = customer
			if billingAddressID == nil {
				billingAddressID = customer.BillingAddressID
			}
			// Skip copying customer shipping address ID if the source system is FSM (Work Order) or CRM (Estimate),
			// or if the shipping fields were explicitly left blank.
			if shippingAddressID == nil && sourceSystem != domain.SourceSystemFSM && sourceSystem != domain.SourceSystemCRM && !shippingExplicitlyBlank {
				shippingAddressID = customer.ShippingAddressID
			}
			if serviceAddressID == nil {
				serviceAddressID = customer.ServiceAddressID
			}
		}
	}

	billingStreet := req.BillingStreet
	billingCity := req.BillingCity
	billingState := req.BillingState
	billingCode := req.BillingCode
	billingCountry := req.BillingCountry
	shippingStreet := req.ShippingStreet
	shippingCity := req.ShippingCity
	shippingState := req.ShippingState
	shippingCode := req.ShippingCode
	shippingCountry := req.ShippingCountry

	if billingStreet == "" || shippingStreet == "" {
		if customerRM == nil {
			if customer, err := s.rmRepo.GetCustomer(ctx, req.CustomerID); err == nil && customer != nil {
				customerRM = customer
			}
		}
		if customerRM != nil {
			if billingStreet == "" {
				billingStreet = customerRM.BillingStreet
				billingCity = customerRM.BillingCity
				billingState = customerRM.BillingState
				billingCode = customerRM.BillingCode
				billingCountry = customerRM.BillingCountry
			}
			// Skip copying customer shipping details if the source system is FSM (Work Order) or CRM (Estimate),
			// or if the shipping fields were explicitly left blank.
			if shippingStreet == "" && sourceSystem != domain.SourceSystemFSM && sourceSystem != domain.SourceSystemCRM && !shippingExplicitlyBlank {
				shippingStreet = customerRM.ShippingStreet
				shippingCity = customerRM.ShippingCity
				shippingState = customerRM.ShippingState
				shippingCode = customerRM.ShippingCode
				shippingCountry = customerRM.ShippingCountry
			}
		}
	}

	invoiceID := uuid.New()
	invoice := &domain.Invoice{
		ID:             invoiceID,
		OrganizationID: orgID,
		CustomerID:     req.CustomerID,

		// Source tracking - makes billing source-agnostic
		SourceSystem:      sourceSystem,
		SourceReferenceID: req.SourceReferenceID,

		ContactID:       req.ContactID,
		OwnerID:         ownerID,
		Subject:         req.Subject,
		InvoiceNumber:   nil, // NOT generated on creation - only on SEND
		ReferenceNo:     req.ReferenceNo,
		PaymentTerms:    req.PaymentTerms,
		InvoiceDate:     req.InvoiceDate,
		DueDate:         req.DueDate,
		Status:          domain.InvoiceStatusDraft,
		Currency:        req.Currency,
		Adjustment:      req.Adjustment,
		ExciseDuty:      req.ExciseDuty,
		SalesCommission: req.SalesCommission,
		SalesOrderID:      req.SalesOrderID,
		SalesOrder:        req.SalesOrder,
		PurchaseOrder:     req.PurchaseOrder,
		ServiceCategoryID: req.ServiceCategoryID,
		Terms:           req.Terms,
		Notes:           req.Notes,
		BillingAddressID:  billingAddressID,
		ShippingAddressID: shippingAddressID,
		ServiceAddressID:  serviceAddressID,
		BillingStreet:   billingStreet,
		BillingCity:     billingCity,
		BillingState:    billingState,
		BillingCode:     billingCode,
		BillingCountry:  billingCountry,
		ShippingStreet:  shippingStreet,
		ShippingCity:    shippingCity,
		ShippingState:   shippingState,
		ShippingCode:    shippingCode,
		ShippingCountry: shippingCountry,
	}

	// Stock Management - DISABLED for draft invoice creation
	// Stock will be checked when invoice is sent (DRAFT → SENT)
	// This allows creating draft invoices without stock constraints
	/*
		shouldCheckStock := invoice.SalesOrderID == nil && s.inventoryClient != nil

		if shouldCheckStock {
			stockItems := make([]outbound.StockCheckItem, 0)
			for _, itemReq := range req.Items {
				stockItems = append(stockItems, outbound.StockCheckItem{
					ItemID:   itemReq.ItemID.String(),
					Quantity: int32(itemReq.Quantity),
				})
			}
			if len(stockItems) > 0 {
				unavailable, err := s.inventoryClient.CheckStockAvailability(ctx, stockItems)
				if err != nil {
					// If method is not implemented, log warning and continue
					if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
						fmt.Printf("[WARNING] Inventory service CheckStockAvailability unimplemented: %v\n", err)
					} else {
						return nil, fmt.Errorf("failed to check stock availability: %w", err)
					}
				} else if len(unavailable) > 0 {
					errMsg := "stock unavailable for items: "
					for _, u := range unavailable {
						errMsg += fmt.Sprintf("%s (requested: %d, available: %d), ", u.ItemName, u.RequestedQuantity, u.AvailableQuantity)
					}
					return nil, fmt.Errorf(errMsg)
				}
			}
		}
	*/

	var subTotal, discountTotal, taxTotal float64
	items := make([]domain.InvoiceItem, 0, len(req.Items))

	for _, itemReq := range req.Items {
		// Validate item exists in Read Model
		itemName := itemReq.Name
		itemRM, err := s.rmRepo.GetItem(ctx, itemReq.ItemID)
		if err != nil {
			// Fallback: If item not found in read model (e.g. sync lag),
			// use the name provided in the request if available.
			if itemName == "" {
				// Use a generic name with ID as last resort to prevent 500
				itemName = fmt.Sprintf("Item %s", itemReq.ItemID.String()[:8])
			}
		} else {
			itemName = itemRM.Name
		}

		itemTotal := (itemReq.Quantity * itemReq.UnitPrice) - itemReq.Discount + itemReq.Tax

		// Serialize metadata to JSON
		var metadataJSON json.RawMessage
		if itemReq.Metadata != nil && len(itemReq.Metadata) > 0 {
			metadataBytes, err := json.Marshal(itemReq.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal item metadata: %w", err)
			}
			metadataJSON = metadataBytes
		}

		items = append(items, domain.InvoiceItem{
			ID:                uuid.New(),
			InvoiceID:         invoiceID,
			ItemID:            itemReq.ItemID,
			ItemType:          itemReq.ItemType,
			Name:              itemName,
			Description:       itemReq.Description,
			Quantity:          itemReq.Quantity,
			UnitPrice:         itemReq.UnitPrice,
			Discount:          itemReq.Discount,
			Tax:               itemReq.Tax,
			Total:             itemTotal,
			ServiceCategoryID: itemReq.ServiceCategoryID,
			Metadata:          metadataJSON, // Store module-specific metadata
		})

		subTotal += (itemReq.Quantity * itemReq.UnitPrice)
		discountTotal += itemReq.Discount
		taxTotal += itemReq.Tax
	}

	invoice.Items = items
	invoice.SubTotal = subTotal
	invoice.DiscountTotal = discountTotal
	invoice.TaxTotal = taxTotal
	invoice.TotalAmount = subTotal - discountTotal + taxTotal + req.Adjustment + req.ExciseDuty
	invoice.PaidAmount = 0
	invoice.BalanceAmount = invoice.TotalAmount

	if err := s.invoiceRepo.Create(ctx, invoice); err != nil {
		return nil, err
	}

	// Update Stock (Decrease) if applicable - DISABLED
	// Stock will be updated when invoice is sent, not when creating a draft
	/*
		if shouldCheckStock {
			stockItems := make([]outbound.StockCheckItem, 0)
			for _, item := range invoice.Items {
				stockItems = append(stockItems, outbound.StockCheckItem{
					ItemID:   item.ItemID.String(),
					Quantity: int32(item.Quantity),
				})
			}
			if len(stockItems) > 0 {
				err := s.inventoryClient.UpdateStock(ctx, stockItems, "sales", "invoice", invoice.ID.String(), "Invoice Created")
				if err != nil {
					// Log warning
					fmt.Printf("invoice created but failed to update stock: %v\n", err)
				}
			}
		}
	*/

	// Publish InvoiceCreated event
	s.publishInvoiceCreated(invoice)

	return s.mapToResponse(ctx, invoice), nil
}

// CreateInvoiceFromEstimate creates an invoice from a CRM estimate
func (s *InvoiceService) CreateInvoiceFromEstimate(ctx context.Context, orgID uuid.UUID, req dto.CreateInvoiceFromEstimateRequest) (*dto.InvoiceResponse, error) {
	invoiceID := uuid.New()

	billingAddressID := req.BillingAddressID
	shippingAddressID := req.ShippingAddressID
	serviceAddressID := req.ServiceAddressID

	shippingExplicitlyBlank := req.ShippingStreet == "" && req.ShippingCity == "" && req.ShippingState == "" && req.ShippingCode == "" && req.ShippingCountry == ""

	var customerRM *domain.CustomerRM
	if billingAddressID == nil || shippingAddressID == nil || serviceAddressID == nil {
		if customer, err := s.rmRepo.GetCustomer(ctx, req.CustomerID); err == nil && customer != nil {
			customerRM = customer
			if billingAddressID == nil {
				billingAddressID = customer.BillingAddressID
			}
			// Skip copying customer shipping address ID since the source system is CRM (Estimate)
			if shippingAddressID == nil && !shippingExplicitlyBlank && false {
				shippingAddressID = customer.ShippingAddressID
			}
			if serviceAddressID == nil {
				serviceAddressID = customer.ServiceAddressID
			}
		}
	}

	billingStreet := req.BillingStreet
	billingCity := req.BillingCity
	billingState := req.BillingState
	billingCode := req.BillingCode
	billingCountry := req.BillingCountry
	shippingStreet := req.ShippingStreet
	shippingCity := req.ShippingCity
	shippingState := req.ShippingState
	shippingCode := req.ShippingCode
	shippingCountry := req.ShippingCountry

	if billingStreet == "" || shippingStreet == "" {
		if customerRM == nil {
			if customer, err := s.rmRepo.GetCustomer(ctx, req.CustomerID); err == nil && customer != nil {
				customerRM = customer
			}
		}
		if customerRM != nil {
			if billingStreet == "" {
				billingStreet = customerRM.BillingStreet
				billingCity = customerRM.BillingCity
				billingState = customerRM.BillingState
				billingCode = customerRM.BillingCode
				billingCountry = customerRM.BillingCountry
			}
			// Skip copying customer shipping details since the source system is CRM (Estimate)
			if shippingStreet == "" && !shippingExplicitlyBlank && false {
				shippingStreet = customerRM.ShippingStreet
				shippingCity = customerRM.ShippingCity
				shippingState = customerRM.ShippingState
				shippingCode = customerRM.ShippingCode
				shippingCountry = customerRM.ShippingCountry
			}
		}
	}

	// Create invoice with CRM source system
	invoice := &domain.Invoice{
		ID:                invoiceID,
		OrganizationID:    orgID,
		CustomerID:        req.CustomerID,
		ContactID:         req.ContactID,
		Subject:           req.Subject,
		InvoiceNumber:     nil, // Generated on send
		SourceSystem:      domain.SourceSystemCRM,
		SourceReferenceID: &req.EstimateID, // Link back to estimate
		InvoiceDate:       req.InvoiceDate,
		DueDate:           req.DueDate,
		Status:            domain.InvoiceStatusDraft,
		Currency:          req.Currency,
		Adjustment:        req.Adjustment,
		Terms:             req.Terms,
		Notes:             req.Notes,
		BillingAddressID:  billingAddressID,
		ShippingAddressID: shippingAddressID,
		ServiceAddressID:  serviceAddressID,
		BillingStreet:     billingStreet,
		BillingCity:       billingCity,
		BillingState:      billingState,
		BillingCode:       billingCode,
		BillingCountry:    billingCountry,
		ShippingStreet:    shippingStreet,
		ShippingCity:      shippingCity,
		ShippingState:     shippingState,
		ShippingCode:      shippingCode,
		ShippingCountry:   shippingCountry,
	}

	// Convert estimate items to invoice items
	var items []domain.InvoiceItem
	var subTotal, discountTotal, taxTotal float64

	for _, itemReq := range req.Items {
		itemTotal := (itemReq.Quantity * itemReq.UnitPrice) - itemReq.Discount + itemReq.Tax

		items = append(items, domain.InvoiceItem{
			ID:          uuid.New(),
			InvoiceID:   invoiceID,
			ItemID:      uuid.MustParse(itemReq.ItemID), // Assuming valid UUID from CRM
			Name:        itemReq.Description,
			Description: itemReq.Description,
			Quantity:    itemReq.Quantity,
			UnitPrice:   itemReq.UnitPrice,
			Discount:    itemReq.Discount,
			Tax:         itemReq.Tax,
			Total:       itemTotal,
		})

		subTotal += (itemReq.Quantity * itemReq.UnitPrice)
		discountTotal += itemReq.Discount
		taxTotal += itemReq.Tax
	}

	invoice.Items = items
	invoice.SubTotal = subTotal
	invoice.DiscountTotal = discountTotal
	invoice.TaxTotal = taxTotal
	invoice.TotalAmount = subTotal - discountTotal + taxTotal + req.Adjustment
	invoice.BalanceAmount = invoice.TotalAmount

	// Stock Checking for Estimate -> Invoice
	if s.inventoryClient != nil {
		stockItems := make([]outbound.StockCheckItem, 0)
		for _, itemReq := range req.Items {
			if itemReq.ItemID != "" {
				stockItems = append(stockItems, outbound.StockCheckItem{
					ItemID:   itemReq.ItemID,
					Quantity: int32(itemReq.Quantity),
				})
			}
		}
		if len(stockItems) > 0 {
			unavailable, err := s.inventoryClient.CheckStockAvailability(ctx, stockItems)
			if err != nil {
				// If method is not implemented, log warning and continue
				if st, ok := status.FromError(err); ok && st.Code() == codes.Unimplemented {
					fmt.Printf("[WARNING] Inventory service CheckStockAvailability unimplemented (Estimate -> Invoice): %v\n", err)
				} else {
					return nil, fmt.Errorf("failed to check stock availability: %w", err)
				}
			} else if len(unavailable) > 0 {
				errMsg := "stock unavailable for items: "
				for _, u := range unavailable {
					errMsg += fmt.Sprintf("%s (requested: %d, available: %d), ", u.ItemName, u.RequestedQuantity, u.AvailableQuantity)
				}
				return nil, fmt.Errorf("%s", errMsg)
			}
		}
	}

	if err := s.invoiceRepo.Create(ctx, invoice); err != nil {
		return nil, fmt.Errorf("failed to create invoice from estimate: %w", err)
	}

	// Update Stock (Decrease)
	if s.inventoryClient != nil {
		stockItems := make([]outbound.StockCheckItem, 0)
		for _, item := range invoice.Items {
			// Only decrease if ItemID is valid (not nil/empty)
			if item.ItemID != uuid.Nil {
				stockItems = append(stockItems, outbound.StockCheckItem{
					ItemID:   item.ItemID.String(),
					Quantity: int32(item.Quantity),
				})
			}
		}
		if len(stockItems) > 0 {
			err := s.inventoryClient.UpdateStock(ctx, stockItems, "sales", "invoice", invoice.ID.String(), fmt.Sprintf("Converted from Estimate %s", req.EstimateID))
			if err != nil {
				fmt.Printf("invoice created from estimate but failed to update stock: %v\n", err)
			}
		}
	}

	// Publish InvoiceCreated event
	s.publishInvoiceCreated(invoice)

	return s.mapToResponse(ctx, invoice), nil
}

// UpdateInvoice updates an existing invoice
// Only DRAFT invoices can be edited - enforces lifecycle rules
func (s *InvoiceService) UpdateInvoice(ctx context.Context, id uuid.UUID, req dto.CreateInvoiceRequest) (*dto.InvoiceResponse, error) {
	// 1. Get Existing Invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if invoice == nil {
		return nil, fmt.Errorf("invoice not found")
	}

	// 2. Lifecycle Validation - Only DRAFT invoices can be edited
	if !invoice.CanEdit() {
		return nil, fmt.Errorf("cannot edit invoice in %s status - only DRAFT invoices can be edited", invoice.Status)
	}

	// 3. Update Fields
	invoice.Subject = req.Subject
	if req.ContactID != nil {
		invoice.ContactID = req.ContactID
	}
	invoice.InvoiceDate = req.InvoiceDate
	invoice.DueDate = req.DueDate
	invoice.PaymentTerms = req.PaymentTerms
	invoice.Currency = req.Currency
	invoice.Adjustment = req.Adjustment
	invoice.ExciseDuty = req.ExciseDuty
	invoice.SalesCommission = req.SalesCommission
	invoice.SalesOrder = req.SalesOrder
	invoice.PurchaseOrder = req.PurchaseOrder
	invoice.Terms = req.Terms
	if req.OwnerID != nil {
		invoice.OwnerID = req.OwnerID
	}
	invoice.ServiceCategoryID = req.ServiceCategoryID
	invoice.Notes = req.Notes
	invoice.BillingAddressID = req.BillingAddressID
	invoice.ShippingAddressID = req.ShippingAddressID
	invoice.ServiceAddressID = req.ServiceAddressID
	invoice.BillingStreet = req.BillingStreet
	invoice.BillingCity = req.BillingCity
	invoice.BillingState = req.BillingState
	invoice.BillingCode = req.BillingCode
	invoice.BillingCountry = req.BillingCountry
	invoice.ShippingStreet = req.ShippingStreet
	invoice.ShippingCity = req.ShippingCity
	invoice.ShippingState = req.ShippingState
	invoice.ShippingCode = req.ShippingCode
	invoice.ShippingCountry = req.ShippingCountry

	// 4. Update Items
	// Clear existing items
	if err := s.invoiceRepo.ClearItems(ctx, invoice.ID); err != nil {
		return nil, fmt.Errorf("failed to clear existing items: %w", err)
	}

	var subTotal, discountTotal, taxTotal float64
	items := make([]domain.InvoiceItem, 0, len(req.Items))

	for _, itemReq := range req.Items {
		itemName := itemReq.Name
		itemRM, err := s.rmRepo.GetItem(ctx, itemReq.ItemID)
		if err != nil {
			if itemName == "" {
				return nil, fmt.Errorf("item %s not found and no name provided: %w", itemReq.ItemID, err)
			}
		} else {
			itemName = itemRM.Name
		}

		itemTotal := (itemReq.Quantity * itemReq.UnitPrice) - itemReq.Discount + itemReq.Tax

		// Serialize metadata
		var metadataJSON json.RawMessage
		if itemReq.Metadata != nil && len(itemReq.Metadata) > 0 {
			metadataBytes, err := json.Marshal(itemReq.Metadata)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal item metadata: %w", err)
			}
			metadataJSON = metadataBytes
		}

		items = append(items, domain.InvoiceItem{
			ID:                uuid.New(),
			InvoiceID:         invoice.ID,
			ItemID:            itemReq.ItemID,
			ItemType:          itemReq.ItemType,
			Name:              itemName,
			Description:       itemReq.Description,
			Quantity:          itemReq.Quantity,
			UnitPrice:         itemReq.UnitPrice,
			Discount:          itemReq.Discount,
			Tax:               itemReq.Tax,
			Total:             itemTotal,
			ServiceCategoryID: itemReq.ServiceCategoryID,
			Metadata:          metadataJSON,
		})

		subTotal += (itemReq.Quantity * itemReq.UnitPrice)
		discountTotal += itemReq.Discount
		taxTotal += itemReq.Tax
	}

	invoice.Items = items
	invoice.SubTotal = subTotal
	invoice.DiscountTotal = discountTotal
	invoice.TaxTotal = taxTotal
	invoice.TotalAmount = subTotal - discountTotal + taxTotal + req.Adjustment + req.ExciseDuty
	invoice.BalanceAmount = invoice.TotalAmount - invoice.PaidAmount

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return nil, err
	}

	return s.mapToResponse(ctx, invoice), nil
}

// SendInvoice transitions an invoice from DRAFT to SENT
// Generates invoice number and PDF on send
func (s *InvoiceService) SendInvoice(ctx context.Context, id uuid.UUID, req dto.SendInvoiceRequest) (*dto.InvoiceResponse, error) {
	// 1. Get existing invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if invoice == nil {
		return nil, fmt.Errorf("invoice not found")
	}

	// 2. Lifecycle validation - only DRAFT invoices can be sent
	if !invoice.CanSend() {
		return nil, fmt.Errorf("cannot send invoice in %s status - only DRAFT invoices can be sent", invoice.Status)
	}

	// 3. Generate invoice number
	invNum, err := s.invoiceRepo.GetNextInvoiceNumber(ctx, invoice.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice number: %w", err)
	}
	invoice.InvoiceNumber = &invNum

	// 4. Generate PDF
	customer, err := s.rmRepo.GetCustomer(ctx, invoice.CustomerID)
	if err != nil || customer == nil {
		fmt.Printf("[WARNING] Customer %s not found for SendInvoice PDF. Using placeholder.\n", invoice.CustomerID)
		customer = &domain.CustomerRM{
			ID:              invoice.CustomerID,
			OrganizationID:  invoice.OrganizationID,
			DisplayName:     "Customer",
			CompanyName:     "Customer",
			BillingStreet:   invoice.BillingStreet,
			BillingCity:     invoice.BillingCity,
			BillingState:    invoice.BillingState,
			BillingCode:     invoice.BillingCode,
			BillingCountry:  invoice.BillingCountry,
			ShippingStreet:  invoice.ShippingStreet,
			ShippingCity:    invoice.ShippingCity,
			ShippingState:   invoice.ShippingState,
			ShippingCode:    invoice.ShippingCode,
			ShippingCountry: invoice.ShippingCountry,
		}
	}

	org, err := s.rmRepo.GetOrganization(ctx, invoice.OrganizationID)
	if err != nil || org == nil {
		fmt.Printf("[WARNING] Organization %s not found for SendInvoice PDF. Using placeholders: %v\n", invoice.OrganizationID, err)
		org = &domain.OrganizationRM{
			ID:               invoice.OrganizationID,
			OrganizationName: "YOUR COMPANY NAME",
			Address:          "Your Company Address Line 1",
			City:             "City",
			State:            "State",
			ZipCode:          "ZIP",
			Country:          "Country",
			Phone:            "(123) 456-7890",
			Website:          "www.yourcompany.com",
		}
	}

	wo, appts, techs := s.getFSMDetails(ctx, invoice)
	pdfPath, err := s.pdfService.GenerateInvoicePDF(ctx, invoice, customer, org, wo, appts, techs)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	invoice.PDFPath = &pdfPath

	// 5. Update status to SENT
	if err := invoice.CanTransitionTo(domain.InvoiceStatusSent); err != nil {
		return nil, err
	}
	invoice.Status = domain.InvoiceStatusSent

	// 6. Save changes
	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return nil, fmt.Errorf("failed to update invoice: %w", err)
	}

	// 7. Create audit log
	auditLog := &domain.InvoiceAuditLog{
		ID:             uuid.New(),
		OrganizationID: invoice.OrganizationID,
		InvoiceID:      invoice.ID,
		Action:         "invoice_sent",
		OldStatus:      string(domain.InvoiceStatusDraft),
		NewStatus:      string(domain.InvoiceStatusSent),
		Notes:          fmt.Sprintf("Invoice sent with number %s", invNum),
		PerformedBy:    "System", // TODO: Get from auth context
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log error but don't fail
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	// 8. Publish InvoiceSent event
	s.publishInvoiceSent(invoice)

	return s.mapToResponse(ctx, invoice), nil
}

// publishInvoiceCreated emits an event when an invoice is created
func (s *InvoiceService) publishInvoiceCreated(inv *domain.Invoice) {
	event := &domain.InvoiceCreatedEvent{
		InvoiceID:         inv.ID.String(),
		OrganizationID:    inv.OrganizationID.String(),
		CustomerID:        inv.CustomerID.String(),
		SourceSystem:      string(inv.SourceSystem),
		SourceReferenceID: inv.SourceReferenceID,
		Subject:           inv.Subject,
		Status:            string(inv.Status),
		TotalAmount:       inv.TotalAmount,
		Currency:          inv.Currency,
		Timestamp:         time.Now().UTC(),
	}

	// Create proper event metadata
	metadata := shared_events.NewEventMetadata(
		shared_events.EventType("billing.invoice.created"),
		shared_events.AggregateType("invoice"),
		inv.ID.String(),
	)
	s.eventPublisher.Publish(context.Background(), metadata, event)
}

// publishInvoiceSent emits an event when an invoice is sent
func (s *InvoiceService) publishInvoiceSent(inv *domain.Invoice) {
	invNumber := ""
	if inv.InvoiceNumber != nil {
		invNumber = *inv.InvoiceNumber
	}

	event := &domain.InvoiceSentEvent{
		InvoiceID:         inv.ID.String(),
		InvoiceNumber:     invNumber,
		OrganizationID:    inv.OrganizationID.String(),
		CustomerID:        inv.CustomerID.String(),
		SourceSystem:      string(inv.SourceSystem),
		SourceReferenceID: inv.SourceReferenceID,
		PDFPath:           inv.PDFPath,
		TotalAmount:       inv.TotalAmount,
		SentAt:            time.Now().UTC(),
		Timestamp:         time.Now().UTC(),
	}

	// Create proper event metadata
	metadata := shared_events.NewEventMetadata(
		shared_events.InvoiceSent,
		shared_events.AggregateInvoice,
		inv.ID.String(),
	)
	s.eventPublisher.Publish(context.Background(), metadata, event)
}

func (s *InvoiceService) GetInvoice(ctx context.Context, id uuid.UUID) (*dto.InvoiceResponse, error) {
	inv, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.mapToResponse(ctx, inv), nil
}

// GetInvoicePDF returns the path to the invoice PDF, generating it if it doesn't exist but should
func (s *InvoiceService) GetInvoicePDF(ctx context.Context, id uuid.UUID) (string, error) {
	// 1. Get invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to get invoice: %w", err)
	}
	if invoice == nil {
		return "", fmt.Errorf("invoice not found")
	}

	// 2. PDF can be generated for any status (including DRAFT)
	// The pdfService will handle adding the "DRAFT" watermark for draft invoices.

	// 3. We always regenerate for now to ensure payments/status are up to date
	// In production, we might want to check if the PDF is older than the last updated_at

	// 4. Generate PDF (or regenerate for draft invoices)
	// Get customer for PDF generation
	customer, err := s.rmRepo.GetCustomer(ctx, invoice.CustomerID) // Changed from s.customerRepo.GetByID
	if err != nil || customer == nil {
		fmt.Printf("[WARNING] Customer %s not found for GetInvoicePDF. Using placeholder.\n", invoice.CustomerID)
		customer = &domain.CustomerRM{
			ID:              invoice.CustomerID,
			OrganizationID:  invoice.OrganizationID,
			DisplayName:     "Customer",
			CompanyName:     "Customer",
			BillingStreet:   invoice.BillingStreet,
			BillingCity:     invoice.BillingCity,
			BillingState:    invoice.BillingState,
			BillingCode:     invoice.BillingCode,
			BillingCountry:  invoice.BillingCountry,
			ShippingStreet:  invoice.ShippingStreet,
			ShippingCity:    invoice.ShippingCity,
			ShippingState:   invoice.ShippingState,
			ShippingCode:    invoice.ShippingCode,
			ShippingCountry: invoice.ShippingCountry,
		}
	}

	// Generate invoice number if missing (for draft invoices)
	if invoice.InvoiceNumber == nil {
		invNum, err := s.invoiceRepo.GetNextInvoiceNumber(ctx, invoice.OrganizationID) // Changed from s.generateInvoiceNumber(ctx)
		if err != nil {
			return "", fmt.Errorf("failed to generate invoice number: %w", err)
		}
		invoice.InvoiceNumber = &invNum
	}

	org, err := s.rmRepo.GetOrganization(ctx, invoice.OrganizationID)
	if err != nil || org == nil {
		fmt.Printf("[WARNING] Organization %s not found for GetInvoicePDF. Using placeholders: %v\n", invoice.OrganizationID, err)
		org = &domain.OrganizationRM{
			ID:               invoice.OrganizationID,
			OrganizationName: "YOUR COMPANY NAME",
			Address:          "Your Company Address Line 1",
			City:             "City",
			State:            "State",
			ZipCode:          "ZIP",
			Country:          "Country",
			Phone:            "(123) 456-7890",
			Website:          "www.yourcompany.com",
		}
	}

	wo, appts, techs := s.getFSMDetails(ctx, invoice)
	pdfPath, err := s.pdfService.GenerateInvoicePDF(ctx, invoice, customer, org, wo, appts, techs) // Added ctx
	if err != nil {
		return "", fmt.Errorf("failed to generate PDF: %w", err)
	}

	// 5. Update invoice with PDF path (only for non-draft invoices)
	if invoice.Status != domain.InvoiceStatusDraft {
		invoice.PDFPath = &pdfPath
		if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
			return "", fmt.Errorf("failed to update invoice with PDF path: %w", err)
		}
	}

	return pdfPath, nil
}

func (s *InvoiceService) DeleteInvoice(ctx context.Context, id uuid.UUID) error {
	// 1. Get existing invoice to verify it exists
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}

	// 2. Delete the invoice (cascade will handle items)
	if err := s.invoiceRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete invoice: %w", err)
	}

	// 3. Optionally publish InvoiceDeleted event
	// s.publishInvoiceDeleted(invoice)

	return nil
}

func (s *InvoiceService) ListInvoices(ctx context.Context, orgID uuid.UUID) ([]dto.InvoiceResponse, error) {
	invoices, err := s.invoiceRepo.List(ctx, map[string]interface{}{"organization_id": orgID})
	if err != nil {
		return nil, err
	}

	res := make([]dto.InvoiceResponse, 0, len(invoices))
	for _, inv := range invoices {
		res = append(res, *s.mapToResponse(ctx, &inv))
	}
	return res, nil
}

// ListInvoicesByModule returns all invoices for an organization filtered by source system (module)
func (s *InvoiceService) ListInvoicesByModule(ctx context.Context, orgID uuid.UUID, sourceSystem domain.SourceSystem) ([]*dto.InvoiceResponse, error) {
	invoices, err := s.invoiceRepo.ListByModule(ctx, orgID, sourceSystem)
	if err != nil {
		return nil, fmt.Errorf("failed to list invoices by module: %w", err)
	}

	responses := make([]*dto.InvoiceResponse, len(invoices))
	for i, inv := range invoices {
		responses[i] = s.mapToResponse(ctx, &inv)
	}

	return responses, nil
}

func (s *InvoiceService) mapToResponse(ctx context.Context, inv *domain.Invoice) *dto.InvoiceResponse {
	res := &dto.InvoiceResponse{
		ID:            inv.ID,
		InvoiceNumber: inv.InvoiceNumber,

		// Source tracking
		SourceSystem:      string(inv.SourceSystem),
		SourceReferenceID: inv.SourceReferenceID,

		Subject:         inv.Subject,
		Status:          string(inv.Status),
		SubTotal:        inv.SubTotal,
		DiscountTotal:   inv.DiscountTotal,
		TaxTotal:        inv.TaxTotal,
		TotalAmount:     inv.TotalAmount,
		PaidAmount:      inv.PaidAmount,
		BalanceAmount:   inv.BalanceAmount,
		Adjustment:      inv.Adjustment,
		ExciseDuty:      inv.ExciseDuty,
		SalesCommission: inv.SalesCommission,
		SalesOrder:      inv.SalesOrder,
		PurchaseOrder:   inv.PurchaseOrder,
		PaymentTerms:    inv.PaymentTerms,
		Currency:        inv.Currency,
		ServiceCategoryID: inv.ServiceCategoryID,
		OwnerID:         inv.OwnerID,
		CustomerID:        inv.CustomerID,
		ContactID:         inv.ContactID,
		InvoiceDate:       ConvertToOrgTZValue(ctx, inv.InvoiceDate, inv.OrganizationID, s.rmRepo),
		DueDate:         ConvertToOrgTZValue(ctx, inv.DueDate, inv.OrganizationID, s.rmRepo),
		CreatedAt:       ConvertToOrgTZValue(ctx, inv.CreatedAt, inv.OrganizationID, s.rmRepo),
		UpdatedAt:       ConvertToOrgTZValue(ctx, inv.UpdatedAt, inv.OrganizationID, s.rmRepo),

		// PDF path
		PDFPath: inv.PDFPath,

		BillingAddressID:  inv.BillingAddressID,
		ShippingAddressID: inv.ShippingAddressID,
		ServiceAddressID:  inv.ServiceAddressID,
		BillingStreet:   inv.BillingStreet,
		BillingCity:     inv.BillingCity,
		BillingState:    inv.BillingState,
		BillingCode:     inv.BillingCode,
		BillingCountry:  inv.BillingCountry,
		ShippingStreet:  inv.ShippingStreet,
		ShippingCity:    inv.ShippingCity,
		ShippingState:   inv.ShippingState,
		ShippingCode:    inv.ShippingCode,
		ShippingCountry: inv.ShippingCountry,
		Notes:           inv.Notes,
		Terms:           inv.Terms,
	}

	// Fetch Customer details from Read Model or fallback to HTTP client
	customer, err := s.rmRepo.GetCustomer(ctx, inv.CustomerID)

	// Fallback to HTTP client if customer is missing OR has empty name (corrupted read model)
	if (err != nil || customer == nil || customer.DisplayName == "") && s.customerClient != nil {
		fmt.Printf("[INFO] Customer %s missing or invalid in ReadModel, fetching from Customer Service\n", inv.CustomerID)
		remoteCustomer, remoteErr := s.customerClient.GetCustomer(ctx, inv.CustomerID)
		if remoteErr == nil && remoteCustomer != nil {
			customer = remoteCustomer
			// Option: We could update the Read Model here to self-heal permanently?
			// But for now, just returning correct data is enough.
		}
	}

	if customer != nil {
		res.Customer = &dto.CustomerResponse{
			ID:          customer.ID,
			DisplayName: customer.DisplayName,
			CompanyName: customer.CompanyName,
		}
	} else {
		// Last resort fallback using stored address if available, or "Generic Customer"
		res.Customer = &dto.CustomerResponse{
			ID:          inv.CustomerID,
			DisplayName: "Unknown Customer",
			CompanyName: "Unknown Customer",
		}
	}

	// Fetch Contact details from Read Model or fallback to HTTP client
	var contact *domain.ContactRM
	if inv.ContactID != nil {
		contact, _ = s.rmRepo.GetContact(ctx, *inv.ContactID)
		if contact == nil && s.customerClient != nil {
			fmt.Printf("[INFO] Contact %s not found in ReadModel, fetching from Customer Service\n", *inv.ContactID)
			contact, _ = s.customerClient.GetContact(ctx, *inv.ContactID)
		}
	} else {
		// FALLBACK: If no contact associated with invoice, try to fetch primary contact of the customer
		contact, _ = s.rmRepo.GetPrimaryContact(ctx, inv.CustomerID)
	}

	if contact != nil {
		res.Contact = &dto.ContactResponse{
			ID:        contact.ID,
			FirstName: contact.FirstName,
			LastName:  contact.LastName,
			Email:     contact.Email,
		}
	}

	if len(inv.Items) > 0 {
		res.Items = make([]dto.ItemResponse, 0, len(inv.Items))
		for _, item := range inv.Items {
			itemResp := dto.ItemResponse{
				ItemID:            item.ItemID,
				ItemType:          item.ItemType,
				Name:              item.Name,
				Description:       item.Description,
				Quantity:          item.Quantity,
				UnitPrice:         item.UnitPrice,
				Discount:          item.Discount,
				Tax:               item.Tax,
				Total:             item.Total,
				ServiceCategoryID: item.ServiceCategoryID,
			}

			// Deserialize metadata if present
			if len(item.Metadata) > 0 {
				var metadata map[string]interface{}
				if err := json.Unmarshal(item.Metadata, &metadata); err == nil {
					itemResp.Metadata = metadata
				}
			}

			res.Items = append(res.Items, itemResp)
		}
	}

	return res
}

func (s *InvoiceService) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus domain.InvoiceStatus, notes string, performedBy string) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}

	oldStatus := string(invoice.Status)
	invoice.Status = newStatus

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return err
	}

	// Record Audit Log
	auditLog := &domain.InvoiceAuditLog{
		ID:             uuid.New(),
		OrganizationID: invoice.OrganizationID,
		InvoiceID:      invoice.ID,
		Action:         "status_change",
		OldStatus:      oldStatus,
		NewStatus:      string(newStatus),
		Notes:          notes,
		PerformedBy:    performedBy,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return nil
}

func (s *InvoiceService) GetAuditLogs(ctx context.Context, invoiceID uuid.UUID) ([]dto.InvoiceAuditLogResponse, error) {
	logs, err := s.auditRepo.ListByInvoiceID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}

	responses := make([]dto.InvoiceAuditLogResponse, len(logs))
	for i, log := range logs {
		responses[i] = dto.InvoiceAuditLogResponse{
			ID:          log.ID,
			InvoiceID:   log.InvoiceID,
			Action:      log.Action,
			OldStatus:   log.OldStatus,
			NewStatus:   log.NewStatus,
			Notes:       log.Notes,
			PerformedBy: log.PerformedBy,
			CreatedAt:   ConvertToOrgTZValue(ctx, log.CreatedAt, log.OrganizationID, s.rmRepo),
		}
	}

	return responses, nil
}

func (s *InvoiceService) getFSMDetails(ctx context.Context, invoice *domain.Invoice) (*domain.WorkOrderRM, []domain.ServiceAppointmentRM, []string) {
	var workOrder *domain.WorkOrderRM
	var appointments []domain.ServiceAppointmentRM
	var technicians []string

	if invoice.SourceSystem == domain.SourceSystemFSM && invoice.SourceReferenceID != nil {
		if woID, err := uuid.Parse(*invoice.SourceReferenceID); err == nil {
			if wo, err := s.rmRepo.GetWorkOrderByID(ctx, woID); err == nil {
				workOrder = wo
				if workOrder != nil {
					if workOrder.DueDate != nil {
						localDue := ConvertToOrgTZValue(ctx, *workOrder.DueDate, invoice.OrganizationID, s.rmRepo)
						workOrder.DueDate = &localDue
					}
					workOrder.CreatedAt = ConvertToOrgTZValue(ctx, workOrder.CreatedAt, invoice.OrganizationID, s.rmRepo)
				}
			}
			if appts, err := s.rmRepo.GetServiceAppointments(ctx, woID); err == nil {
				appointments = appts
				for i := range appointments {
					appt := &appointments[i]
					appt.ScheduledDate = ConvertToOrgTZValue(ctx, appt.ScheduledDate, invoice.OrganizationID, s.rmRepo)
					if appt.ScheduledStartTime != nil {
						localStart := ConvertToOrgTZValue(ctx, *appt.ScheduledStartTime, invoice.OrganizationID, s.rmRepo)
						appt.ScheduledStartTime = &localStart
					}
					if appt.ScheduledEndTime != nil {
						localEnd := ConvertToOrgTZValue(ctx, *appt.ScheduledEndTime, invoice.OrganizationID, s.rmRepo)
						appt.ScheduledEndTime = &localEnd
					}
					if appt.ActualStartTime != nil {
						localActStart := ConvertToOrgTZValue(ctx, *appt.ActualStartTime, invoice.OrganizationID, s.rmRepo)
						appt.ActualStartTime = &localActStart
					}
					if appt.ActualEndTime != nil {
						localActEnd := ConvertToOrgTZValue(ctx, *appt.ActualEndTime, invoice.OrganizationID, s.rmRepo)
						appt.ActualEndTime = &localActEnd
					}

					if names, err := s.rmRepo.GetTechnicianNamesForAppointment(ctx, appt.ID); err == nil {
						appt.TechnicianNames = names
						for _, name := range names {
							exists := false
							for _, ext := range technicians {
								if ext == name {
									exists = true
									break
								}
							}
							if !exists {
								technicians = append(technicians, name)
							}
						}
					}
				}
			}
		}
	}
	return workOrder, appointments, technicians
}
