package ledger

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRatesFromECBCubesReturnsUSDRelativeRates verifies ECB EUR rates are converted to units per USD.
func TestRatesFromECBCubesReturnsUSDRelativeRates(t *testing.T) {
	rates, err := ratesFromECBCubes([]ecbRateCube{
		{Currency: "USD", Rate: "1.25"},
		{Currency: "CAD", Rate: "1.70"},
		{Currency: "CNY", Rate: "9.00"},
	}, time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	require.Len(t, rates, 4)
	require.Equal(t, "CAD", rates[0].Currency)
	require.Equal(t, "1.36000000", rates[0].UnitsPerUSD)
	require.Equal(t, "CNY", rates[1].Currency)
	require.Equal(t, "7.20000000", rates[1].UnitsPerUSD)
	require.Equal(t, "EUR", rates[2].Currency)
	require.Equal(t, "0.80000000", rates[2].UnitsPerUSD)
	require.Equal(t, "USD", rates[3].Currency)
	require.Equal(t, "1.00000000", rates[3].UnitsPerUSD)
}

// TestStartDailyExchangeRateUpdaterStopsOnContextCancel verifies scheduled refresh workers exit on cancel.
func TestStartDailyExchangeRateUpdaterStopsOnContextCancel(t *testing.T) {
	baseline := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(t.Context())
	fetcher := blockingRateFetcher{started: make(chan struct{})}
	service := NewService()

	service.StartDailyExchangeRateUpdater(ctx, fetcher)
	<-fetcher.started
	cancel()

	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= baseline+1
	}, time.Second, 10*time.Millisecond)
}

type blockingRateFetcher struct {
	started chan struct{}
}

func (f blockingRateFetcher) FetchExchangeRates(ctx context.Context) ([]ExchangeRate, error) {
	close(f.started)
	<-ctx.Done()
	return nil, ctx.Err()
}
