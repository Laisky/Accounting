package imports

import (
	"context"
	"os"
	"path/filepath"
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

// TestPreviewWacaiFileParsesRealXLSX verifies the parser handles the observed Wacai workbook export shape.
func TestPreviewWacaiFileParsesRealXLSX(t *testing.T) {
	service := NewService(NewMemoryStore())
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "ref", "wacai", "import", "wacai_日常账本_202607022149857_924.xlsx"))
	require.NoError(t, err)

	batch, err := service.PreviewWacaiFile(context.Background(), PreviewRequest{
		Actor:       Actor{UserID: "user-owner"},
		Filename:    "wacai.xlsx",
		ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Data:        data,
	})

	require.NoError(t, err)
	require.Equal(t, "wacai-preview-v2", batch.ParserVersion)
	require.NotEmpty(t, batch.Detected.Books)
	require.NotEmpty(t, batch.Detected.Accounts)
	require.Contains(t, batch.Detected.Currencies, "CNY")
	require.Len(t, batch.Rows, maxPreviewRows)
	require.Equal(t, 8, batch.Rows[0].RowNumber)
	require.NotEmpty(t, batch.Rows[0].SourceType)
	require.NotEmpty(t, batch.Rows[0].Account)
	require.NotEmpty(t, batch.Rows[0].Raw["日期时间"])
	require.Equal(t, "transfer", batch.Rows[0].Type)
	require.NotEmpty(t, batch.Rows[0].DestinationAccount)
	require.Empty(t, batch.Rows[0].Errors)
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

// TestBatchLoadsOnlyActorOwnedBatches verifies batch lookup enforces import ownership.
func TestBatchLoadsOnlyActorOwnedBatches(t *testing.T) {
	service := NewService(NewMemoryStore())
	created, err := service.PreviewWacaiCSV(context.Background(), PreviewRequest{
		Actor:    Actor{UserID: "user-owner"},
		Filename: "wacai.csv",
		Data:     []byte("date,type,amount\n2026-07-01,expense,12.30\n"),
	})
	require.NoError(t, err)

	loaded, err := service.Batch(context.Background(), BatchRequest{
		Actor:   Actor{UserID: "user-owner"},
		BatchID: created.ID,
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, loaded.ID)

	_, err = service.Batch(context.Background(), BatchRequest{
		Actor:   Actor{UserID: "user-other"},
		BatchID: created.ID,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNotFound))
}

// TestFinalizeAppliedStoresCommitMetadata verifies applied import metadata is durable and
// state-checked: it can only follow a claim, and re-finalizing an applied batch conflicts.
func TestFinalizeAppliedStoresCommitMetadata(t *testing.T) {
	service := NewService(NewMemoryStore())
	created, err := service.PreviewWacaiCSV(context.Background(), PreviewRequest{
		Actor:    Actor{UserID: "user-owner"},
		Filename: "wacai.csv",
		Data:     []byte("date,type,amount\n2026-07-01,expense,12.30\n"),
	})
	require.NoError(t, err)

	// Finalizing before claiming conflicts (the batch is still preview, not applying).
	_, err = service.FinalizeApplied(context.Background(), MarkAppliedRequest{
		Actor: Actor{UserID: "user-owner"}, BatchID: created.ID, BookID: "book-household", EntryIDs: []string{"entry-1"},
	})
	require.ErrorIs(t, err, ErrConflict)

	_, err = service.ClaimForApply(context.Background(), BatchRequest{Actor: Actor{UserID: "user-owner"}, BatchID: created.ID})
	require.NoError(t, err)

	applied, err := service.FinalizeApplied(context.Background(), MarkAppliedRequest{
		Actor:    Actor{UserID: "user-owner"},
		BatchID:  created.ID,
		BookID:   "book-household",
		EntryIDs: []string{" entry-1 ", "entry-1", "entry-2"},
		SkippedRows: []AppliedSkippedRow{
			{RowNumber: 3, Reason: "account is not mapped"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, BatchStatusApplied, applied.Status)
	require.Equal(t, "book-household", applied.AppliedBookID)
	require.Equal(t, []string{"entry-1", "entry-2"}, applied.AppliedEntryIDs)
	require.Len(t, applied.AppliedSkippedRows, 1)
	require.NotNil(t, applied.AppliedAt)

	// Re-finalizing an already-applied batch conflicts (idempotent replay is the handler's job).
	_, err = service.FinalizeApplied(context.Background(), MarkAppliedRequest{
		Actor: Actor{UserID: "user-owner"}, BatchID: created.ID, BookID: "book-household", EntryIDs: []string{"entry-3"},
	})
	require.ErrorIs(t, err, ErrConflict)
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
