package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReadModelRepository struct {
	db *gorm.DB
}

func NewReadModelRepository(db *gorm.DB) *ReadModelRepository {
	return &ReadModelRepository{db: db}
}

func (r *ReadModelRepository) GetCustomer(ctx context.Context, id uuid.UUID) (*domain.CustomerRM, error) {
	var rm domain.CustomerRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *ReadModelRepository) SearchCustomers(ctx context.Context, orgID uuid.UUID, query string) ([]domain.CustomerRM, error) {
	var res []domain.CustomerRM
	q := "%" + query + "%"
	err := r.db.WithContext(ctx).Where("organization_id = ? AND (display_name ILIKE ? OR company_name ILIKE ?)", orgID, q, q).Limit(20).Find(&res).Error
	return res, err
}

func (r *ReadModelRepository) GetItem(ctx context.Context, id uuid.UUID) (*domain.ItemRM, error) {
	var rm domain.ItemRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *ReadModelRepository) SearchItems(ctx context.Context, orgID uuid.UUID, query string) ([]domain.ItemRM, error) {
	// Call the serviceandparts service API to get items
	baseURL := "http://localhost:8087/api/v1/items"
	params := url.Values{}
	params.Add("organization_id", orgID.String())
	if query != "" {
		params.Add("search", query)
	}
	params.Add("limit", "20")

	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call serviceandparts API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("serviceandparts API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var items []struct {
		ID             string                 `json:"id"`
		OrganizationID string                 `json:"organization_id"`
		SKU            string                 `json:"sku"`
		Name           string                 `json:"name"`
		Type           string                 `json:"type"`
		SalesInfo      map[string]interface{} `json:"sales_info"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to domain.ItemRM
	var res []domain.ItemRM
	for _, item := range items {
		itemID, _ := uuid.Parse(item.ID)
		orgID, _ := uuid.Parse(item.OrganizationID)

		price := 0.0
		if salesInfo, ok := item.SalesInfo["selling_price"].(float64); ok {
			price = salesInfo
		}

		res = append(res, domain.ItemRM{
			ID:             itemID,
			OrganizationID: orgID,
			SKU:            item.SKU,
			Name:           item.Name,
			ItemType:       item.Type,
			Price:          price,
		})
	}

	return res, nil
}

func (r *ReadModelRepository) GetContact(ctx context.Context, id uuid.UUID) (*domain.ContactRM, error) {
	var rm domain.ContactRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *ReadModelRepository) SearchContacts(ctx context.Context, orgID uuid.UUID, customerID uuid.UUID, query string) ([]domain.ContactRM, error) {
	var res []domain.ContactRM
	q := "%" + query + "%"
	db := r.db.WithContext(ctx).Where("organization_id = ?", orgID)
	if customerID != uuid.Nil {
		db = db.Where("customer_id = ?", customerID)
	}
	err := db.Where("(first_name ILIKE ? OR last_name ILIKE ? OR email ILIKE ?)", q, q, q).Limit(20).Find(&res).Error
	return res, err
}
