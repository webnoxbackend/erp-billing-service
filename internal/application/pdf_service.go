package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"erp-billing-service/internal/domain"

	"github.com/phpdave11/gofpdf"
)

type PDFService struct {
	storageBasePath string
}

func NewPDFService(storageBasePath string) *PDFService {
	return &PDFService{storageBasePath: storageBasePath}
}

func (s *PDFService) GenerateInvoicePDF(ctx context.Context, invoice *domain.Invoice, customer *domain.CustomerRM) (string, error) {
	orgDir := filepath.Join(s.storageBasePath, invoice.OrganizationID.String())
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create PDF directory: %w", err)
	}

	filename := fmt.Sprintf("%s.pdf", invoice.ID.String())
	pdfPath := filepath.Join(orgDir, filename)

	invoiceNumber := "DRAFT"
	if invoice.InvoiceNumber != nil {
		invoiceNumber = *invoice.InvoiceNumber
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)

	// 1. DRAFT Watermark / Banner if still in draft
	if invoice.Status == domain.InvoiceStatusDraft {
		pdf.SetFont("Arial", "B", 60)
		pdf.SetTextColor(240, 240, 240)
		pdf.Text(40, 150, "DRAFT")
		pdf.SetTextColor(0, 0, 0)
	}

	// 2. Header: Company Info (Left) and TAX INVOICE (Right)
	pdf.SetX(15)
	pdf.SetY(15)
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(100, 8, "YOUR COMPANY NAME")
	
	pdf.SetX(120)
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(75, 8, "TAX INVOICE", "", 1, "R", false, 0, "")
	
	pdf.SetX(15)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(0, 4, "Your Company Address Line 1, City, State, ZIP")
	pdf.Ln(4)
	pdf.SetX(15)
	pdf.Cell(0, 4, "Email: info@yourcompany.com | Phone: (123) 456-7890")
	pdf.Ln(10)

	// 3. Invoice Metadata Box (As seen in Zoho)
	pdf.SetDrawColor(220, 220, 220)
	pdf.Rect(15, 45, 180, 25, "D")
	
	pdf.SetY(48)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetX(20)
	pdf.Cell(30, 5, "#")
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(60, 5, ": " + invoiceNumber)
	
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(30, 5, "Invoice Date")
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(0, 5, ": " + invoice.InvoiceDate.Format("02/01/2006"))
	pdf.Ln(6)
	
	pdf.SetX(20)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(30, 5, "Terms")
	pdf.SetFont("Arial", "", 9)
	terms := "Due on Receipt"
	if invoice.Terms != "" {
		terms = invoice.Terms
	}
	pdf.Cell(60, 5, ": " + terms)
	
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(30, 5, "Due Date")
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(0, 5, ": " + invoice.DueDate.Format("02/01/2006"))
	pdf.Ln(12)

	// 4. Bill To / Ship To Grid
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(245, 245, 245)
	pdf.CellFormat(90, 6, "  Bill To", "1", 0, "L", true, 0, "")
	pdf.CellFormat(90, 6, "  Ship To", "1", 1, "L", true, 0, "")
	
	addressY := pdf.GetY()
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(90, 5, "  " + customer.DisplayName)
	pdf.Cell(90, 5, "  " + customer.DisplayName)
	pdf.Ln(5)
	
	pdf.SetFont("Arial", "", 8)
	pdf.SetTextColor(60, 60, 60)
	// Billing block
	pdf.SetX(15)
	pdf.MultiCell(90, 4, "  " + invoice.BillingStreet + "\n  " + invoice.BillingCity + ", " + invoice.BillingState + " " + invoice.BillingCode + "\n  " + invoice.BillingCountry, "L", "L", false)
	
	// Shipping block (side by side requires manual Y reset)
	pdf.SetY(addressY + 5)
	pdf.SetX(105)
	pdf.MultiCell(90, 4, "  " + invoice.ShippingStreet + "\n  " + invoice.ShippingCity + ", " + invoice.ShippingState + " " + invoice.ShippingCode + "\n  " + invoice.ShippingCountry, "R", "L", false)
	
	pdf.SetY(pdf.GetY() + 5)

	// 5. Line Items Table
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(10, 8, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(90, 8, "Item & Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Rate", "1", 0, "R", true, 0, "")
	pdf.CellFormat(35, 8, "Amount", "1", 1, "R", true, 0, "")

	pdf.SetFont("Arial", "", 9)
	for i, item := range invoice.Items {
		pdf.CellFormat(10, 7, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(90, 7, " " + item.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, fmt.Sprintf("%.2f", item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 7, fmt.Sprintf("$%.2f", item.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(35, 7, fmt.Sprintf("$%.2f", item.Total), "1", 1, "R", false, 0, "")
	}

	// 6. Footer Totals
	pdf.SetY(pdf.GetY() + 5)
	pdf.SetFont("Arial", "", 9)
	pdf.SetX(120)
	pdf.Cell(40, 6, "Sub Total")
	pdf.CellFormat(35, 6, fmt.Sprintf("$%.2f", invoice.SubTotal), "", 1, "R", false, 0, "")

	if invoice.DiscountTotal > 0 {
		pdf.SetX(120)
		pdf.Cell(40, 6, "Discount")
		pdf.CellFormat(35, 6, fmt.Sprintf("$%.2f", invoice.DiscountTotal), "", 1, "R", false, 0, "")
	}

	if invoice.TaxTotal > 0 {
		pdf.SetX(120)
		pdf.Cell(40, 6, "Tax")
		pdf.CellFormat(35, 6, fmt.Sprintf("$%.2f", invoice.TaxTotal), "", 1, "R", false, 0, "")
	}

	pdf.SetX(120)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 8, "Total")
	pdf.CellFormat(35, 8, fmt.Sprintf("$%.2f", invoice.TotalAmount), "T", 1, "R", false, 0, "")
	
	pdf.SetX(120)
	pdf.SetFillColor(245, 245, 245)
	pdf.CellFormat(40, 8, "Balance Due", "", 0, "L", true, 0, "")
	pdf.CellFormat(35, 8, fmt.Sprintf("$%.2f", invoice.TotalAmount - invoice.PaidAmount), "", 1, "R", true, 0, "")

	// 7. Notes & Signature
	if invoice.Notes != "" {
		pdf.SetY(pdf.GetY() + 10)
		pdf.SetFont("Arial", "B", 9)
		pdf.Cell(0, 5, "Notes")
		pdf.Ln(5)
		pdf.SetFont("Arial", "", 8)
		pdf.MultiCell(100, 4, invoice.Notes, "", "L", false)
	}

	pdf.SetY(-40)
	pdf.SetX(130)
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(60, 5, "Authorized Signature", "T", 1, "C", false, 0, "")

	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return "", fmt.Errorf("failed to save PDF file: %w", err)
	}

	return pdfPath, nil
}
