package kafka

import (
	"erp-billing-service/internal/domain"
	"fmt"
	"log"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (h *EventHandler) handleInvoiceEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	switch event.Metadata.EventType {
	case "billing.invoice.paid":
		// Unmarshal payload manually since we don't have shared_events.InvoicePaidPayload equivalent imported yet?
		// Actually, let's use a generic map or struct matching what we expect.
		// Since we are in the same service, we know what we published.
		// domain.InvoicePaidEvent was used.
		
		type InvoicePaidPayload struct {
			InvoiceID string `json:"invoice_id"`
		}
		
		var payload InvoicePaidPayload
		if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
			return err
		}

		invoiceID, err := uuid.Parse(payload.InvoiceID)
		if err != nil {
			return err
		}

		// Update Sales Order status to 'paid' if it is linked to this invoice
		// We search for SalesOrder where InvoiceID = payload.InvoiceID
		if err := tx.Model(&domain.SalesOrder{}).
			Where("invoice_id = ?", invoiceID).
			Update("status", domain.SalesOrderStatusPaid).Error; err != nil {
			return fmt.Errorf("failed to update sales order status to paid: %w", err)
		}
		
		log.Printf("Updated Sales Order status to PAID for InvoiceID: %s", invoiceID)

	case "billing.invoice.created":
		// Could handle this if we needed to clear flags, but sales_order_service handles the initial link
	}
	return nil
}
