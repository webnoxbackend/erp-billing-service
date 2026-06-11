package application

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"erp-billing-service/internal/domain"

	"github.com/phpdave11/gofpdf"
)

type PDFService struct {
	storageBasePath string
}

func NewPDFService(storageBasePath string) *PDFService {
	return &PDFService{storageBasePath: storageBasePath}
}

func (s *PDFService) GenerateInvoicePDF(
	ctx context.Context,
	invoice *domain.Invoice,
	customer *domain.CustomerRM,
	organization *domain.OrganizationRM,
	workOrder *domain.WorkOrderRM,
	appointments []domain.ServiceAppointmentRM,
	technicians []domain.UserReadOnly,
) (string, error) {
	orgDir := filepath.Join(s.storageBasePath, invoice.OrganizationID.String())
	if err := os.MkdirAll(orgDir, 0755); err != nil {
		// Log warning and fallback to system temp directory if permission is denied
		fmt.Printf("[WARNING] Failed to create PDF directory %s: %v. Falling back to temporary directory.\n", orgDir, err)
		orgDir = filepath.Join(os.TempDir(), "billing-pdfs", invoice.OrganizationID.String())
		if errFallback := os.MkdirAll(orgDir, 0755); errFallback != nil {
			return "", fmt.Errorf("failed to create PDF directory (fallback): %w", errFallback)
		}
	}

	filename := fmt.Sprintf("%s.pdf", invoice.ID.String())
	pdfPath := filepath.Join(orgDir, filename)

	invoiceNumber := "DRAFT"
	if invoice.InvoiceNumber != nil {
		invoiceNumber = *invoice.InvoiceNumber
	}

	currencySymbol := getCurrencySymbol(invoice.Currency)

	// Design System color palette tokens
	primaryR, primaryG, primaryB := 26, 54, 93    // Deep Corporate Navy
	textDarkR, textDarkG, textDarkB := 15, 23, 42 // Dark Slate
	textMutedR, textMutedG, textMutedB := 100, 116, 139 // Slate Gray
	lineR, lineG, lineB := 226, 232, 240         // Light Gray borders
	bgLightR, bgLightG, bgLightB := 248, 250, 252 // Alternate row slate background

	// Address formatting variables
	var compAddress, compCityStateZip, compCountry, compPhone, compWebsite string
	if organization != nil {
		compAddress = organization.Address
		
		cityStateZipParts := make([]string, 0)
		if organization.City != "" {
			cityStateZipParts = append(cityStateZipParts, organization.City)
		}
		if organization.State != "" {
			cityStateZipParts = append(cityStateZipParts, organization.State)
		}
		if organization.ZipCode != "" {
			cityStateZipParts = append(cityStateZipParts, organization.ZipCode)
		}
		
		if len(cityStateZipParts) > 0 {
			compCityStateZip = cityStateZipParts[0]
			if len(cityStateZipParts) > 1 {
				compCityStateZip += ", " + cityStateZipParts[1]
			}
			if len(cityStateZipParts) > 2 {
				compCityStateZip += " " + cityStateZipParts[2]
			}
		}
		
		compCountry = organization.Country
		compPhone = organization.Phone
		compWebsite = organization.Website
	}

	// Dynamic fallback for missing organization
	if organization == nil || organization.OrganizationName == "" {
		organization = &domain.OrganizationRM{
			OrganizationName: "YOUR COMPANY NAME",
			Address:          "Your Company Address Line 1",
			City:             "City",
			State:            "State",
			ZipCode:          "ZIP",
			Country:          "Country",
			Phone:            "(123) 456-7890",
			Website:          "www.yourcompany.com",
		}
		compAddress = organization.Address
		compCityStateZip = fmt.Sprintf("%s, %s %s", organization.City, organization.State, organization.ZipCode)
		compCountry = organization.Country
		compPhone = organization.Phone
		compWebsite = organization.Website
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AliasNbPages("{nb}")

	// 1. Footer Function (for elegant page numbering)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.CellFormat(0, 10, fmt.Sprintf("Page %d of {nb}", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	pdf.AddPage()
	pdf.SetMargins(15, 15, 15)

	// Page break helper to prevent custom grids from overflowing
	checkPageBreak := func(spaceNeeded float64) {
		if pdf.GetY()+spaceNeeded > 272.0 {
			pdf.AddPage()
		}
	}

	// 2. Background DRAFT Watermark if invoice status is Draft
	if invoice.Status == domain.InvoiceStatusDraft {
		pdf.SetFont("Arial", "B", 55)
		pdf.SetTextColor(245, 245, 245)
		pdf.Text(45, 145, "DRAFT COPY")
		pdf.SetTextColor(0, 0, 0)
	}

	// 3. Header: Company Logo + Info (Left) and TAX INVOICE (Right)
	logoTempPath := ""
	if organization.IconURL != "" {
		if p, err := downloadLogoToTemp(organization.IconURL); err == nil {
			logoTempPath = p
			defer os.Remove(p)
		} else {
			fmt.Printf("[WARNING] Could not download org logo: %v\n", err)
		}
	}

	leftX := 15.0

	if logoTempPath != "" {
		// Detect dimensions to scale correctly
		imgWidth := 0
		imgHeight := 0
		if file, err := os.Open(logoTempPath); err == nil {
			if config, _, err := image.DecodeConfig(file); err == nil {
				imgWidth = config.Width
				imgHeight = config.Height
			}
			file.Close()
		}

		// Default fallback dimensions if decoding failed
		if imgWidth == 0 || imgHeight == 0 {
			imgWidth = 200
			imgHeight = 100
		}

		aspectRatio := float64(imgWidth) / float64(imgHeight)
		
		// Circular logo: radius 9mm (18mm diameter)
		r := 9.0
		centerX := 15.0 + r
		centerY := 15.0 + r

		// Enable circular clipping
		pdf.ClipCircle(centerX, centerY, r, false)

		// Render the image covering the circle
		var imgX, imgY, imgW, imgH float64
		if aspectRatio > 1.0 {
			// Landscape/wide: height covers the diameter, width scaled
			imgH = r * 2.0
			imgW = r * 2.0 * aspectRatio
			imgX = centerX - imgW/2.0
			imgY = centerY - imgH/2.0
		} else {
			// Portrait/tall/square: width covers the diameter, height scaled
			imgW = r * 2.0
			imgH = (r * 2.0) / aspectRatio
			imgX = centerX - imgW/2.0
			imgY = centerY - imgH/2.0
		}

		pdf.ImageOptions(logoTempPath, imgX, imgY, imgW, imgH, false, gofpdf.ImageOptions{ImageType: "", ReadDpi: true}, 0, "")
		
		// End clipping
		pdf.ClipEnd()

		// Draw a clean, subtle circular border around the logo
		pdf.SetDrawColor(lineR, lineG, lineB)
		pdf.SetLineWidth(0.2)
		pdf.Circle(centerX, centerY, r, "D")
		
		leftX = 15.0 + (r * 2.0) + 6.0 // 6mm gap after the round logo
	}

	pdf.SetXY(leftX, 15)
	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(primaryR, primaryG, primaryB)
	pdf.Cell(120.0-leftX, 6, organization.OrganizationName)

	// TAX INVOICE label – always top-right regardless of logo
	pdf.SetXY(120, 15)
	pdf.SetFont("Arial", "B", 20)
	pdf.CellFormat(75, 6, "INVOICE", "", 1, "R", false, 0, "")
	
	pdf.SetX(leftX)
	pdf.SetFont("Arial", "", 8.5)
	pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
	
	currentY := 22.0
	if compAddress != "" {
		pdf.SetXY(leftX, currentY)
		pdf.Cell(100, 4, compAddress)
		currentY += 4
	}
	
	cityStateZipStr := compCityStateZip
	if compCountry != "" {
		if cityStateZipStr != "" {
			cityStateZipStr += ", "
		}
		cityStateZipStr += compCountry
	}
	if cityStateZipStr != "" {
		pdf.SetXY(leftX, currentY)
		pdf.Cell(100, 4, cityStateZipStr)
		currentY += 4
	}
	
	contactStr := ""
	if compPhone != "" {
		contactStr += "Phone: " + compPhone
	}
	if organization != nil && organization.Email != "" {
		if contactStr != "" {
			contactStr += " | "
		}
		contactStr += "Email: " + organization.Email
	}
	if compWebsite != "" {
		if contactStr != "" {
			contactStr += " | "
		}
		contactStr += "Website: " + compWebsite
	}
	if contactStr != "" {
		pdf.SetXY(leftX, currentY)
		pdf.Cell(100, 4, contactStr)
	}

	// 4. Zoho-Style 2-Column Metadata Box (Grid)
	hasThirdRow := invoice.SalesOrder != "" || invoice.PurchaseOrder != "" || invoice.ReferenceNo != ""
	boxHeight := 13.0
	if hasThirdRow {
		boxHeight = 18.0
	}

	pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
	pdf.SetDrawColor(lineR, lineG, lineB)
	pdf.Rect(15, 45, 180, boxHeight, "DF")
	
	// Column 1
	pdf.SetY(47)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
	pdf.SetX(20)
	pdf.Cell(30, 5, "Invoice Number")
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(60, 5, ": " + invoiceNumber)
	
	// Column 2
	pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(30, 5, "Invoice Date")
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(0, 5, ": " + invoice.InvoiceDate.Format("02/01/2006"))
	pdf.Ln(5)
	
	pdf.SetX(20)
	pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(30, 5, "Payment Terms")
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	pdf.SetFont("Arial", "", 9)
	paymentTerms := "Due on Receipt"
	if invoice.PaymentTerms != "" {
		paymentTerms = invoice.PaymentTerms
	}
	pdf.Cell(60, 5, ": " + paymentTerms)
	
	pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
	pdf.SetFont("Arial", "B", 9)
	pdf.Cell(30, 5, "Due Date")
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(0, 5, ": " + invoice.DueDate.Format("02/01/2006"))
	
	if hasThirdRow {
		pdf.Ln(5)
		pdf.SetX(20)
		if invoice.SalesOrder != "" || invoice.PurchaseOrder != "" {
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.SetFont("Arial", "B", 9)
			label := "Sales Order"
			value := invoice.SalesOrder
			if value == "" {
				label = "Purchase Order"
				value = invoice.PurchaseOrder
			}
			pdf.Cell(30, 5, label)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.SetFont("Arial", "", 9)
			pdf.Cell(60, 5, ": " + value)
		} else if invoice.ReferenceNo != "" {
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.SetFont("Arial", "B", 9)
			pdf.Cell(30, 5, "Reference No")
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.SetFont("Arial", "", 9)
			pdf.Cell(60, 5, ": " + invoice.ReferenceNo)
		}
	}
	
	// Reset Y position after the metadata box
	pdf.SetY(45 + boxHeight + 4.0)

	// 5. Dynamic Address Block — show only the addresses that are present
	type addressCol struct {
		label   string
		name    string
		street  string
		city    string
		state   string
		code    string
		country string
	}

	var addressCols []addressCol

	if invoice.BillingStreet != "" || invoice.BillingCity != "" {
		addressCols = append(addressCols, addressCol{
			label:   "BILLING ADDRESS",
			name:    customer.DisplayName,
			street:  invoice.BillingStreet,
			city:    invoice.BillingCity,
			state:   invoice.BillingState,
			code:    invoice.BillingCode,
			country: invoice.BillingCountry,
		})
	}

	if invoice.ServiceStreet != "" || invoice.ServiceCity != "" {
		addressCols = append(addressCols, addressCol{
			label:   "SERVICE ADDRESS",
			name:    customer.DisplayName,
			street:  invoice.ServiceStreet,
			city:    invoice.ServiceCity,
			state:   invoice.ServiceState,
			code:    invoice.ServiceCode,
			country: invoice.ServiceCountry,
		})
	}

	if invoice.ShippingStreet != "" || invoice.ShippingCity != "" {
		// Only add shipping if it's different from service to avoid duplication
		isDuplicate := false
		for _, col := range addressCols {
			if col.street == invoice.ShippingStreet && col.city == invoice.ShippingCity {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			addressCols = append(addressCols, addressCol{
				label:   "SHIPPING ADDRESS",
				name:    customer.DisplayName,
				street:  invoice.ShippingStreet,
				city:    invoice.ShippingCity,
				state:   invoice.ShippingState,
				code:    invoice.ShippingCode,
				country: invoice.ShippingCountry,
			})
		}
	}

	// 5. Combined Customer & Address Details Card
	var nameLines, emailLines, phoneLines [][]byte
	nameVal := "N/A"
	emailVal := "N/A"
	phoneVal := "N/A"
	if customer != nil {
		if customer.DisplayName != "" {
			nameVal = customer.DisplayName
		}
		if customer.Email != "" {
			emailVal = customer.Email
		}
		if customer.Phone != "" {
			phoneVal = customer.Phone
		}
	}
	
	nameLines = pdf.SplitLines([]byte(nameVal), 54.0)
	emailLines = pdf.SplitLines([]byte(emailVal), 54.0)
	phoneLines = pdf.SplitLines([]byte(phoneVal), 54.0)
	
	maxCustLines := len(nameLines)
	if len(emailLines) > maxCustLines {
		maxCustLines = len(emailLines)
	}
	if len(phoneLines) > maxCustLines {
		maxCustLines = len(phoneLines)
	}
	
	custHeight := 4.5 + 1.0 + float64(maxCustLines)*4.0

	var addrCardHeight float64 = 0.0
	type addressColLayout struct {
		label        string
		addressLines [][]byte
		height       float64
	}
	var layouts []addressColLayout
	var colWidth float64 = 0.0
	var textWidth float64 = 0.0

	if len(addressCols) > 0 {
		totalWidth := 180.0
		colWidth = totalWidth / float64(len(addressCols))
		textWidth = colWidth - 6.0 // 3mm padding on each side

		layouts = make([]addressColLayout, len(addressCols))
		maxColHeight := 0.0

		for i, col := range addressCols {
			layout := addressColLayout{
				label: col.label,
			}
			addrText := formatAddress(col.street, col.city, col.state, col.code, col.country)
			layout.addressLines = pdf.SplitLines([]byte(addrText), textWidth)

			// Calculate height for this column: Label (4.5) + Gap (2.0) + Address lines
			h := 4.5 + 2.0 + float64(len(layout.addressLines))*4.0
			layout.height = h
			layouts[i] = layout
			if h > maxColHeight {
				maxColHeight = h
			}
		}

		addrCardHeight = maxColHeight
	}

	// Calculate total height of combined card
	cardHeight := 4.0 + custHeight + 3.0 // Top padding + customer height + separator spacing
	if len(addressCols) > 0 {
		cardHeight += 3.0 + addrCardHeight // spacing + address height
	}
	cardHeight += 4.0 // Bottom padding

	// Page break check for header and card
	checkPageBreak(7.0 + cardHeight + 10.0)

	// Section Header
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(primaryR, primaryG, primaryB)
	pdf.Cell(0, 6, "CUSTOMER DETAILS")
	pdf.Ln(7)

	cardStartY := pdf.GetY()

	// Draw card container with light grey background and thin border
	pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
	pdf.SetDrawColor(lineR, lineG, lineB)
	pdf.SetLineWidth(0.2)
	pdf.Rect(15, cardStartY, 180, cardHeight, "DF")

	// 1. Render Customer Details (3 Columns)
	colWidthCust := 180.0 / 3.0
	
	// Column 0: Customer Name
	pdf.SetXY(18.0, cardStartY+4.0)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetTextColor(primaryR, primaryG, primaryB)
	pdf.Cell(colWidthCust-6.0, 4.5, "CUSTOMER NAME")
	
	pdf.SetFont("Arial", "B", 8.5)
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	currentY = cardStartY + 9.5
	for _, line := range nameLines {
		pdf.SetXY(18.0, currentY)
		pdf.Cell(colWidthCust-6.0, 4.0, string(line))
		currentY += 4.0
	}

	// Column 1: Email
	pdf.SetXY(15.0+colWidthCust+3.0, cardStartY+4.0)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetTextColor(primaryR, primaryG, primaryB)
	pdf.Cell(colWidthCust-6.0, 4.5, "EMAIL")
	
	pdf.SetFont("Arial", "", 8.5)
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	currentY = cardStartY + 9.5
	for _, line := range emailLines {
		pdf.SetXY(15.0+colWidthCust+3.0, currentY)
		pdf.Cell(colWidthCust-6.0, 4.0, string(line))
		currentY += 4.0
	}

	// Column 2: Phone
	pdf.SetXY(15.0+colWidthCust*2.0+3.0, cardStartY+4.0)
	pdf.SetFont("Arial", "B", 7.5)
	pdf.SetTextColor(primaryR, primaryG, primaryB)
	pdf.Cell(colWidthCust-6.0, 4.5, "PHONE")
	
	pdf.SetFont("Arial", "", 8.5)
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	currentY = cardStartY + 9.5
	for _, line := range phoneLines {
		pdf.SetXY(15.0+colWidthCust*2.0+3.0, currentY)
		pdf.Cell(colWidthCust-6.0, 4.0, string(line))
		currentY += 4.0
	}

	// Draw vertical separators in Customer Details row
	pdf.SetDrawColor(lineR, lineG, lineB)
	pdf.SetLineWidth(0.15)
	pdf.Line(15.0+colWidthCust, cardStartY+4.0, 15.0+colWidthCust, cardStartY+4.0+custHeight)
	pdf.Line(15.0+colWidthCust*2.0, cardStartY+4.0, 15.0+colWidthCust*2.0, cardStartY+4.0+custHeight)

	// 2. Draw horizontal separator line between customer details and address details
	sepY := cardStartY + 4.0 + custHeight + 2.0
	pdf.SetDrawColor(lineR, lineG, lineB)
	pdf.SetLineWidth(0.15)
	pdf.Line(18.0, sepY, 192.0, sepY)

	// 3. Render Address Details (side-by-side columns)
	if len(addressCols) > 0 {
		addrStartY := sepY + 3.0

		// Draw vertical separators in Address row
		if len(addressCols) > 1 {
			pdf.SetDrawColor(lineR, lineG, lineB)
			pdf.SetLineWidth(0.15)
			for i := 1; i < len(addressCols); i++ {
				sepX := 15.0 + float64(i)*colWidth
				pdf.Line(sepX, addrStartY, sepX, addrStartY+addrCardHeight)
			}
		}

		for i, col := range addressCols {
			colStartX := 15.0 + float64(i)*colWidth + 3.0
			currentY = addrStartY

			// Print Label
			pdf.SetXY(colStartX, currentY)
			pdf.SetFont("Arial", "B", 7.5)
			pdf.SetTextColor(primaryR, primaryG, primaryB)
			pdf.Cell(textWidth, 4.5, col.label)
			currentY += 4.5
			currentY += 2.0 // Gap below label before address

			// Print Address lines
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			for _, line := range layouts[i].addressLines {
				pdf.SetXY(colStartX, currentY)
				pdf.Cell(textWidth, 4, string(line))
				currentY += 4
			}
		}

		pdf.SetY(addrStartY + addrCardHeight + 6.0)
	} else {
		pdf.SetY(pdf.GetY() + 4)
	}

	// 1. Service Details Section (merged Work Order and Appointment details)
	if workOrder != nil || len(appointments) > 0 {
		// Calculate the total height of the single unified card
		var totalCardHeight float64 = 0.0
		var workOrderHeight float64 = 0.0
		var summaryLines [][]byte
		var summaryHeight float64 = 0.0

		if workOrder != nil {
			summaryLines = pdf.SplitLines([]byte(workOrder.Summary), 170.0)
			summaryHeight = float64(len(summaryLines)) * 4.0
			workOrderHeight = 28.0 + summaryHeight // Inner header (6) + fields (12) + description (10)
			totalCardHeight += workOrderHeight
		}

		type apptLayout struct {
			appt        domain.ServiceAppointmentRM
			techList    []domain.UserReadOnly
			notesLines  [][]byte
			notesHeight float64
			techsHeight float64
			apptHeight  float64
		}
		var apptLayouts []apptLayout

		for _, appt := range appointments {
			var techList []domain.UserReadOnly
			if len(appt.Technicians) > 0 {
				techList = appt.Technicians
			} else {
				techList = technicians
			}

			var notesLines [][]byte
			notesHeight := 0.0
			if appt.Notes != "" {
				notesLines = pdf.SplitLines([]byte(appt.Notes), 170.0)
				notesHeight = float64(len(notesLines))*4.0 + 4.0
			}

			techsHeight := 5.0
			if len(techList) > 0 {
				techsHeight = 10.0 + float64(len(techList))*12.0
			}

			apptHeight := 28.0 + notesHeight + techsHeight
			apptLayouts = append(apptLayouts, apptLayout{
				appt:        appt,
				techList:    techList,
				notesLines:  notesLines,
				notesHeight: notesHeight,
				techsHeight: techsHeight,
				apptHeight:  apptHeight,
			})
		}

		if len(apptLayouts) > 0 {
			if totalCardHeight > 0 {
				totalCardHeight += 5.0 // spacer between work order and appointments
			}
			for _, al := range apptLayouts {
				totalCardHeight += al.apptHeight
			}
			if len(apptLayouts) > 1 {
				totalCardHeight += float64(len(apptLayouts)-1) * 5.0
			}
		}

		checkPageBreak(totalCardHeight + 15.0)

		// Section Header
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 6, "SERVICE DETAILS")
		pdf.Ln(7)

		cardStartY := pdf.GetY()

		// Borderless card with light grey background covering the entire service details
		pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
		pdf.SetDrawColor(lineR, lineG, lineB)
		pdf.SetLineWidth(0.2)
		pdf.Rect(15, cardStartY, 180, totalCardHeight, "DF")

		currentY := cardStartY

		// Render Work Order Details Card Section
		if workOrder != nil {
			// Inner Header line inside card
			pdf.SetXY(18, currentY+3.0)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(primaryR, primaryG, primaryB)
			pdf.Cell(0, 5, "Work Order Details")

			// Row 1
			pdf.SetXY(18, currentY+9.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(35, 5, "Work Order ID")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.Cell(59, 5, ": "+workOrder.ID.String())

			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(25, 5, "Job Type")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			woType := "Standard"
			if workOrder.Type != "" {
				woType = workOrder.Type
			}
			pdf.Cell(0, 5, ": "+woType)

			// Row 2
			pdf.SetXY(18, currentY+15.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(35, 5, "Due Date")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			dueDateStr := "N/A"
			if workOrder.DueDate != nil {
				dueDateStr = workOrder.DueDate.Format("02/01/2006")
			}
			pdf.Cell(59, 5, ": "+dueDateStr)

			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(25, 5, "Created At")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.Cell(0, 5, ": "+workOrder.CreatedAt.Format("02/01/2006"))

			// Row 3: Summary / Description
			pdf.SetXY(18, currentY+21.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(30, 5, "Description")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.SetXY(18, currentY+25.0)
			
			lineHeight := 4.0
			for _, line := range summaryLines {
				pdf.SetX(18)
				pdf.Cell(170, lineHeight, string(line))
				pdf.Ln(lineHeight)
			}

			currentY += workOrderHeight
		}

		// Render Service Appointment Card Sections
		for _, al := range apptLayouts {
			if currentY > cardStartY {
				// Draw separator line between Work Order and Appointments or between consecutive Appointments
				pdf.SetDrawColor(lineR, lineG, lineB)
				pdf.SetLineWidth(0.15)
				pdf.Line(18, currentY+2.0, 192, currentY+2.0)
				currentY += 5.0
			}

			// Header line inside card
			pdf.SetXY(18, currentY+3.0)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(primaryR, primaryG, primaryB)
			pdf.Cell(0, 5, "Service Appointment Details")

			// Row 1
			pdf.SetXY(18, currentY+9.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(35, 5, "Appointment Number")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.Cell(59, 5, ": "+al.appt.AppointmentNumber)

			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(25, 5, "Status")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.Cell(0, 5, ": "+al.appt.Status)

			// Row 2: Actual Start and Actual End
			pdf.SetXY(18, currentY+15.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(35, 5, "Actual Start")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			actStartStr := "N/A"
			if al.appt.ActualStartTime != nil {
				actStartStr = al.appt.ActualStartTime.Format("02/01/2006 15:04")
			}
			pdf.Cell(59, 5, ": "+actStartStr)

			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(25, 5, "Actual End")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			actEndStr := "N/A"
			if al.appt.ActualEndTime != nil {
				actEndStr = al.appt.ActualEndTime.Format("02/01/2006 15:04")
			}
			pdf.Cell(0, 5, ": "+actEndStr)

			// Row 3: Total Time Worked
			pdf.SetXY(18, currentY+21.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(35, 5, "Total Time Worked")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			
			durationStr := "N/A"
			if al.appt.ActualStartTime != nil && al.appt.ActualEndTime != nil {
				diff := al.appt.ActualEndTime.Sub(*al.appt.ActualStartTime)
				hours := int(diff.Hours())
				minutes := int(diff.Minutes()) % 60
				if hours > 0 {
					durationStr = fmt.Sprintf("%d hrs %d mins", hours, minutes)
				} else {
					durationStr = fmt.Sprintf("%d mins", minutes)
				}
			}
			pdf.Cell(0, 5, ": "+durationStr)

			// Row 4: Notes
			if al.appt.Notes != "" {
				pdf.SetXY(18, currentY+27.0)
				pdf.SetFont("Arial", "B", 8)
				pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
				pdf.Cell(30, 4, "Appointment Notes")
				pdf.SetFont("Arial", "", 8.5)
				pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
				pdf.SetXY(18, currentY+30.0)
				
				for _, line := range al.notesLines {
					pdf.SetX(18)
					pdf.Cell(170, 4.0, string(line))
					pdf.Ln(4.0)
				}
			}

			// Row 5: Detailed Technicians section
			techStartY := currentY + 27.0
			if al.appt.Notes != "" {
				techStartY += al.notesHeight
			}

			// Draw separator line inside the card
			pdf.SetDrawColor(lineR, lineG, lineB)
			pdf.SetLineWidth(0.1)
			pdf.Line(18, techStartY, 192, techStartY)

			pdf.SetXY(18, techStartY+2.0)
			pdf.SetFont("Arial", "B", 7.5)
			pdf.SetTextColor(primaryR, primaryG, primaryB)
			pdf.Cell(0, 5, "ASSIGNED TECHNICIANS")

			techY := techStartY + 8.0
			if len(al.techList) == 0 {
				pdf.SetXY(18, techY)
				pdf.SetFont("Arial", "I", 8.5)
				pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
				pdf.Cell(0, 5, "No technicians assigned")
			} else {
				for _, tech := range al.techList {
					// Row 1 of technician: Name & Employee ID
					pdf.SetXY(18, techY)
					pdf.SetFont("Arial", "B", 8)
					pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
					pdf.Cell(35, 5, "Technician Name")
					pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
					pdf.SetFont("Arial", "", 8.5)
					pdf.Cell(59, 5, ": "+tech.FullName)

					pdf.SetFont("Arial", "B", 8)
					pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
					pdf.Cell(25, 5, "Employee ID")
					pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
					pdf.SetFont("Arial", "", 8.5)
					empID := "N/A"
					if tech.EmployeeID != nil && *tech.EmployeeID != "" {
						empID = *tech.EmployeeID
					}
					pdf.Cell(0, 5, ": "+empID)

					// Row 2 of technician: Phone & Email
					pdf.SetXY(18, techY+6.0)
					pdf.SetFont("Arial", "B", 8)
					pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
					pdf.Cell(35, 5, "Mobile")
					pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
					pdf.SetFont("Arial", "", 8.5)
					mobile := "N/A"
					if tech.PhoneNumber != nil && *tech.PhoneNumber != "" {
						mobile = *tech.PhoneNumber
					}
					pdf.Cell(59, 5, ": "+mobile)

					pdf.SetFont("Arial", "B", 8)
					pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
					pdf.Cell(25, 5, "Email")
					pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
					pdf.SetFont("Arial", "", 8.5)
					pdf.Cell(0, 5, ": "+tech.Email)

					techY += 12.0
				}
			}

			currentY += al.apptHeight
		}

		pdf.SetY(cardStartY + totalCardHeight + 5.0)
		pdf.Ln(2)
	}

	// Separate services and parts
	var services []domain.InvoiceItem
	var parts []domain.InvoiceItem

	for _, item := range invoice.Items {
		itemTypeLower := strings.ToLower(item.ItemType)
		if itemTypeLower == "part" || itemTypeLower == "goods" || itemTypeLower == "product" {
			parts = append(parts, item)
		} else {
			services = append(services, item)
		}
	}

	// 2. Services Provided Section
	if len(services) > 0 {
		checkPageBreak(25.0)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 6, "SERVICES PROVIDED")
		pdf.Ln(7)

		// Table Header
		pdf.SetFont("Arial", "B", 8)
		pdf.SetFillColor(primaryR, primaryG, primaryB)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetDrawColor(lineR, lineG, lineB)

		pdf.CellFormat(8, 7, "#", "1", 0, "C", true, 0, "")
		pdf.CellFormat(62, 7, "Service Name & Description", "1", 0, "L", true, 0, "")
		pdf.CellFormat(15, 7, "Qty", "1", 0, "C", true, 0, "")
		pdf.CellFormat(23, 7, "Rate", "1", 0, "R", true, 0, "")
		pdf.CellFormat(22, 7, "Discount", "1", 0, "R", true, 0, "")
		pdf.CellFormat(22, 7, "Tax", "1", 0, "R", true, 0, "")
		pdf.CellFormat(28, 7, "Amount", "1", 1, "R", true, 0, "")

		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.SetFont("Arial", "", 8.5)

		for i, item := range services {
			nameLines := pdf.SplitLines([]byte(item.Name), 58.0)
			var descLines [][]byte
			if item.Description != "" {
				descLines = pdf.SplitLines([]byte(item.Description), 58.0)
			}
			lineHeight := 4.0
			totalTextHeight := float64(len(nameLines)) * lineHeight
			if len(descLines) > 0 {
				totalTextHeight += float64(len(descLines))*3.5 + 1.5
			}
			rowHeight := totalTextHeight + 3.0
			if rowHeight < 8.0 {
				rowHeight = 8.0
			}

			checkPageBreak(rowHeight)
			
			fill := i%2 == 1
			if fill {
				pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
			}
			
			startY := pdf.GetY()
			
			pdf.CellFormat(8, rowHeight, fmt.Sprintf("%d", i+1), "1", 0, "C", fill, 0, "")
			currX := pdf.GetX()
			pdf.CellFormat(62, rowHeight, "", "1", 0, "L", fill, 0, "") // empty placeholder
			pdf.CellFormat(15, rowHeight, fmt.Sprintf("%.2f", item.Quantity), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(23, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.UnitPrice), "1", 0, "R", fill, 0, "")
			pdf.CellFormat(22, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.Discount), "1", 0, "R", fill, 0, "")
			pdf.CellFormat(22, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.Tax), "1", 0, "R", fill, 0, "")
			pdf.CellFormat(28, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.Total), "1", 1, "R", fill, 0, "")
			
			endY := pdf.GetY()
			
			// Render wrapped name
			pdf.SetFont("Arial", "B", 8.5)
			nameY := startY + 1.5
			for _, line := range nameLines {
				pdf.SetXY(currX + 2.0, nameY)
				pdf.Cell(58, lineHeight, string(line))
				nameY += lineHeight
			}
			
			// Render wrapped description
			if len(descLines) > 0 {
				pdf.SetFont("Arial", "I", 7.5)
				pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
				descY := nameY + 0.5
				for _, line := range descLines {
					pdf.SetXY(currX + 2.0, descY)
					pdf.Cell(58, 3.5, string(line))
					descY += 3.5
				}
				pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			}
			
			pdf.SetY(endY)
			pdf.SetFont("Arial", "", 8.5)
		}
		pdf.Ln(6)
	}

	// 3. Parts Used Section
	if len(parts) > 0 {
		checkPageBreak(25.0)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 6, "PARTS USED")
		pdf.Ln(7)

		// Table Header
		pdf.SetFont("Arial", "B", 8)
		pdf.SetFillColor(primaryR, primaryG, primaryB)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetDrawColor(lineR, lineG, lineB)

		pdf.CellFormat(8, 7, "#", "1", 0, "C", true, 0, "")
		pdf.CellFormat(62, 7, "Part Name & Description", "1", 0, "L", true, 0, "")
		pdf.CellFormat(15, 7, "Qty", "1", 0, "C", true, 0, "")
		pdf.CellFormat(23, 7, "Rate", "1", 0, "R", true, 0, "")
		pdf.CellFormat(22, 7, "Discount", "1", 0, "R", true, 0, "")
		pdf.CellFormat(22, 7, "Tax", "1", 0, "R", true, 0, "")
		pdf.CellFormat(28, 7, "Amount", "1", 1, "R", true, 0, "")

		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.SetFont("Arial", "", 8.5)

		for i, item := range parts {
			nameLines := pdf.SplitLines([]byte(item.Name), 58.0)
			var descLines [][]byte
			if item.Description != "" {
				descLines = pdf.SplitLines([]byte(item.Description), 58.0)
			}
			lineHeight := 4.0
			totalTextHeight := float64(len(nameLines)) * lineHeight
			if len(descLines) > 0 {
				totalTextHeight += float64(len(descLines))*3.5 + 1.5
			}
			rowHeight := totalTextHeight + 3.0
			if rowHeight < 8.0 {
				rowHeight = 8.0
			}

			checkPageBreak(rowHeight)
			
			fill := i%2 == 1
			if fill {
				pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
			}
			
			startY := pdf.GetY()
			
			pdf.CellFormat(8, rowHeight, fmt.Sprintf("%d", i+1), "1", 0, "C", fill, 0, "")
			currX := pdf.GetX()
			pdf.CellFormat(62, rowHeight, "", "1", 0, "L", fill, 0, "") // empty placeholder
			pdf.CellFormat(15, rowHeight, fmt.Sprintf("%.2f", item.Quantity), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(23, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.UnitPrice), "1", 0, "R", fill, 0, "")
			pdf.CellFormat(22, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.Discount), "1", 0, "R", fill, 0, "")
			pdf.CellFormat(22, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.Tax), "1", 0, "R", fill, 0, "")
			pdf.CellFormat(28, rowHeight, fmt.Sprintf("%s%.2f", currencySymbol, item.Total), "1", 1, "R", fill, 0, "")
			
			endY := pdf.GetY()
			
			// Render wrapped name
			pdf.SetFont("Arial", "B", 8.5)
			nameY := startY + 1.5
			for _, line := range nameLines {
				pdf.SetXY(currX + 2.0, nameY)
				pdf.Cell(58, lineHeight, string(line))
				nameY += lineHeight
			}
			
			// Render wrapped description
			if len(descLines) > 0 {
				pdf.SetFont("Arial", "I", 7.5)
				pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
				descY := nameY + 0.5
				for _, line := range descLines {
					pdf.SetXY(currX + 2.0, descY)
					pdf.Cell(58, 3.5, string(line))
					descY += 3.5
				}
				pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			}
			
			pdf.SetY(endY)
			pdf.SetFont("Arial", "", 8.5)
		}
		pdf.Ln(6)
	}

	// 5. Footer Totals (Aligned Right with exact formatting)
	checkPageBreak(50.0)
	pdf.SetY(pdf.GetY() + 5)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	
	labelX := 125.0
	valWidth := 70.0
	rowH := 5.5
	
	pdf.SetX(labelX)
	pdf.Cell(40, rowH, "Sub Total")
	pdf.CellFormat(valWidth - 40, rowH, fmt.Sprintf("%s%.2f", currencySymbol, invoice.SubTotal), "", 1, "R", false, 0, "")

	if invoice.DiscountTotal > 0 {
		pdf.SetX(labelX)
		pdf.Cell(40, rowH, "Discount Total")
		pdf.CellFormat(valWidth - 40, rowH, fmt.Sprintf("-%s%.2f", currencySymbol, invoice.DiscountTotal), "", 1, "R", false, 0, "")
	}

	if invoice.TaxTotal > 0 {
		pdf.SetX(labelX)
		pdf.Cell(40, rowH, "Tax Total")
		pdf.CellFormat(valWidth - 40, rowH, fmt.Sprintf("+%s%.2f", currencySymbol, invoice.TaxTotal), "", 1, "R", false, 0, "")
	}

	if invoice.ExciseDuty > 0 {
		pdf.SetX(labelX)
		pdf.Cell(40, rowH, "Excise Duty")
		pdf.CellFormat(valWidth - 40, rowH, fmt.Sprintf("+%s%.2f", currencySymbol, invoice.ExciseDuty), "", 1, "R", false, 0, "")
	}

	if invoice.Adjustment != 0 {
		pdf.SetX(labelX)
		pdf.Cell(40, rowH, "Adjustment")
		sign := "+"
		val := invoice.Adjustment
		if val < 0 {
			sign = "-"
			val = -val
		}
		pdf.CellFormat(valWidth - 40, rowH, fmt.Sprintf("%s%s%.2f", sign, currencySymbol, val), "", 1, "R", false, 0, "")
	}

	if invoice.TDSAmount > 0 {
		pdf.SetX(labelX)
		pdf.Cell(40, rowH, "TDS Amount")
		pdf.CellFormat(valWidth - 40, rowH, fmt.Sprintf("-%s%.2f", currencySymbol, invoice.TDSAmount), "", 1, "R", false, 0, "")
	}

	if invoice.TCSAmount > 0 {
		pdf.SetX(labelX)
		pdf.Cell(40, rowH, "TCS Amount")
		pdf.CellFormat(valWidth - 40, rowH, fmt.Sprintf("+%s%.2f", currencySymbol, invoice.TCSAmount), "", 1, "R", false, 0, "")
	}

	pdf.SetX(labelX)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(40, 8, "Total")
	pdf.CellFormat(valWidth - 40, 8, fmt.Sprintf("%s%.2f", currencySymbol, invoice.TotalAmount), "T", 1, "R", false, 0, "")
	
	// Balance Due banner with Navy solid fill and White text
	pdf.SetX(labelX)
	pdf.SetFillColor(primaryR, primaryG, primaryB)
	pdf.SetTextColor(255, 255, 255)
	
	balanceDue := invoice.TotalAmount - invoice.PaidAmount
	pdf.CellFormat(40, 8, "  Balance Due", "", 0, "L", true, 0, "")
	pdf.CellFormat(valWidth - 40, 8, fmt.Sprintf("%s%.2f  ", currencySymbol, balanceDue), "", 1, "R", true, 0, "")
	
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	pdf.SetFont("Arial", "", 8.5)

	// 6. Notes & Signature Sections
	trimmedNotes := strings.TrimSpace(invoice.Notes)
	if trimmedNotes != "" && trimmedNotes != "<nil>" {
		notesLines := pdf.SplitLines([]byte(trimmedNotes), 180.0)
		notesH := float64(len(notesLines))*4.0 + 10.0
		checkPageBreak(notesH)
		pdf.SetY(pdf.GetY() + 6)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 5, "Notes")
		pdf.Ln(5)
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.MultiCell(180, 4, trimmedNotes, "", "L", false)
	}

	trimmedTerms := strings.TrimSpace(invoice.Terms)
	if trimmedTerms != "" && trimmedTerms != "<nil>" {
		termsLines := pdf.SplitLines([]byte(trimmedTerms), 180.0)
		termsH := float64(len(termsLines))*4.0 + 10.0
		checkPageBreak(termsH)
		pdf.SetY(pdf.GetY() + 6)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 5, "Terms & Conditions")
		pdf.Ln(5)
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.MultiCell(180, 4, trimmedTerms, "", "L", false)
	}

	checkPageBreak(25.0)
	pdf.SetY(-35)
	pdf.SetX(125)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(primaryR, primaryG, primaryB)
	pdf.CellFormat(70, 5, "Authorized Signature", "T", 1, "C", false, 0, "")

	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return "", fmt.Errorf("failed to save PDF file: %w", err)
	}

	return pdfPath, nil
}

