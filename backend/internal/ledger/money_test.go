package ledger

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestConvertAmountCentsConvertsDirectAndReverseRates verifies decimal FX conversion without float math.
func TestConvertAmountCentsConvertsDirectAndReverseRates(t *testing.T) {
	rates := map[string]*big.Rat{
		"USD": big.NewRat(1, 1),
		"CNY": big.NewRat(720, 100),
	}

	direct, err := convertAmountCents(12345, "CNY", "USD", "CNY/USD=0.14", rates)
	require.NoError(t, err)
	require.Equal(t, int64(1728), direct)

	reverse, err := convertAmountCents(1400, "USD", "CNY", "CNY/USD=0.14", rates)
	require.NoError(t, err)
	require.Equal(t, int64(10000), reverse)

	table, err := convertAmountCents(10000, "CNY", "USD", "", rates)
	require.NoError(t, err)
	require.Equal(t, int64(1389), table)
}

// TestConvertAmountCentsRejectsUnusableExchangeRate verifies rates must match the requested pair.
func TestConvertAmountCentsRejectsUnusableExchangeRate(t *testing.T) {
	rates := map[string]*big.Rat{
		"USD": big.NewRat(1, 1),
		"CNY": big.NewRat(720, 100),
	}

	_, err := convertAmountCents(1000, "CNY", "USD", "EUR/USD=1.09", rates)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidInput)

	_, err = convertAmountCents(1000, "CNY", "USD", "CNY/USD=0", rates)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidInput)

	_, err = convertAmountCents(1000, "JPY", "USD", "", rates)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidInput)
}
