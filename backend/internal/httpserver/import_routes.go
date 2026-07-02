package httpserver

import (
	"context"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	gmw "github.com/Laisky/gin-middlewares/v7"
	glog "github.com/Laisky/go-utils/v6/log"
	"github.com/Laisky/zap"
	"github.com/gin-gonic/gin"

	"github.com/Laisky/Accounting/backend/internal/audit"
	"github.com/Laisky/Accounting/backend/internal/auth"
	importsvc "github.com/Laisky/Accounting/backend/internal/imports"
	"github.com/Laisky/Accounting/backend/internal/ledger"
)

const maxImportUploadBytes = 6 * 1024 * 1024

// registerImportRoutes receives an API group and registers protected import endpoints.
func registerImportRoutes(api *gin.RouterGroup, importService *importsvc.Service, ledgerService *ledger.Service, auditService *audit.Service) {
	api.POST("/imports/wacai/preview", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportUploadBytes)
		fileHeader, err := c.FormFile("file")
		if err != nil {
			log.Debug("import file missing", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "import file is required"})
			return
		}
		file, err := fileHeader.Open()
		if err != nil {
			log.Debug("open import file failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "import file is invalid"})
			return
		}
		defer func() {
			if closeErr := file.Close(); closeErr != nil {
				log.Debug("close import file failed", zap.Error(closeErr))
			}
		}()

		data, err := io.ReadAll(file)
		if err != nil {
			log.Debug("read import file failed", zap.Error(err))
			c.JSON(http.StatusBadRequest, gin.H{"error": "import file is invalid"})
			return
		}

		batch, err := importService.PreviewWacaiCSV(c.Request.Context(), importsvc.PreviewRequest{
			Actor:       importsvc.Actor{UserID: actor.UserID},
			Filename:    fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Data:        data,
		})
		if err != nil {
			respondImportError(c, log, err)
			return
		}

		log.Debug("wacai import preview created", zap.String("batch_id", batch.ID), zap.String("user_id", actor.UserID))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionImportPreviewCreated,
			TargetType: "import_batch",
			TargetID:   batch.ID,
			Metadata: map[string]string{
				"source":   batch.Source,
				"filename": batch.Filename,
			},
		})
		c.JSON(http.StatusCreated, batch)
	})

	api.POST("/books/:bookID/imports/:batchID/apply", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		var request importCommitRequest
		if !decodeStrictJSON(c, &request) {
			return
		}

		batch, err := importService.Batch(c.Request.Context(), importsvc.BatchRequest{
			Actor:   importsvc.Actor{UserID: actor.UserID},
			BatchID: c.Param("batchID"),
		})
		if err != nil {
			respondImportError(c, log, err)
			return
		}
		if batch.Source != "wacai" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid import source"})
			return
		}
		if strings.TrimSpace(request.SourceHash) == "" || strings.TrimSpace(request.SourceHash) != batch.SourceHash {
			c.JSON(http.StatusConflict, gin.H{"error": "import batch source hash mismatch"})
			return
		}
		if batch.ErrorCount > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "import batch has row errors"})
			return
		}
		if batch.Status == importsvc.BatchStatusApplied {
			response, err := appliedWacaiBatch(c.Request.Context(), ledgerService, ledger.Actor{UserID: actor.UserID}, c.Param("bookID"), batch)
			if err != nil {
				respondLedgerError(c, log, err)
				return
			}

			log.Debug("wacai import replayed",
				zap.String("batch_id", batch.ID),
				zap.String("book_id", response.BookID),
				zap.Int("imported_count", response.ImportedCount))
			c.JSON(http.StatusOK, response)
			return
		}

		response, err := commitWacaiBatch(c.Request.Context(), ledgerService, ledger.Actor{UserID: actor.UserID}, c.Param("bookID"), batch)
		if err != nil {
			respondLedgerError(c, log, err)
			return
		}
		_, err = importService.MarkApplied(c.Request.Context(), importsvc.MarkAppliedRequest{
			Actor:       importsvc.Actor{UserID: actor.UserID},
			BatchID:     batch.ID,
			BookID:      response.BookID,
			EntryIDs:    entryIDsForImportResponse(response.Entries),
			SkippedRows: importSkippedRowsForService(response.SkippedRows),
		})
		if err != nil {
			respondImportError(c, log, err)
			return
		}

		log.Debug("wacai import committed",
			zap.String("batch_id", batch.ID),
			zap.String("book_id", response.BookID),
			zap.Int("imported_count", response.ImportedCount))
		recordAuditEvent(c, auditService, audit.RecordRequest{
			ActorID:    actor.UserID,
			ActorEmail: actor.Email,
			Action:     audit.ActionImportCommitted,
			TargetType: "import_batch",
			TargetID:   batch.ID,
			Metadata: map[string]string{
				"source":         batch.Source,
				"source_hash":    batch.SourceHash,
				"book_id":        response.BookID,
				"imported_count": strconv.Itoa(response.ImportedCount),
				"skipped_count":  strconv.Itoa(len(response.SkippedRows)),
			},
		})
		for _, entry := range response.Entries {
			recordAuditEvent(c, auditService, audit.RecordRequest{
				ActorID:    actor.UserID,
				ActorEmail: actor.Email,
				Action:     audit.ActionEntryCreated,
				TargetType: "entry",
				TargetID:   entry.ID,
				Metadata: map[string]string{
					"book_id": entry.BookID,
					"type":    string(entry.Type),
					"source":  batch.Source,
				},
			})
		}
		c.JSON(http.StatusCreated, response)
	})
}

