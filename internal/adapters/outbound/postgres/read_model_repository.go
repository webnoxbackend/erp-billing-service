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
	db               *gorm.DB
	inventoryBaseURL string
}

func NewReadModelRepository(db *gorm.DB, inventoryBaseURL string) *ReadModelRepository {
	return &ReadModelRepository{db: db, inventoryBaseURL: inventoryBaseURL}
}

func (r *ReadModelRepository) GetCustomer(ctx context.Context, id uuid.UUID) (*domain.CustomerRM, error) {
	var rm domain.CustomerRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	r.populateCustomerFlatFields(ctx, &rm)
	return &rm, nil
}

func (r *ReadModelRepository) SearchCustomers(ctx context.Context, orgID uuid.UUID, query string) ([]domain.CustomerRM, error) {
	var res []domain.CustomerRM
	q := "%" + query + "%"
	err := r.db.WithContext(ctx).Where("organization_id = ? AND (display_name ILIKE ? OR company_name ILIKE ?)", orgID, q, q).Limit(20).Find(&res).Error
	if err != nil {
		return nil, err
	}
	for i := range res {
		r.populateCustomerFlatFields(ctx, &res[i])
	}
	return res, nil
}

func (r *ReadModelRepository) populateCustomerFlatFields(ctx context.Context, rm *domain.CustomerRM) {
	if rm == nil {
		return
	}
	if rm.BillingAddressID != nil {
		var addr domain.AddressReadOnly
		if err := r.db.WithContext(ctx).First(&addr, "id = ?", *rm.BillingAddressID).Error; err == nil {
			rm.BillingStreet = addr.Street1
			if addr.Street2 != "" {
				rm.BillingStreet = addr.Street1 + ", " + addr.Street2
			}
			rm.BillingCity = addr.City
			rm.BillingState = addr.State
			rm.BillingCode = addr.PostalCode
			rm.BillingCountry = addr.Country
		}
	}
	if rm.ShippingAddressID != nil {
		var addr domain.AddressReadOnly
		if err := r.db.WithContext(ctx).First(&addr, "id = ?", *rm.ShippingAddressID).Error; err == nil {
			rm.ShippingStreet = addr.Street1
			if addr.Street2 != "" {
				rm.ShippingStreet = addr.Street1 + ", " + addr.Street2
			}
			rm.ShippingCity = addr.City
			rm.ShippingState = addr.State
			rm.ShippingCode = addr.PostalCode
			rm.ShippingCountry = addr.Country
		}
	}
	if rm.PhoneWork != "" {
		rm.Phone = rm.PhoneWork
	} else {
		rm.Phone = rm.PhoneMobile
	}
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
	baseURL := r.inventoryBaseURL + "/api/v1/items"
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

	// Parse the response directly into the new ItemRM struct slice
	var res []domain.ItemRM
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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

func (r *ReadModelRepository) GetPrimaryContact(ctx context.Context, customerID uuid.UUID) (*domain.ContactRM, error) {
	var rm domain.ContactRM
	err := r.db.WithContext(ctx).Where("customer_id = ? AND is_primary = ?", customerID, true).First(&rm).Error
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

func (r *ReadModelRepository) GetOrganization(ctx context.Context, id uuid.UUID) (*domain.OrganizationRM, error) {
	var rm domain.OrganizationRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	// Populate IconURL and Email from the admin/owner user
	var adminUser domain.UserReadOnly
	err = r.db.WithContext(ctx).
		Where("organization_id = ? AND LOWER(role) IN ?", id.String(), []string{"admin", "owner"}).
		Order("created_at ASC").
		First(&adminUser).Error
	if err != nil {
		// Try any user of the organization
		r.db.WithContext(ctx).
			Where("organization_id = ?", id.String()).
			Order("created_at ASC").
			First(&adminUser)
	}
	if adminUser.ProfilePhotoURL != "" {
		rm.IconURL = adminUser.ProfilePhotoURL
	}
	rm.Email = adminUser.Email
	return &rm, nil
}

func (r *ReadModelRepository) SearchWorkOrders(ctx context.Context, orgID uuid.UUID, query string) ([]domain.WorkOrderRM, error) {
	var res []domain.WorkOrderRM
	db := r.db.WithContext(ctx).
		Model(&domain.WorkOrderRM{}).
		Select("work_orders_readonly.*").
		Preload("Customer").
		Joins("JOIN service_appointments_readonly ON service_appointments_readonly.work_order_id = work_orders_readonly.id AND service_appointments_readonly.deleted_at IS NULL").
		Where("work_orders_readonly.organization_id = ? AND service_appointments_readonly.status = ?", orgID, "COMPLETED")

	if query != "" {
		q := "%" + query + "%"
		db = db.Where("work_orders_readonly.summary ILIKE ?", q)
	}
	err := db.Order("work_orders_readonly.updated_at DESC").Limit(30).Find(&res).Error
	return res, err
}

func (r *ReadModelRepository) GetWorkOrderByID(ctx context.Context, id uuid.UUID) (*domain.WorkOrderRM, error) {
	var rm domain.WorkOrderRM
	err := r.db.WithContext(ctx).
		Preload("Customer").
		Preload("ServiceLines").
		Preload("PartLines").
		First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *ReadModelRepository) GetOrganizationAdminID(ctx context.Context, orgID uuid.UUID) (*uuid.UUID, error) {
	var user domain.UserReadOnly
	// Try to find admin/owner first
	err := r.db.WithContext(ctx).
		Where("organization_id = ? AND LOWER(role) IN ?", orgID.String(), []string{"admin", "owner"}).
		Order("created_at ASC").
		First(&user).Error
	if err != nil {
		// Fallback to any user of the organization
		err = r.db.WithContext(ctx).
			Where("organization_id = ?", orgID.String()).
			Order("created_at ASC").
			First(&user).Error
		if err != nil {
			return nil, err
		}
	}
	parsedUUID, err := uuid.Parse(user.ID)
	if err != nil {
		return nil, err
	}
	return &parsedUUID, nil
}

func (r *ReadModelRepository) GetServiceAppointments(ctx context.Context, workOrderID uuid.UUID) ([]domain.ServiceAppointmentRM, error) {
	var appointments []domain.ServiceAppointmentRM
	err := r.db.WithContext(ctx).
		Where("work_order_id = ? AND (deleted_at IS NULL)", workOrderID).
		Find(&appointments).Error
	if err != nil {
		return nil, err
	}
	return appointments, nil
}

func (r *ReadModelRepository) GetTechnicianNamesForAppointment(ctx context.Context, appointmentID uuid.UUID) ([]string, error) {
	var names []string
	
	// 1. Join service_appointment_resources_readonly and users_readonly with string typecast (matching standard ID or workforce_user_id)
	err := r.db.WithContext(ctx).
		Table("service_appointment_resources_readonly").
		Select("users_readonly.full_name").
		Joins("JOIN users_readonly ON users_readonly.workforce_user_id::text = service_appointment_resources_readonly.resource_id::text OR users_readonly.id::text = service_appointment_resources_readonly.resource_id::text").
		Where("service_appointment_resources_readonly.service_appointment_id = ?", appointmentID).
		Where("service_appointment_resources_readonly.deleted_at IS NULL").
		Scan(&names).Error
	if err != nil {
		return nil, err
	}
	
	// 2. Also check if the appointment has AssignedTechnicianID directly on service_appointments_readonly
	var directName string
	err = r.db.WithContext(ctx).
		Table("service_appointments_readonly").
		Select("users_readonly.full_name").
		Joins("JOIN users_readonly ON users_readonly.workforce_user_id::text = service_appointments_readonly.assigned_technician_id::text OR users_readonly.id::text = service_appointments_readonly.assigned_technician_id::text").
		Where("service_appointments_readonly.id = ?", appointmentID).
		Where("service_appointments_readonly.deleted_at IS NULL").
		Scan(&directName).Error
	if err == nil && directName != "" {
		// Avoid duplicates
		exists := false
		for _, name := range names {
			if name == directName {
				exists = true
				break
			}
		}
		if !exists {
			names = append(names, directName)
		}
	}
	
	return names, nil
}

func (r *ReadModelRepository) GetTechniciansForAppointment(ctx context.Context, appointmentID uuid.UUID) ([]domain.UserReadOnly, error) {
	var technicians []domain.UserReadOnly
	
	// Query resources associated with the appointment
	err := r.db.WithContext(ctx).
		Table("users_readonly").
		Select("users_readonly.*").
		Joins("JOIN service_appointment_resources_readonly ON service_appointment_resources_readonly.resource_id::text = users_readonly.workforce_user_id::text OR service_appointment_resources_readonly.resource_id::text = users_readonly.id::text").
		Where("service_appointment_resources_readonly.service_appointment_id = ?", appointmentID).
		Where("service_appointment_resources_readonly.deleted_at IS NULL").
		Scan(&technicians).Error
	if err != nil {
		return nil, err
	}
	
	// Query direct technician if any
	var directTech domain.UserReadOnly
	err = r.db.WithContext(ctx).
		Table("users_readonly").
		Select("users_readonly.*").
		Joins("JOIN service_appointments_readonly ON service_appointments_readonly.assigned_technician_id::text = users_readonly.workforce_user_id::text OR service_appointments_readonly.assigned_technician_id::text = users_readonly.id::text").
		Where("service_appointments_readonly.id = ?", appointmentID).
		Where("service_appointments_readonly.deleted_at IS NULL").
		First(&directTech).Error
	if err == nil && directTech.ID != "" {
		exists := false
		for _, tech := range technicians {
			if tech.ID == directTech.ID {
				exists = true
				break
			}
		}
		if !exists {
			technicians = append(technicians, directTech)
		}
	}
	
	return technicians, nil
}

func (r *ReadModelRepository) GetCustomerIDsByEmailAndOrg(ctx context.Context, orgID uuid.UUID, email string) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Model(&domain.CustomerRM{}).
		Where("organization_id = ? AND LOWER(email) = LOWER(?)", orgID, email).
		Pluck("id", &ids).Error
	return ids, err
}

func (r *ReadModelRepository) GetAddress(ctx context.Context, id uuid.UUID) (*domain.AddressReadOnly, error) {
	var addr domain.AddressReadOnly
	if err := r.db.WithContext(ctx).First(&addr, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &addr, nil
}



