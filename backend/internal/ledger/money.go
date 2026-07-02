package ledger

import (
	"math/big"
	"regexp"
	"strings"

	"github.com/Laisky/errors/v2"
)

type exchangeRate struct {
	From string
	To   string
	Rate *big.Rat
}

var exchangeRatePattern = regexp.MustCompile(`^([A-Z]{3})/([A-Z]{3})=([0-9]+(?:\.[0-9]+)?)$`)

var supportedCurrencies = map[string]struct{}{
	"USD": {},
	"EUR": {},
	"CNY": {},
	"CAD": {},
}

// entryAmountInCurrencyCents receives an entry, target currency, and rates and returns converted target minor units.
func entryAmountInCurrencyCents(entry Entry, targetCurrency string, rates map[string]*big.Rat) (int64, error) {
	return convertAmountCents(entry.AmountCents, entry.TransactionCurrency, targetCurrency, entry.ExchangeRate, rates)
}

// convertAmountCents receives an amount, source currency, target currency, exchange metadata, and rates and returns converted cents.
func convertAmountCents(amountCents int64, sourceCurrency string, targetCurrency string, exchangeRateText string, rates map[string]*big.Rat) (int64, error) {
	sourceCurrency = strings.ToUpper(strings.TrimSpace(sourceCurrency))
	targetCurrency = strings.ToUpper(strings.TrimSpace(targetCurrency))
	if !isSupportedCurrency(sourceCurrency) {
		return 0, errors.Wrap(ErrInvalidInput, "source currency is invalid")
	}
	if !isSupportedCurrency(targetCurrency) {
		return 0, errors.Wrap(ErrInvalidInput, "target currency is invalid")
	}
	if sourceCurrency == targetCurrency {
		return amountCents, nil
	}

	amount := big.NewRat(amountCents, 1)
	if strings.TrimSpace(exchangeRateText) != "" {
		rate, err := parseExchangeRate(exchangeRateText)
		if err != nil {
			return 0, err
		}
		switch {
		case rate.From == sourceCurrency && rate.To == targetCurrency:
			amount.Mul(amount, rate.Rate)
		case rate.From == targetCurrency && rate.To == sourceCurrency:
			amount.Quo(amount, rate.Rate)
		default:
			return 0, errors.Wrapf(ErrInvalidInput, "exchange rate %s/%s does not convert %s to %s", rate.From, rate.To, sourceCurrency, targetCurrency)
		}

		return roundRatToInt64(amount), nil
	}

	sourceRate, ok := rates[sourceCurrency]
	if !ok || sourceRate.Sign() <= 0 {
		return 0, errors.Wrapf(ErrInvalidInput, "exchange rate for %s is missing", sourceCurrency)
	}
	targetRate, ok := rates[targetCurrency]
	if !ok || targetRate.Sign() <= 0 {
		return 0, errors.Wrapf(ErrInvalidInput, "exchange rate for %s is missing", targetCurrency)
	}
	amount.Mul(amount, targetRate)
	amount.Quo(amount, sourceRate)

	return roundRatToInt64(amount), nil
}

// parseExchangeRate receives exchange metadata and returns a normalized decimal conversion rate.
func parseExchangeRate(exchangeRateText string) (exchangeRate, error) {
	normalized := strings.ToUpper(strings.TrimSpace(exchangeRateText))
	matches := exchangeRatePattern.FindStringSubmatch(normalized)
	if len(matches) != 4 {
		return exchangeRate{}, errors.Wrap(ErrInvalidInput, "exchange rate must use FROM/TO=rate")
	}

	rate, ok := new(big.Rat).SetString(matches[3])
	if !ok || rate.Sign() <= 0 {
		return exchangeRate{}, errors.Wrap(ErrInvalidInput, "exchange rate must be positive")
	}

	return exchangeRate{
		From: matches[1],
		To:   matches[2],
		Rate: rate,
	}, nil
}

// isSupportedCurrency receives a currency code and reports whether this phase supports it.
func isSupportedCurrency(currency string) bool {
	_, ok := supportedCurrencies[strings.ToUpper(strings.TrimSpace(currency))]
	return ok
}

// roundRatToInt64 receives a rational amount and returns the nearest integer using half-up rounding.
func roundRatToInt64(value *big.Rat) int64 {
	numerator := new(big.Int).Set(value.Num())
	denominator := new(big.Int).Set(value.Denom())
	quotient, remainder := new(big.Int).QuoRem(numerator, denominator, new(big.Int))
	if remainder.Sign() == 0 {
		return quotient.Int64()
	}

	doubleRemainder := new(big.Int).Mul(new(big.Int).Abs(remainder), big.NewInt(2))
	if doubleRemainder.Cmp(denominator) >= 0 {
		if value.Sign() >= 0 {
			quotient.Add(quotient, big.NewInt(1))
		} else {
			quotient.Sub(quotient, big.NewInt(1))
		}
	}

	return quotient.Int64()
}
