package application

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
)

// ConvertToUTC interprets the given time value t as being in the wall-clock time 
// of the provided IANA timezone name and returns the equivalent UTC instant.
// If the time is zero or the timezone is empty/invalid, it returns t in UTC.
func ConvertToUTC(t time.Time, timezone string) time.Time {
	if t.IsZero() || timezone == "" {
		return t.UTC()
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Printf("[timezone_utils] WARNING: unknown timezone %q, treating time as UTC: %v\n", timezone, err)
		return t.UTC()
	}

	// Re-interpret the wall-clock reading of t in the given location,
	// then convert to UTC.
	localTime := time.Date(
		t.Year(), t.Month(), t.Day(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(),
		loc,
	)
	return localTime.UTC()
}

// ConvertUTCToOrgTimezone converts a UTC time value to the local time of the
// organization identified by organizationID.
func ConvertUTCToOrgTimezone(
	ctx context.Context,
	organizationID uuid.UUID,
	utcTime time.Time,
	rmRepo domain.ReadModelRepository,
) (time.Time, error) {
	if utcTime.IsZero() {
		return utcTime, nil
	}

	if rmRepo == nil {
		return utcTime.UTC(), fmt.Errorf("ConvertUTCToOrgTimezone: rmRepo is nil")
	}

	org, err := rmRepo.GetOrganization(ctx, organizationID)
	if err != nil {
		return utcTime.UTC(), fmt.Errorf("ConvertUTCToOrgTimezone: failed to fetch organization %q: %w", organizationID, err)
	}

	return ConvertUTCToTimeZone(utcTime, org.Timezone), nil
}

// ConvertUTCToTimeZone converts a UTC instant to a specific IANA timezone.
func ConvertUTCToTimeZone(utcTime time.Time, timezone string) time.Time {
	if utcTime.IsZero() || timezone == "" {
		return utcTime.UTC()
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		fmt.Printf("[timezone_utils] WARNING: unknown timezone %q, returning UTC time: %v\n", timezone, err)
		return utcTime.UTC()
	}

	return utcTime.In(loc)
}

// ConvertToOrgTZValue converts a stored UTC time.Time back to the org's local timezone.
func ConvertToOrgTZValue(ctx context.Context, t time.Time, orgID uuid.UUID, rmRepo domain.ReadModelRepository) time.Time {
	if t.IsZero() || rmRepo == nil || orgID == uuid.Nil {
		return t.UTC()
	}
	local, err := ConvertUTCToOrgTimezone(ctx, orgID, t, rmRepo)
	if err != nil {
		return t.UTC()
	}
	return local
}

// ConvertToOrgTZ converts a stored UTC *time.Time back to the org's local timezone.
func ConvertToOrgTZ(ctx context.Context, t *time.Time, orgID uuid.UUID, rmRepo domain.ReadModelRepository) *time.Time {
	if t == nil {
		return nil
	}
	local := ConvertToOrgTZValue(ctx, *t, orgID, rmRepo)
	return &local
}