type importCommitRequest struct {
	SourceHash string `json:"sourceHash"`
}

type importCommitResponse struct {
	BatchID       string             `json:"batchId"`
	BookID        string             `json:"bookId"`
	Status        string             `json:"status"`
	ImportedCount int                `json:"importedCount"`
	SkippedCount  int                `json:"skippedCount"`
	Entries       []ledger.Entry     `json:"entries"`
	SkippedRows   []importSkippedRow `json:"skippedRows,omitempty"`
}

type importSkippedRow struct {
	RowNumber int    `json:"rowNumber"`
	Reason    string `json:"reason"`
}

// commitWacaiBatch receives a parsed Wacai batch and creates ledger entries for mappable rows.
func commitWacaiBatch(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, batch importsvc.Batch) (importCommitResponse, error) {
	accounts, err := ledgerService.ListAccounts(ctx, ledger.ListAccountsRequest{Actor: actor, Page: 1, PageSize: 100})
	if err != nil {
		return importCommitResponse{}, errors.Wrap(err, "load accounts for import commit")
	}
	categories, err := ledgerService.ListCategories(ctx, ledger.ListCategoriesRequest{Actor: actor, BookID: bookID, Page: 1, PageSize: 100})
	if err != nil {
		return importCommitResponse{}, errors.Wrap(err, "load categories for import commit")
	}

	response := importCommitResponse{BatchID: batch.ID, BookID: bookID, Status: "applied", Entries: []ledger.Entry{}}
	for _, row := range batch.Rows {
		entry, skipped, err := commitWacaiRow(ctx, ledgerService, actor, bookID, row, accounts.Items, categories.Items)
		if err != nil {
			return importCommitResponse{}, err
		}
		if skipped != nil {
			response.SkippedRows = append(response.SkippedRows, *skipped)
			continue
		}
		response.Entries = append(response.Entries, entry)
	}

	response.ImportedCount = len(response.Entries)
	response.SkippedCount = len(response.SkippedRows)
	if response.ImportedCount == 0 {
		return importCommitResponse{}, errors.Wrap(ledger.ErrInvalidInput, "no import rows could be committed")
	}

	return response, nil
}

// appliedWacaiBatch receives an already-applied batch and returns its stored commit summary.
func appliedWacaiBatch(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, batch importsvc.Batch) (importCommitResponse, error) {
	if batch.AppliedBookID != bookID {
		return importCommitResponse{}, errors.Wrap(ledger.ErrInvalidInput, "import batch already applied to another book")
	}

	entries, err := importEntriesByIDs(ctx, ledgerService, actor, bookID, batch.AppliedEntryIDs)
	if err != nil {
		return importCommitResponse{}, err
	}

	return importCommitResponse{
		BatchID:       batch.ID,
		BookID:        bookID,
		Status:        string(importsvc.BatchStatusApplied),
		ImportedCount: len(batch.AppliedEntryIDs),
		SkippedCount:  len(batch.AppliedSkippedRows),
		Entries:       entries,
		SkippedRows:   importSkippedRowsFromService(batch.AppliedSkippedRows),
	}, nil
}

// importEntriesByIDs receives stored imported entry ids and returns visible entries in stored order.
func importEntriesByIDs(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, entryIDs []string) ([]ledger.Entry, error) {
	if len(entryIDs) == 0 {
		return []ledger.Entry{}, nil
	}

	wanted := make(map[string]struct{}, len(entryIDs))
	for _, entryID := range entryIDs {
		wanted[entryID] = struct{}{}
	}

	found := make(map[string]ledger.Entry, len(entryIDs))
	for page := 1; ; page++ {
		list, err := ledgerService.ListEntries(ctx, ledger.ListEntriesRequest{
			Actor:    actor,
			BookID:   bookID,
			Page:     page,
			PageSize: 100,
		})
		if err != nil {
			return nil, errors.Wrap(err, "load applied import entries")
		}
		for _, entry := range list.Entries {
			if _, ok := wanted[entry.ID]; ok {
				found[entry.ID] = entry
			}
		}
		if len(found) == len(wanted) || page*list.PageSize >= list.Total {
			break
		}
	}

	entries := make([]ledger.Entry, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		if entry, ok := found[entryID]; ok {
			entries = append(entries, entry)
		}
	}

	return entries, nil
}

