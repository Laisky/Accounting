package imports

import (
	"context"
	"testing"

	"github.com/Laisky/errors/v2"
	"github.com/stretchr/testify/require"
)

// TestPreviewWacaiCSVParsesRowsAndDetectedValues verifies CSV preview parsing and diagnostics.
func TestPreviewWacaiCSVParsesRowsAndDetectedValues(t *testing.T) {
	service := NewService(NewMemoryStore())
	data := []byte("date,type,amount,currency,account,category,book,member,merchant,note,tags\n2026-07-01,expense,12.30,cny,Cash,Dining,Household,Alice,Market,Lunch,food|work\nnot-a-date,income,abc,,Bank,Salary,Household,Bob,,Bonus,\n")

	batch, err := service.PreviewWacaiCSV(context.Background(), PreviewRequest{
		Actor:       Actor{UserID: "user-owner"},
		Filename:    "wacai.csv",
		ContentType: "text/csv",
		Data:        data,
	})

	require.NoError(t, err)
	require.NotEmpty(t, batch.ID)
	require.Equal(t, "user-owner", batch.UserID)
	require.Equal(t, "wacai", batch.Source)
	require.Equal(t, BatchStatusPreview, batch.Status)
	require.Equal(t, "date", batch.DetectedSchema.Columns["occurredAt"])
	require.Len(t, batch.Rows, 2)
	require.Equal(t, "2026-07-01", batch.Rows[0].OccurredAt)
	require.Equal(t, "12.30", batch.Rows[0].Amount)
	require.Equal(t, "CNY", batch.Rows[0].Currency)
	require.Equal(t, []string{"food", "work"}, batch.Rows[0].Tags)
	require.Contains(t, batch.Rows[1].Warnings, "currency missing; defaulted to CNY")
	require.Contains(t, batch.Rows[1].Warnings, "occurredAt format needs review")
	require.Contains(t, batch.Rows[1].Errors, "amount is invalid")
	require.Equal(t, 2, batch.WarningCount)
	require.Equal(t, 1, batch.ErrorCount)
	require.Equal(t, []string{"Household"}, batch.Detected.Books)
	require.Equal(t, []string{"Cash", "Bank"}, batch.Detected.Accounts)
	require.Equal(t, []string{"Dining", "Salary"}, batch.Detected.Categories)
	require.Equal(t, []string{"CNY"}, batch.Detected.Currencies)
	require.True(t, batch.CreatedAt.Equal(batch.CreatedAt.UTC()))
}

// TestPreviewWacaiCSVIsIdempotentByHash verifies repeated uploads return the stored batch.
func TestPreviewWacaiCSVIsIdempotentByHash(t *testing.T) {
	service := NewService(NewMemoryStore())
	request := PreviewRequest{
		Actor:    Actor{UserID: "user-owner"},
		Filename: "wacai.csv",
		Data:     []byte("date,type,amount\n2026-07-01,expense,12.30\n"),
	}

	first, err := service.PreviewWacaiCSV(context.Background(), request)
	require.NoError(t, err)
	second, err := service.PreviewWacaiCSV(context.Background(), request)
	require.NoError(t, err)

	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.SourceHash, second.SourceHash)
}

// TestPreviewWacaiCSVRejectsInvalidInput verifies preview requests fail closed.
func TestPreviewWacaiCSVRejectsInvalidInput(t *testing.T) {
	service := NewService(NewMemoryStore())

	_, err := service.PreviewWacaiCSV(context.Background(), PreviewRequest{
		Actor:    Actor{UserID: "user-owner"},
		Filename: "wacai.xlsx",
		Data:     []byte("not csv"),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.PreviewWacaiCSV(context.Background(), PreviewRequest{
		Actor:    Actor{UserID: "user-owner"},
		Filename: "wacai.csv",
		Data:     []byte("date,type,amount\n"),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))

	_, err = service.PreviewWacaiCSV(context.Background(), PreviewRequest{
		Actor:    Actor{UserID: "user-owner"},
		Filename: "wacai.csv",
		Data:     []byte("\"unterminated"),
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvalidInput))
}
