package household

import (
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/shawnwu2022/family-finance-os/pkg/money"
)

func normalizeCurrency(raw string) (string, error) {
	currency := strings.ToUpper(strings.TrimSpace(raw))
	if len(currency) != 3 {
		return "", fmt.Errorf("currency must be a 3-letter code")
	}
	for i := 0; i < len(currency); i++ {
		if currency[i] < 'A' || currency[i] > 'Z' {
			return "", fmt.Errorf("currency must contain only A-Z")
		}
	}
	return currency, nil
}

func validateNewHousehold(input NewHousehold) (NewHousehold, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return NewHousehold{}, errors.New("household name is required")
	}
	currency, err := normalizeCurrency(input.BaseCurrency)
	if err != nil {
		return NewHousehold{}, fmt.Errorf("base currency: %w", err)
	}
	input.BaseCurrency = currency
	input.Timezone = strings.TrimSpace(input.Timezone)
	if input.Timezone == "" {
		return NewHousehold{}, errors.New("household timezone is required")
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return NewHousehold{}, fmt.Errorf("household timezone %q: %w", input.Timezone, err)
	}
	return input, nil
}

func validateMoneyForBase(amount money.Money, baseCurrency string) error {
	currency, err := normalizeCurrency(amount.Currency)
	if err != nil {
		return fmt.Errorf("amount currency: %w", err)
	}
	if currency != baseCurrency {
		return fmt.Errorf("amount currency %s must match household base currency %s", currency, baseCurrency)
	}
	if amount.Minor < 0 {
		return errors.New("planning amount must not be negative")
	}
	return nil
}

func validateName(name, label string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%s name is required", label)
	}
	return name, nil
}
