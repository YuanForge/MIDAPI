package billing

import (
	"reflect"
	"testing"

	"fanapi/internal/model"
)

func TestQuotaReserveNeededOnlyFillsActualGap(t *testing.T) {
	tests := []struct {
		name            string
		required        int64
		activeRemaining int64
		want            int64
	}{
		{name: "no active quota", required: 36_000, activeRemaining: 0, want: 36_000},
		{name: "partial active quota", required: 36_000, activeRemaining: 10_000, want: 26_000},
		{name: "active quota already enough", required: 36_000, activeRemaining: 36_000, want: 0},
		{name: "active quota above requirement", required: 36_000, activeRemaining: 90_000, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quotaReserveNeeded(tt.required, tt.activeRemaining); got != tt.want {
				t.Fatalf("quotaReserveNeeded(%d, %d) = %d, want %d", tt.required, tt.activeRemaining, got, tt.want)
			}
		})
	}
}

func TestQuotaLeaseDebitPlanSpansActiveLeases(t *testing.T) {
	leases := []model.BillingQuotaLease{
		{ID: 1, RemainingCredits: 30},
		{ID: 2, RemainingCredits: 0},
		{ID: 3, RemainingCredits: 50},
		{ID: 4, RemainingCredits: 40},
	}

	got, ok := quotaLeaseDebitPlan(leases, 95)
	if !ok {
		t.Fatal("quotaLeaseDebitPlan returned ok=false")
	}

	want := []quotaLeaseDebit{
		{ID: 1, Amount: 30},
		{ID: 3, Amount: 50},
		{ID: 4, Amount: 15},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("quotaLeaseDebitPlan() = %#v, want %#v", got, want)
	}
}

func TestQuotaLeaseDebitPlanReportsShortfall(t *testing.T) {
	leases := []model.BillingQuotaLease{
		{ID: 1, RemainingCredits: 30},
		{ID: 2, RemainingCredits: 50},
	}

	got, ok := quotaLeaseDebitPlan(leases, 95)
	if ok {
		t.Fatalf("quotaLeaseDebitPlan returned ok=true with plan %#v", got)
	}
}

func TestQuotaCacheVersionIsBumped(t *testing.T) {
	if quotaCacheVersion == "" {
		t.Fatal("quotaCacheVersion must not be empty")
	}
	if quotaCacheVersion == "1" {
		t.Fatal("quotaCacheVersion must move past legacy unversioned cache")
	}
}