// entryIDsForImportResponse receives committed entries and returns their ids in response order.
func entryIDsForImportResponse(entries []ledger.Entry) []string {
	entryIDs := make([]string, 0, len(entries))
	for _, entry := range entries {
		entryIDs = append(entryIDs, entry.ID)
	}

	return entryIDs
}

// importSkippedRowsForService receives API skipped rows and returns import service metadata.
func importSkippedRowsForService(rows []importSkippedRow) []importsvc.AppliedSkippedRow {
	converted := make([]importsvc.AppliedSkippedRow, 0, len(rows))
	for _, row := range rows {
		converted = append(converted, importsvc.AppliedSkippedRow{RowNumber: row.RowNumber, Reason: row.Reason})
	}

	return converted
}

// importSkippedRowsFromService receives import service skipped rows and returns API metadata.
func importSkippedRowsFromService(rows []importsvc.AppliedSkippedRow) []importSkippedRow {
	converted := make([]importSkippedRow, 0, len(rows))
	for _, row := range rows {
		converted = append(converted, importSkippedRow{RowNumber: row.RowNumber, Reason: row.Reason})
	}

	return converted
}

// commitWacaiRow receives one preview row and creates a ledger entry when all references map cleanly.
func commitWacaiRow(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, row importsvc.PreviewRow, accounts []ledger.Account, categories []ledger.Category) (ledger.Entry, *importSkippedRow, error) {
	if len(row.Errors) > 0 {
		return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: strings.Join(row.Errors, "; ")}, nil
	}

	accountID := accountIDByName(accounts, row.Account)
	if accountID == "" {
		return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: "account is not mapped"}, nil
	}
	entryType, ok := importEntryType(row.Type)
	if !ok {
		return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: "type is not supported"}, nil
	}
	categoryID := ""
	if strings.TrimSpace(row.Category) != "" {
		categoryID = categoryIDByName(categories, row.Category)
		if categoryID == "" {
			return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: "category is not mapped"}, nil
		}
	}
	amountCents, err := importAmountCents(row.Amount)
	if err != nil {
		return ledger.Entry{}, nil, err
	}
	occurredAt, err := importOccurredAt(row.OccurredAt)
	if err != nil {
		return ledger.Entry{}, nil, err
	}

	entry, err := ledgerService.CreateEntry(ctx, ledger.CreateEntryRequest{
		Actor:               actor,
		BookID:              bookID,
		Type:                entryType,
		AccountID:           accountID,
		CategoryID:          categoryID,
		AmountCents:         amountCents,
		TransactionCurrency: row.Currency,
		OccurredAt:          occurredAt,
		Note:                row.Note,
		Merchant:            row.Merchant,
		Tags:                row.Tags,
	})
	if err != nil {
		return ledger.Entry{}, nil, errors.Wrap(err, "create imported entry")
	}

	return entry, nil, nil
}

// accountIDByName receives accounts and a source name and returns the exact matching account id.
func accountIDByName(accounts []ledger.Account, name string) string {
	name = strings.TrimSpace(name)
	for _, account := range accounts {
		if account.Name == name {
			return account.ID
		}
	}

	return ""
}

// categoryIDByName receives categories and a source name and returns the exact active category id.
func categoryIDByName(categories []ledger.Category, name string) string {
	name = strings.TrimSpace(name)
	for _, category := range categories {
		if category.Name == name && !category.Archived {
			return category.ID
		}
	}

	return ""
}

// importEntryType receives source text and returns the supported ledger entry type.
func importEntryType(raw string) (ledger.EntryType, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "expense":
		return ledger.EntryTypeExpense, true
	case "income":
		return ledger.EntryTypeIncome, true
	default:
		return "", false
	}
}

// importAmountCents receives a decimal source amount and returns cents.
func importAmountCents(raw string) (int64, error) {
	value, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(raw), ",", ""), 64)
	if err != nil || value <= 0 {
		return 0, errors.Wrap(ledger.ErrInvalidInput, "import amount is invalid")
	}

	return int64(math.Round(value * 100)), nil
}

// importOccurredAt receives a source date string and returns a UTC timestamp.
func importOccurredAt(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", time.DateOnly, "2006/01/02 15:04:05", "2006/01/02"} {
		if occurredAt, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return occurredAt.UTC(), nil
		}
	}

	return time.Time{}, errors.Wrap(ledger.ErrInvalidInput, "import occurredAt is invalid")
}

// respondImportError receives an import service error and writes a stable API response.
func respondImportError(c *gin.Context, log glog.Logger, err error) {
	switch {
	case errors.Is(err, importsvc.ErrInvalidInput):
		log.Debug("import input rejected", zap.Error(err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid import input"})
	case errors.Is(err, importsvc.ErrNotFound):
		log.Debug("import resource not found", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "import resource not found"})
	default:
		log.Debug("import request failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "import request failed"})
	}
}
