package ledger

import (
	"context"
	"encoding/xml"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/Laisky/zap"

	"github.com/Laisky/Accounting/backend/internal/logger"
)

const (
	defaultECBRateURL    = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"
	exchangeRateSource   = "ecb-eurofxref"
	exchangeRateInterval = 24 * time.Hour
)

// ExchangeRateFetcher fetches current supported currency rates for storage.
type ExchangeRateFetcher interface {
	FetchExchangeRates(ctx context.Context) ([]ExchangeRate, error)
}

// ECBExchangeRateFetcher fetches ECB euro reference rates and converts them to USD-relative rates.
type ECBExchangeRateFetcher struct {
	Client *http.Client
	URL    string
}

type ecbEnvelope struct {
	Cubes []ecbRateCube `xml:"Cube>Cube>Cube"`
}

type ecbRateCube struct {
	Currency string `xml:"currency,attr"`
	Rate     string `xml:"rate,attr"`
}

// NewECBExchangeRateFetcher receives no parameters and returns a default ECB-backed fetcher.
func NewECBExchangeRateFetcher() ECBExchangeRateFetcher {
	return ECBExchangeRateFetcher{
		Client: &http.Client{Timeout: 10 * time.Second},
		URL:    defaultECBRateURL,
	}
}

// FetchExchangeRates receives a context and returns supported rates as currency units per USD.
func (f ECBExchangeRateFetcher) FetchExchangeRates(ctx context.Context) ([]ExchangeRate, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	url := strings.TrimSpace(f.URL)
	if url == "" {
		url = defaultECBRateURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrap(err, "create ecb exchange rate request")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "fetch ecb exchange rates")
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, errors.Wrapf(ErrInvalidInput, "ecb exchange rate status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, errors.Wrap(err, "read ecb exchange rates")
	}

	var envelope ecbEnvelope
	if err := xml.Unmarshal(body, &envelope); err != nil {
		return nil, errors.Wrap(err, "parse ecb exchange rates")
	}

	return ratesFromECBCubes(envelope.Cubes, time.Now().UTC())
}

// RefreshExchangeRates receives a fetcher, refreshes the stored rate table, and returns no value.
func (s *Service) RefreshExchangeRates(ctx context.Context, fetcher ExchangeRateFetcher) error {
	if fetcher == nil {
		return errors.WithStack(errors.New("exchange rate fetcher is nil"))
	}

	rates, err := fetcher.FetchExchangeRates(ctx)
	if err != nil {
		return err
	}
	if err := s.store.ReplaceExchangeRates(ctx, normalizeExchangeRates(rates)); err != nil {
		return errors.Wrap(err, "store exchange rates")
	}

	return nil
}

// StartDailyExchangeRateUpdater receives a context and fetcher and refreshes rates immediately and then daily.
func (s *Service) StartDailyExchangeRateUpdater(ctx context.Context, fetcher ExchangeRateFetcher) {
	go func() {
		s.refreshExchangeRatesForSchedule(ctx, fetcher)
		ticker := time.NewTicker(exchangeRateInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.refreshExchangeRatesForSchedule(ctx, fetcher)
			}
		}
	}()
}

// refreshExchangeRatesForSchedule receives a context and fetcher and logs scheduled refresh failures.
func (s *Service) refreshExchangeRatesForSchedule(ctx context.Context, fetcher ExchangeRateFetcher) {
	if err := s.RefreshExchangeRates(ctx, fetcher); err != nil {
		log := logger.FromContext(ctx)
		if log != nil {
			log.Debug("exchange rate refresh failed", zap.Error(err))
		}
	}
}