func getCurrencySymbol(code string) string {
	switch code {
	case "INR":
		return "Rs. "
	case "EUR":
		return "EUR "
	case "GBP":
		return "GBP "
	default:
		return "$ "
	}
}

func formatAddress(street, city, state, code, country string) string {
	street = strings.TrimSpace(street)
	city = strings.TrimSpace(city)
	state = strings.TrimSpace(state)
	code = strings.TrimSpace(code)
	country = strings.TrimSpace(country)

	// Replace all newlines in street with commas to make it continuous
	street = strings.ReplaceAll(street, "\n", ", ")
	
	// Split by commas, trim, and build a unique slice of parts
	rawParts := strings.Split(street, ",")
	var uniqueParts []string
	seen := make(map[string]bool)

	addPart := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		lower := strings.ToLower(p)
		if !seen[lower] {
			seen[lower] = true
			uniqueParts = append(uniqueParts, p)
		}
	}

	for _, p := range rawParts {
		addPart(p)
	}

	// Add city and state if not already in the street
	addPart(city)
	addPart(state)
	
	// Append ZIP code directly to state if the state is the last item
	if code != "" && !seen[strings.ToLower(code)] {
		if len(uniqueParts) > 0 && strings.ToLower(uniqueParts[len(uniqueParts)-1]) == strings.ToLower(state) {
			uniqueParts[len(uniqueParts)-1] = uniqueParts[len(uniqueParts)-1] + " " + code
			seen[strings.ToLower(code)] = true
		} else {
			addPart(code)
		}
	}
	
	addPart(country)

	// Join all parts with ", " to form a single continuous string
	addrLine := strings.Join(uniqueParts, ", ")
	
	// Return the address line without padding prefix for PDF layout
	return addrLine
}

