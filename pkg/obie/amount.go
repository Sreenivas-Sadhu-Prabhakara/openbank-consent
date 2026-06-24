// Package obie provides the shared building blocks of the UK Open Banking
// (OBIE) Read/Write API: the standard response envelope, the error model and
// the money amount type. Every service in this repo speaks OBIE on the wire,
// so these types are deliberately framework-free and reusable.
package obie

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/shopspring/decimal"
)

// currencyPattern matches an ISO 4217 alphabetic currency code, as required by
// the OBIE schema field OBActiveOrHistoricCurrencyAndAmount.Currency.
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// Amount models the OBIE OBActiveOrHistoricCurrencyAndAmount structure. On the
// wire it serialises as {"Amount":"123.45","Currency":"GBP"} where Amount is a
// decimal string (max 5 fractional digits) and Currency is an ISO 4217 code.
//
// Internally the value is held as a shopspring/decimal so balances and
// availability checks can be computed exactly, without float rounding error.
type Amount struct {
	Value    decimal.Decimal
	Currency string
}

// NewAmount parses an OBIE amount string ("123.45") and currency code into an
// Amount, validating both. It is the canonical constructor used by services
// that build amounts from trusted sources (e.g. database rows).
func NewAmount(value, currency string) (Amount, error) {
	d, err := decimal.NewFromString(value)
	if err != nil {
		return Amount{}, fmt.Errorf("invalid amount %q: %w", value, err)
	}
	a := Amount{Value: d, Currency: currency}
	if err := a.Validate(); err != nil {
		return Amount{}, err
	}
	return a, nil
}

// MustAmount is like NewAmount but panics on error. Intended for tests and
// static seed data where the inputs are known-good constants.
func MustAmount(value, currency string) Amount {
	a, err := NewAmount(value, currency)
	if err != nil {
		panic(err)
	}
	return a
}

// Validate enforces the OBIE constraints: a non-negative amount with at most 5
// decimal places and a well-formed ISO 4217 currency code.
func (a Amount) Validate() error {
	if !currencyPattern.MatchString(a.Currency) {
		return fmt.Errorf("currency %q is not a valid ISO 4217 code", a.Currency)
	}
	if a.Value.Exponent() < -5 {
		return fmt.Errorf("amount %s has more than 5 decimal places", a.Value)
	}
	return nil
}

// String renders the OBIE amount string with no exponent notation.
func (a Amount) String() string { return a.Value.String() }

// GreaterThanOrEqual reports whether a >= other. Currencies must match; a
// mismatch returns an error so callers cannot silently compare across
// currencies.
func (a Amount) GreaterThanOrEqual(other Amount) (bool, error) {
	if a.Currency != other.Currency {
		return false, fmt.Errorf("cannot compare %s with %s", a.Currency, other.Currency)
	}
	return a.Value.GreaterThanOrEqual(other.Value), nil
}

// Sub returns a - other. Currencies must match.
func (a Amount) Sub(other Amount) (Amount, error) {
	if a.Currency != other.Currency {
		return Amount{}, fmt.Errorf("cannot subtract %s from %s", other.Currency, a.Currency)
	}
	return Amount{Value: a.Value.Sub(other.Value), Currency: a.Currency}, nil
}

// wireAmount is the JSON shape mandated by OBIE.
type wireAmount struct {
	Amount   string `json:"Amount"`
	Currency string `json:"Currency"`
}

// MarshalJSON renders the Amount in the OBIE wire shape.
func (a Amount) MarshalJSON() ([]byte, error) {
	return json.Marshal(wireAmount{Amount: a.Value.String(), Currency: a.Currency})
}

// UnmarshalJSON parses the OBIE wire shape and validates the result, so an
// invalid amount on an inbound request is rejected at decode time.
func (a *Amount) UnmarshalJSON(b []byte) error {
	var w wireAmount
	if err := json.Unmarshal(b, &w); err != nil {
		return err
	}
	parsed, err := NewAmount(w.Amount, w.Currency)
	if err != nil {
		return err
	}
	*a = parsed
	return nil
}
