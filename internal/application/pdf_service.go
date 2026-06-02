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
	technicians []string,
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
	pdf.CellFormat(75, 6, "TAX INVOICE", "", 1, "R", false, 0, "")
	
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
	pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
	pdf.SetDrawColor(lineR, lineG, lineB)
	pdf.Rect(15, 45, 180, 25, "DF")
	
	// Column 1
	pdf.SetY(48)
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
	pdf.Ln(6)
	
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
	pdf.Ln(6)
	
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
	
	// Reset Y position after the metadata box
	pdf.SetY(75)

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

	if len(addressCols) > 0 {
		totalWidth := 180.0
		colWidth := totalWidth / float64(len(addressCols))

		// Header row
		pdf.SetFont("Arial", "B", 9)
		pdf.SetFillColor(primaryR, primaryG, primaryB)
		pdf.SetTextColor(255, 255, 255)
		for i, col := range addressCols {
			align := "L"
			ln := 0
			if i == len(addressCols)-1 {
				ln = 1
			}
			pdf.CellFormat(colWidth, 6, "  "+col.label, "1", ln, align, true, 0, "")
		}

		// Customer name row
		addressY := pdf.GetY()
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		for i, col := range addressCols {
			ln := 0
			if i == len(addressCols)-1 {
				ln = 1
			}
			pdf.CellFormat(colWidth, 5, "  "+col.name, "", ln, "L", false, 0, "")
		}

		// Address body rows — render each column's multiline address and track max Y
		bodyStartY := pdf.GetY()
		colEndYs := make([]float64, len(addressCols))

		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)

		for i, col := range addressCols {
			pdf.SetXY(15+float64(i)*colWidth, bodyStartY)
			addrText := formatAddress(col.street, col.city, col.state, col.code, col.country)

			// Determine left/right border alignment
			borderStyle := ""
			if i == 0 {
				borderStyle = "L"
			} else if i == len(addressCols)-1 {
				borderStyle = "R"
			}
			pdf.MultiCell(colWidth, 4.5, addrText, borderStyle, "L", false)
			colEndYs[i] = pdf.GetY()
		}

		// Find the max Y across all columns to ensure consistent bottom alignment
		maxAddrY := bodyStartY
		for _, y := range colEndYs {
			if y > maxAddrY {
				maxAddrY = y
			}
		}

		// Draw bottom border line across full address block width
		pdf.SetDrawColor(lineR, lineG, lineB)
		pdf.Line(15, maxAddrY, 195, maxAddrY)

		_ = addressY
		pdf.SetY(maxAddrY + 6)
	} else {
		pdf.SetY(pdf.GetY() + 4)
	}

	// Page break helper to prevent custom grids from overflowing
	checkPageBreak := func(spaceNeeded float64) {
		if pdf.GetY()+spaceNeeded > 272.0 {
			pdf.AddPage()
		}
	}

	// 1. Work Order Details Section
	if workOrder != nil {
		summaryLines := pdf.SplitLines([]byte(workOrder.Summary), 170.0)
		lineHeight := 4.0
		summaryHeight := float64(len(summaryLines)) * lineHeight
		cardHeight := 28.0 + summaryHeight

		checkPageBreak(cardHeight + 10.0)
		startY := pdf.GetY()

		// Section Header
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 6, "1. WORK ORDER DETAILS")
		pdf.Ln(7)
		
		startY = pdf.GetY()

		// Borderless card with light grey background
		pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
		pdf.SetDrawColor(lineR, lineG, lineB)
		pdf.SetLineWidth(0.2)
		pdf.Rect(15, startY, 180, cardHeight, "DF")

		// Left accent bar
		pdf.SetFillColor(primaryR, primaryG, primaryB)
		pdf.Rect(15, startY, 1.5, cardHeight, "F")

		// Row 1
		pdf.SetXY(18, startY+3.0)
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.Cell(30, 5, "Work Order ID")
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.Cell(60, 5, ": "+workOrder.ID.String())

		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.Cell(30, 5, "Job Type")
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		woType := "Standard"
		if workOrder.Type != "" {
			woType = workOrder.Type
		}
		pdf.Cell(0, 5, ": "+woType)

		// Row 2
		pdf.SetXY(18, startY+9.0)
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.Cell(30, 5, "Priority")
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		woPriority := "Normal"
		if workOrder.Priority != "" {
			woPriority = workOrder.Priority
		}
		pdf.Cell(60, 5, ": "+woPriority)

		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.Cell(30, 5, "Status")
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.Cell(0, 5, ": "+workOrder.Status)

		// Row 3
		pdf.SetXY(18, startY+15.0)
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.Cell(30, 5, "Due Date")
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		dueDateStr := "N/A"
		if workOrder.DueDate != nil {
			dueDateStr = workOrder.DueDate.Format("02/01/2006")
		}
		pdf.Cell(60, 5, ": "+dueDateStr)

		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.Cell(30, 5, "Created At")
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.Cell(0, 5, ": "+workOrder.CreatedAt.Format("02/01/2006"))

		// Row 4: Summary / Description
		pdf.SetXY(18, startY+21.0)
		pdf.SetFont("Arial", "B", 8)
		pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
		pdf.Cell(30, 5, "Description")
		pdf.SetFont("Arial", "", 8.5)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.SetXY(18, startY+25.0)
		
		for _, line := range summaryLines {
			pdf.SetX(18)
			pdf.Cell(170, lineHeight, string(line))
			pdf.Ln(lineHeight)
		}

		pdf.SetY(startY + cardHeight + 6.0)
	}

	// 2. Service Appointment Details Section
	if len(appointments) > 0 {
		checkPageBreak(15.0)
		
		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 6, "2. SERVICE APPOINTMENT DETAILS")
		pdf.Ln(7)

		for _, appt := range appointments {
			techsStr := "N/A"
			if len(appt.TechnicianNames) > 0 {
				techsStr = strings.Join(appt.TechnicianNames, ", ")
			} else if len(technicians) > 0 {
				// fallback to combined technicians list
				techsStr = strings.Join(technicians, ", ")
			}

			var notesLines [][]byte
			notesHeight := 0.0
			if appt.Notes != "" {
				notesLines = pdf.SplitLines([]byte(appt.Notes), 170.0)
				notesHeight = float64(len(notesLines))*4.0 + 4.0
			}

			cardHeight := 28.0 + notesHeight
			checkPageBreak(cardHeight + 6.0)
			startY := pdf.GetY()

			// Borderless card with light grey background
			pdf.SetFillColor(bgLightR, bgLightG, bgLightB)
			pdf.SetDrawColor(lineR, lineG, lineB)
			pdf.SetLineWidth(0.2)
			pdf.Rect(15, startY, 180, cardHeight, "DF")

			// Left accent bar
			pdf.SetFillColor(primaryR, primaryG, primaryB)
			pdf.Rect(15, startY, 1.5, cardHeight, "F")

			// Header line inside card
			pdf.SetXY(18, startY+3.0)
			pdf.SetFont("Arial", "B", 9)
			pdf.SetTextColor(primaryR, primaryG, primaryB)
			pdf.Cell(0, 5, "Service Appointment Details")

			// Row 1
			pdf.SetXY(18, startY+9.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(30, 5, "Appointment Number")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.Cell(60, 5, ": "+appt.AppointmentNumber)

			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(30, 5, "Status")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.Cell(0, 5, ": "+appt.Status)

			// Row 2: Actual Start and Actual End
			pdf.SetXY(18, startY+15.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(30, 5, "Actual Start")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			actStartStr := "N/A"
			if appt.ActualStartTime != nil {
				actStartStr = appt.ActualStartTime.Format("02/01/2006 15:04")
			}
			pdf.Cell(60, 5, ": "+actStartStr)

			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(30, 5, "Actual End")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			actEndStr := "N/A"
			if appt.ActualEndTime != nil {
				actEndStr = appt.ActualEndTime.Format("02/01/2006 15:04")
			}
			pdf.Cell(0, 5, ": "+actEndStr)

			// Row 3: Total Time Worked and Technicians
			pdf.SetXY(18, startY+21.0)
			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(30, 5, "Total Time Worked")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			
			durationStr := "N/A"
			if appt.ActualStartTime != nil && appt.ActualEndTime != nil {
				diff := appt.ActualEndTime.Sub(*appt.ActualStartTime)
				hours := int(diff.Hours())
				minutes := int(diff.Minutes()) % 60
				if hours > 0 {
					durationStr = fmt.Sprintf("%d hrs %d mins", hours, minutes)
				} else {
					durationStr = fmt.Sprintf("%d mins", minutes)
				}
			}
			pdf.Cell(60, 5, ": "+durationStr)

			pdf.SetFont("Arial", "B", 8)
			pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
			pdf.Cell(30, 5, "Technicians")
			pdf.SetFont("Arial", "", 8.5)
			pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
			pdf.Cell(0, 5, ": "+techsStr)

			// Row 4: Notes
			if appt.Notes != "" {
				pdf.SetXY(18, startY+27.0)
				pdf.SetFont("Arial", "B", 8)
				pdf.SetTextColor(textMutedR, textMutedG, textMutedB)
				pdf.Cell(30, 4, "Appointment Notes")
				pdf.SetFont("Arial", "", 8.5)
				pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
				pdf.SetXY(18, startY+30.0)
				
				for _, line := range notesLines {
					pdf.SetX(18)
					pdf.Cell(170, 4.0, string(line))
					pdf.Ln(4.0)
				}
			}

			pdf.SetY(startY + cardHeight + 5.0)
		}
		pdf.Ln(2)
	}

	// Separate services and parts
	var services []domain.InvoiceItem
	var parts []domain.InvoiceItem

	for _, item := range invoice.Items {
		if item.ItemType == "part" {
			parts = append(parts, item)
		} else {
			services = append(services, item)
		}
	}

	// 3. Services Provided Section
	if len(services) > 0 {
		checkPageBreak(25.0)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 6, "3. SERVICES PROVIDED")
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

	// 4. Parts Used Section
	if len(parts) > 0 {
		checkPageBreak(25.0)

		pdf.SetFont("Arial", "B", 10)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 6, "4. PARTS USED")
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
	
	labelX := 110.0
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
	if invoice.Notes != "" {
		notesLines := pdf.SplitLines([]byte(invoice.Notes), 180.0)
		notesH := float64(len(notesLines))*4.0 + 10.0
		checkPageBreak(notesH)
		pdf.SetY(pdf.GetY() + 6)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 5, "Notes")
		pdf.Ln(5)
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.MultiCell(180, 4, invoice.Notes, "", "L", false)
	}

	if invoice.Terms != "" {
		termsLines := pdf.SplitLines([]byte(invoice.Terms), 180.0)
		termsH := float64(len(termsLines))*4.0 + 10.0
		checkPageBreak(termsH)
		pdf.SetY(pdf.GetY() + 6)
		pdf.SetFont("Arial", "B", 9)
		pdf.SetTextColor(primaryR, primaryG, primaryB)
		pdf.Cell(0, 5, "Terms & Conditions")
		pdf.Ln(5)
		pdf.SetFont("Arial", "", 8)
		pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
		pdf.MultiCell(180, 4, invoice.Terms, "", "L", false)
	}

	checkPageBreak(25.0)
	pdf.SetY(-35)
	pdf.SetX(130)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(textDarkR, textDarkG, textDarkB)
	pdf.CellFormat(65, 5, "Authorized Signature", "T", 1, "C", false, 0, "")

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

	if street == "" {
		parts := make([]string, 0)
		if city != "" {
			parts = append(parts, city)
		}
		if state != "" {
			parts = append(parts, state)
		}
		if code != "" {
			parts = append(parts, code)
		}
		
		addr := ""
		if len(parts) > 0 {
			addr = "  " + strings.Join(parts, ", ")
		}
		if country != "" {
			if addr != "" {
				addr += "\n"
			}
			addr += "  " + country
		}
		return addr
	}

	streetLines := strings.Split(street, "\n")
	formattedStreetLines := make([]string, 0, len(streetLines))
	for _, line := range streetLines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			formattedStreetLines = append(formattedStreetLines, "  " + trimmed)
		}
	}
	addr := strings.Join(formattedStreetLines, "\n")
	
	extraParts := make([]string, 0)
	if city != "" && !strings.Contains(strings.ToLower(street), strings.ToLower(city)) {
		extraParts = append(extraParts, city)
	}
	if state != "" && !strings.Contains(strings.ToLower(street), strings.ToLower(state)) {
		extraParts = append(extraParts, state)
	}
	if code != "" && !strings.Contains(strings.ToLower(street), strings.ToLower(code)) {
		extraParts = append(extraParts, code)
	}
	
	if len(extraParts) > 0 {
		addr += "\n  " + strings.Join(extraParts, ", ")
	}
	
	if country != "" && !strings.Contains(strings.ToLower(street), strings.ToLower(country)) {
		addr += "\n  " + country
	}
	
	return addr
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
