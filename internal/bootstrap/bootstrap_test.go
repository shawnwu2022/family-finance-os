package bootstrap

import (
	"errors"
	"testing"
)

func TestValidateCanonicalizesWhitespaceAndRejectsUnsafeDefaults(t *testing.T) {
	input, err := Validate(Input{
		Name: " 家庭 ", Currency: "CNY", Timezone: "Asia/Shanghai", Period: "2026-08", LiquidityFloorMinor: 500_000,
	})
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if input.Name != "家庭" {
		t.Fatalf("name = %q", input.Name)
	}

	tests := []Input{
		{Name: "", Currency: "CNY", Timezone: "Asia/Shanghai", Period: "2026-08"},
		{Name: "家庭", Currency: "cny", Timezone: "Asia/Shanghai", Period: "2026-08"},
		{Name: "家庭", Currency: "CNY", Timezone: "Missing/Timezone", Period: "2026-08"},
		{Name: "家庭", Currency: "CNY", Timezone: "Asia/Shanghai", Period: "2026-13"},
		{Name: "家庭", Currency: "CNY", Timezone: "Asia/Shanghai", Period: "2026-08", LiquidityFloorMinor: -1},
	}
	for _, test := range tests {
		if _, err := Validate(test); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("Validate(%#v) error = %v, want ErrInvalidInput", test, err)
		}
	}
}
