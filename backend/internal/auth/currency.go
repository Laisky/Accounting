package auth

import (
	"strings"

	"github.com/Laisky/errors/v2"
)

// DefaultBaseCurrency is the user preference used when older records have no saved currency.
const DefaultBaseCurrency = "USD"

var supportedBaseCurrencies = map[string]struct{}{
	"CAD": {},
	"CNY": {},
	"EUR": {},
	"USD": {},
}

// NormalizeBaseCurrency receives a currency code and returns a supported uppercase profile currency.
func NormalizeBaseCurrency(value string) (string, error) {
	currency := strings.ToUpper(strings.TrimSpace(value))
	if _, ok := supportedBaseCurrencies[currency]; !ok {
		return "", errors.WithStack(errors.New("base currency is not supported"))
	}

	return currency, nil
}