func containsIgnoreCase(s, substr string) bool {
	if substr == "" {
		return false
	}
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// downloadLogoToTemp fetches an image URL (resolving relative paths if needed)
// and writes it to a temporary file, returning the file path so gofpdf can embed it.
// The caller is responsible for removing the temp file when done.
func downloadLogoToTemp(logoURL string) (string, error) {
	fullURL := logoURL
	if !strings.HasPrefix(logoURL, "http://") && !strings.HasPrefix(logoURL, "https://") {
		mediaBaseURL := os.Getenv("MEDIA_BASE_URL")
		if mediaBaseURL == "" {
			mediaBaseURL = "https://webnox.blr1.digitaloceanspaces.com/"
		}
		// Ensure mediaBaseURL ends with a slash if needed, and logoURL doesn't start with a slash
		if !strings.HasSuffix(mediaBaseURL, "/") && !strings.HasPrefix(logoURL, "/") {
			mediaBaseURL += "/"
		} else if strings.HasSuffix(mediaBaseURL, "/") && strings.HasPrefix(logoURL, "/") {
			mediaBaseURL = strings.TrimSuffix(mediaBaseURL, "/")
		}
		fullURL = mediaBaseURL + logoURL
	}

	resp, err := http.Get(fullURL)
	if err != nil {
		return "", fmt.Errorf("http get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("non-200 response from %s: %d", fullURL, resp.StatusCode)
	}

	// Detect extension from Content-Type
	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	switch {
	case strings.Contains(ct, "png"):
		ext = ".png"
	case strings.Contains(ct, "gif"):
		ext = ".gif"
	case strings.Contains(ct, "webp"):
		ext = ".webp"
	}

	tmp, err := os.CreateTemp("", "org-logo-*"+ext)
	if err != nil {
		return "", fmt.Errorf("create temp: %w", err)
	}
	defer tmp.Close()

	if _, err = io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("write temp: %w", err)
	}

	return tmp.Name(), nil
}
