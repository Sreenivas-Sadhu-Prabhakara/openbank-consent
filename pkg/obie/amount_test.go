package obie

import (
	"encoding/json"
	"testing"
)

func TestNewAmount(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		currency string
		wantErr  bool
	}{
		{"valid whole", "100", "GBP", false},
		{"valid two dp", "100.50", "GBP", false},
		{"valid five dp", "0.00001", "EUR", false},
		{"zero", "0", "USD", false},
		{"six dp rejected", "0.000001", "GBP", true},
		{"bad currency lowercase", "10.00", "gbp", true},
		{"bad currency length", "10.00", "GB", true},
		{"non-numeric amount", "abc", "GBP", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAmount(tt.value, tt.currency)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewAmount(%q,%q) err=%v, wantErr=%v", tt.value, tt.currency, err, tt.wantErr)
			}
		})
	}
}

func TestAmountJSONRoundTrip(t *testing.T) {
	a := MustAmount("123.45", "GBP")
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"Amount":"123.45","Currency":"GBP"}`
	if string(b) != want {
		t.Fatalf("marshal = %s, want %s", b, want)
	}

	var got Amount
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.String() != "123.45" || got.Currency != "GBP" {
		t.Fatalf("round trip = %s %s", got, got.Currency)
	}
}

func TestAmountUnmarshalRejectsInvalid(t *testing.T) {
	var a Amount
	err := json.Unmarshal([]byte(`{"Amount":"1.000000","Currency":"GBP"}`), &a)
	if err == nil {
		t.Fatal("expected error for >5 dp amount, got nil")
	}
}

func TestAmountComparisonAndSub(t *testing.T) {
	bal := MustAmount("100.00", "GBP")
	req := MustAmount("40.00", "GBP")

	ok, err := bal.GreaterThanOrEqual(req)
	if err != nil || !ok {
		t.Fatalf("100 >= 40 should hold: ok=%v err=%v", ok, err)
	}

	remaining, err := bal.Sub(req)
	if err != nil {
		t.Fatalf("sub: %v", err)
	}
	if remaining.String() != "60" {
		t.Fatalf("100 - 40 = %s, want 60", remaining)
	}

	// Currency mismatch must error rather than silently compare.
	if _, err := bal.GreaterThanOrEqual(MustAmount("1", "EUR")); err == nil {
		t.Fatal("expected currency-mismatch error")
	}
}
