package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/outbound"

	"github.com/google/uuid"
)

type CustomerHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewCustomerHTTPClient(baseURL string) outbound.CustomerClient {
	return &CustomerHTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *CustomerHTTPClient) GetCustomer(ctx context.Context, id uuid.UUID) (*domain.CustomerRM, error) {
	url := fmt.Sprintf("%s/api/v1/customers/%s", c.baseURL, id.String())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// This assumes internal service-to-service communication doesn't need complex auth or uses a shared secret/network trust
	// If needed, add headers here.

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil // Not found is valid result
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("customer service returned status: %d", resp.StatusCode)
	}

	// The response structure from Customer Service might differ slightly, but let's assume it maps somewhat or we map it.
	// We need to map the external response to our CustomerRM
	var externalCust struct {
		ID             uuid.UUID `json:"id"`
		OrganizationID uuid.UUID `json:"organization_id"`
		DisplayName    string    `json:"display_name"`
		CompanyName    string    `json:"company_name"`
		Email          string    `json:"email"`
		PhoneWork      string    `json:"phone_work"`
		// Address fields might be nested or flat, adjusting based on typical patterns.
		// Assuming flat or we might need to adjust.
		Street1 string `json:"street1"`
		City    string `json:"city"`
		State   string `json:"state"`
		ZipCode string `json:"zip_code"`
		Country string `json:"country"`

		ShippingStreet1 string `json:"shipping_street1"`
		ShippingCity    string `json:"shipping_city"`
		ShippingState   string `json:"shipping_state"`
		ShippingZipCode string `json:"shipping_zip_code"`
		ShippingCountry string `json:"shipping_country"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&externalCust); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &domain.CustomerRM{
		ID:              externalCust.ID,
		OrganizationID:  externalCust.OrganizationID,
		DisplayName:     externalCust.DisplayName,
		CompanyName:     externalCust.CompanyName,
		Email:           externalCust.Email,
		Phone:           externalCust.PhoneWork,
		BillingStreet:   externalCust.Street1,
		BillingCity:     externalCust.City,
		BillingState:    externalCust.State,
		BillingCode:     externalCust.ZipCode,
		BillingCountry:  externalCust.Country,
		ShippingStreet:  externalCust.ShippingStreet1,
		ShippingCity:    externalCust.ShippingCity,
		ShippingState:   externalCust.ShippingState,
		ShippingCode:    externalCust.ShippingZipCode,
		ShippingCountry: externalCust.ShippingCountry,
		UpdatedAt:       time.Now(), // Fresh fetch
	}, nil
}

func (c *CustomerHTTPClient) GetContact(ctx context.Context, id uuid.UUID) (*domain.ContactRM, error) {
	// Assuming contacts are fetched similarly
	url := fmt.Sprintf("%s/api/v1/contacts/%s", c.baseURL, id.String())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("customer service returned status: %d", resp.StatusCode)
	}

	var externalContact struct {
		ID         uuid.UUID `json:"id"`
		CustomerID uuid.UUID `json:"customer_id"`
		FirstName  string    `json:"first_name"`
		LastName   string    `json:"last_name"`
		Email      string    `json:"email"`
		Phone      string    `json:"phone"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&externalContact); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &domain.ContactRM{
		ID:         externalContact.ID,
		CustomerID: externalContact.CustomerID,
		FirstName:  externalContact.FirstName,
		LastName:   externalContact.LastName,
		Email:      externalContact.Email,
		Phone:      externalContact.Phone,
		UpdatedAt:  time.Now(),
	}, nil
}
