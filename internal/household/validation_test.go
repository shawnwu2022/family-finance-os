package household

import (
	"testing"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func TestNormalizeCurrency(t *testing.T) {
	t.Parallel()

	got, err := normalizeCurrency(" cny ")
	if err != nil {
		t.Fatalf("normalizeCurrency: %v", err)
	}
	if got != "CNY" {
		t.Fatalf("currency = %q, want CNY", got)
	}

	for _, invalid := range []string{"", "CN", "CNY1", "C-1"} {
		if _, err := normalizeCurrency(invalid); err == nil {
			t.Fatalf("normalizeCurrency(%q) succeeded", invalid)
		}
	}
}

func TestValidateNewHouseholdRejectsInvalidTimezone(t *testing.T) {
	t.Parallel()

	_, err := validateNewHousehold(NewHousehold{
		Name:         "测试家庭",
		BaseCurrency: "CNY",
		Timezone:     "Not/A_Real_Zone",
	})
	if err == nil {
		t.Fatal("validateNewHousehold accepted invalid timezone")
	}
}

func TestValidateMoneyForBase(t *testing.T) {
	t.Parallel()

	if err := validateMoneyForBase(money.Money{Minor: 100, Currency: "CNY"}, "CNY"); err != nil {
		t.Fatalf("valid money rejected: %v", err)
	}
	if err := validateMoneyForBase(money.Money{Minor: -1, Currency: "CNY"}, "CNY"); err == nil {
		t.Fatal("negative planning amount accepted")
	}
	if err := validateMoneyForBase(money.Money{Minor: 100, Currency: "USD"}, "CNY"); err == nil {
		t.Fatal("non-base currency accepted")
	}
}

func TestEnumValidation(t *testing.T) {
	t.Parallel()

	if !MemberKindAdult.valid() || !MemberKindChild.valid() || !MemberKindDependent.valid() {
		t.Fatal("known member kind rejected")
	}
	if MemberKind("other").valid() {
		t.Fatal("unknown member kind accepted")
	}

	if !CadenceMonthly.valid() || !CadenceAnnual.valid() || !CadenceIrregular.valid() {
		t.Fatal("known cadence rejected")
	}
	if Cadence("weekly").valid() {
		t.Fatal("unsupported cadence accepted")
	}

	if !IncomeStabilityStable.valid() || !IncomeStabilityVariable.valid() || !IncomeStabilityIrregular.valid() {
		t.Fatal("known stability rejected")
	}
	if IncomeStability("guaranteed").valid() {
		t.Fatal("unknown stability accepted")
	}
}
