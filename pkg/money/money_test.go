package money

import (
	"errors"
	"math"
	"testing"
)

func TestMoneyAdd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		left    Money
		right   Money
		want    Money
		wantErr error
	}{
		{
			name:  "same currency positive values",
			left:  Money{Minor: 12345, Currency: "CNY"},
			right: Money{Minor: 655, Currency: "CNY"},
			want:  Money{Minor: 13000, Currency: "CNY"},
		},
		{
			name:  "same currency negative value",
			left:  Money{Minor: 1000, Currency: "CNY"},
			right: Money{Minor: -250, Currency: "CNY"},
			want:  Money{Minor: 750, Currency: "CNY"},
		},
		{
			name:    "currency mismatch",
			left:    Money{Minor: 100, Currency: "CNY"},
			right:   Money{Minor: 100, Currency: "USD"},
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:    "positive overflow",
			left:    Money{Minor: math.MaxInt64, Currency: "CNY"},
			right:   Money{Minor: 1, Currency: "CNY"},
			wantErr: ErrOverflow,
		},
		{
			name:    "negative overflow",
			left:    Money{Minor: math.MinInt64, Currency: "CNY"},
			right:   Money{Minor: -1, Currency: "CNY"},
			wantErr: ErrOverflow,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.left.Add(tt.right)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Add() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Add() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Add() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestMoneySub(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		left    Money
		right   Money
		want    Money
		wantErr error
	}{
		{
			name:  "same currency positive values",
			left:  Money{Minor: 13000, Currency: "CNY"},
			right: Money{Minor: 655, Currency: "CNY"},
			want:  Money{Minor: 12345, Currency: "CNY"},
		},
		{
			name:  "subtract negative value",
			left:  Money{Minor: 1000, Currency: "CNY"},
			right: Money{Minor: -250, Currency: "CNY"},
			want:  Money{Minor: 1250, Currency: "CNY"},
		},
		{
			name:    "currency mismatch",
			left:    Money{Minor: 100, Currency: "CNY"},
			right:   Money{Minor: 100, Currency: "USD"},
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:    "negative overflow",
			left:    Money{Minor: math.MinInt64, Currency: "CNY"},
			right:   Money{Minor: 1, Currency: "CNY"},
			wantErr: ErrOverflow,
		},
		{
			name:    "positive overflow",
			left:    Money{Minor: math.MaxInt64, Currency: "CNY"},
			right:   Money{Minor: -1, Currency: "CNY"},
			wantErr: ErrOverflow,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.left.Sub(tt.right)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Sub() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Sub() unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Sub() = %#v, want %#v", got, tt.want)
			}
		})
	}
}