// exchangeRateIndex receives no parameters and returns supported rates indexed by currency.
func (s *Service) exchangeRateIndex(ctx context.Context) (map[string]*big.Rat, error) {
	rates, err := s.store.ExchangeRates(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "load exchange rates")
	}
	if len(rates) == 0 {
		rates = defaultExchangeRates(time.Now().UTC())
	}

	index := make(map[string]*big.Rat, len(rates))
	for _, rate := range normalizeExchangeRates(rates) {
		parsed, ok := new(big.Rat).SetString(rate.UnitsPerUSD)
		if !ok || parsed.Sign() <= 0 {
			return nil, errors.Wrapf(ErrInvalidInput, "exchange rate for %s is invalid", rate.Currency)
		}
		index[rate.Currency] = parsed
	}

	for currency := range supportedCurrencies {
		if _, ok := index[currency]; !ok {
			return nil, errors.Wrapf(ErrInvalidInput, "exchange rate for %s is missing", currency)
		}
	}

	return index, nil
}

// ratesFromECBCubes receives ECB XML cubes and returns supported USD-relative exchange rates.
func ratesFromECBCubes(cubes []ecbRateCube, updatedAt time.Time) ([]ExchangeRate, error) {
	euroRates := map[string]*big.Rat{"EUR": big.NewRat(1, 1)}
	for _, cube := range cubes {
		currency := strings.ToUpper(strings.TrimSpace(cube.Currency))
		if _, ok := supportedCurrencies[currency]; !ok {
			continue
		}
		rate, ok := new(big.Rat).SetString(strings.TrimSpace(cube.Rate))
		if !ok || rate.Sign() <= 0 {
			return nil, errors.Wrapf(ErrInvalidInput, "ecb rate for %s is invalid", currency)
		}
		euroRates[currency] = rate
	}

	usdRate, ok := euroRates["USD"]
	if !ok || usdRate.Sign() <= 0 {
		return nil, errors.Wrap(ErrInvalidInput, "ecb USD rate is missing")
	}

	rates := make([]ExchangeRate, 0, len(supportedCurrencies))
	for currency := range supportedCurrencies {
		unitsPerUSD := big.NewRat(1, 1)
		if currency != "USD" {
			euroRate, ok := euroRates[currency]
			if !ok {
				return nil, errors.Wrapf(ErrInvalidInput, "ecb rate for %s is missing", currency)
			}
			unitsPerUSD.Quo(euroRate, usdRate)
		}
		rates = append(rates, ExchangeRate{
			Currency:    currency,
			UnitsPerUSD: unitsPerUSD.FloatString(8),
			Source:      exchangeRateSource,
			UpdatedAt:   updatedAt.UTC(),
		})
	}

	return normalizeExchangeRates(rates), nil
}

// defaultExchangeRates receives an update time and returns deterministic bootstrap rates for supported currencies.
func defaultExchangeRates(updatedAt time.Time) []ExchangeRate {
	return normalizeExchangeRates([]ExchangeRate{
		{Currency: "USD", UnitsPerUSD: "1", Source: "bootstrap", UpdatedAt: updatedAt.UTC()},
		{Currency: "EUR", UnitsPerUSD: "0.85", Source: "bootstrap", UpdatedAt: updatedAt.UTC()},
		{Currency: "CNY", UnitsPerUSD: "7.20", Source: "bootstrap", UpdatedAt: updatedAt.UTC()},
		{Currency: "CAD", UnitsPerUSD: "1.36", Source: "bootstrap", UpdatedAt: updatedAt.UTC()},
	})
}

// normalizeExchangeRates receives rates and returns supported, sorted, normalized rates.
func normalizeExchangeRates(rates []ExchangeRate) []ExchangeRate {
	normalized := make([]ExchangeRate, 0, len(rates))
	seen := map[string]struct{}{}
	for _, rate := range rates {
		currency := strings.ToUpper(strings.TrimSpace(rate.Currency))
		if !isSupportedCurrency(currency) {
			continue
		}
		if _, ok := seen[currency]; ok {
			continue
		}
		seen[currency] = struct{}{}
		rate.Currency = currency
		rate.UnitsPerUSD = strings.TrimSpace(rate.UnitsPerUSD)
		rate.Source = strings.TrimSpace(rate.Source)
		rate.UpdatedAt = rate.UpdatedAt.UTC()
		normalized = append(normalized, rate)
	}

	return cloneExchangeRates(normalized)
}
