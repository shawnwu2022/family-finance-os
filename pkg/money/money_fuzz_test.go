package money

import "testing"

func FuzzMoneyAddZeroIdentity(f *testing.F) {
	f.Add(int64(0), "CNY")
	f.Add(int64(12345), "CNY")
	f.Add(int64(-98765), "USD")

	f.Fuzz(func(t *testing.T, minor int64, currency string) {
		value := Money{Minor: minor, Currency: currency}
		zero := Money{Minor: 0, Currency: currency}

		got, err := value.Add(zero)
		if err != nil {
			t.Fatalf("Add(zero) error = %v", err)
		}
		if got != value {
			t.Fatalf("Add(zero) = %#v, want %#v", got, value)
		}
	})
}
