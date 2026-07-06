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
const wacaiImportAccountGroupName = "Wacai Import"

// registerImportRoutes receives an API group and registers protected import endpoints.
func registerImportRoutes(api *gin.RouterGroup, importService *importsvc.Service, ledgerService *ledger.Service, authService *auth.Service, auditService *audit.Service) {
	api.POST("/imports/wacai/preview", RequireSession(), func(c *gin.Context) {
		log := gmw.GetLogger(c)

		actor, ok := auth.ActorFromContext(c.Request.Context())
		if !ok {
			log.Debug("actor context missing")
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
			return
		}

		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportUploadBytes)
		fileHeader, err := c.FormFile("file")
		if err != nil {
			log.Debug("import file missing", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "import file is required")
			return
		}
		file, err := fileHeader.Open()
		if err != nil {
			log.Debug("open import file failed", zap.Error(err))
			respondAPIMessage(c, http.StatusBadRequest, "import file is invalid")
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
			respondAPIMessage(c, http.StatusBadRequest, "import file is invalid")
			return
		}

		batch, err := importService.PreviewWacaiFile(c.Request.Context(), importsvc.PreviewRequest{
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
			respondAPIMessage(c, http.StatusUnauthorized, "authentication required")
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
			respondAPIMessage(c, http.StatusBadRequest, "invalid import source")
			return
		}
		if strings.TrimSpace(request.SourceHash) == "" || strings.TrimSpace(request.SourceHash) != batch.SourceHash {
			respondAPIMessage(c, http.StatusConflict, "import batch source hash mismatch")
			return
		}
		if batch.ErrorCount > 0 {
			respondAPIMessage(c, http.StatusBadRequest, "import batch has row errors")
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

		response, err := commitWacaiBatch(c.Request.Context(), ledgerService, authService, ledger.Actor{UserID: actor.UserID}, actor.Email, c.Param("bookID"), batch, request.MemberMappings)
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
	SourceHash     string            `json:"sourceHash"`
	MemberMappings map[string]string `json:"memberMappings"`
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
func commitWacaiBatch(ctx context.Context, ledgerService *ledger.Service, authService *auth.Service, actor ledger.Actor, actorEmail string, bookID string, batch importsvc.Batch, memberMappings map[string]string) (importCommitResponse, error) {
	creators, err := resolveWacaiMemberMappings(ctx, authService, ledgerService, actor, actorEmail, bookID, batch.Rows, memberMappings)
	if err != nil {
		return importCommitResponse{}, err
	}
	accounts, categories, err := ensureWacaiImportReferences(ctx, ledgerService, actor, bookID, batch.Rows)
	if err != nil {
		return importCommitResponse{}, err
	}

	response := importCommitResponse{BatchID: batch.ID, BookID: bookID, Status: "applied", Entries: []ledger.Entry{}}
	for _, row := range batch.Rows {
		entry, skipped, err := commitWacaiRow(ctx, ledgerService, actor, bookID, creators[row.RowNumber], row, accounts, categories)
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

// ensureWacaiImportReferences receives preview rows and creates missing account and category references.
func ensureWacaiImportReferences(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, rows []importsvc.PreviewRow) ([]ledger.Account, []ledger.Category, error) {
	accounts, err := listAllImportAccounts(ctx, ledgerService, actor)
	if err != nil {
		return nil, nil, errors.Wrap(err, "load accounts for import commit")
	}
	groups, err := listAllImportAccountGroups(ctx, ledgerService, actor)
	if err != nil {
		return nil, nil, errors.Wrap(err, "load account groups for import commit")
	}
	categories, err := listAllImportCategories(ctx, ledgerService, actor, bookID)
	if err != nil {
		return nil, nil, errors.Wrap(err, "load categories for import commit")
	}

	groupID := ""
	for _, row := range rows {
		if len(row.Errors) > 0 {
			continue
		}
		entryType, ok := importEntryType(row.Type)
		if !ok {
			continue
		}
		groupID, accounts, err = ensureWacaiImportAccount(ctx, ledgerService, actor, bookID, groups, groupID, accounts, row.Account, row.Currency)
		if err != nil {
			return nil, nil, err
		}
		if entryType == ledger.EntryTypeTransfer {
			groupID, accounts, err = ensureWacaiImportAccount(ctx, ledgerService, actor, bookID, groups, groupID, accounts, row.DestinationAccount, row.Currency)
			if err != nil {
				return nil, nil, err
			}
			continue
		}
		categories, err = ensureWacaiImportCategory(ctx, ledgerService, actor, bookID, categories, row.Category, importCategoryDirection(entryType))
		if err != nil {
			return nil, nil, err
		}
	}

	return accounts, categories, nil
}

// commitWacaiRow receives one preview row and creates a ledger entry when all references map cleanly.
func commitWacaiRow(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, creatorUserID string, row importsvc.PreviewRow, accounts []ledger.Account, categories []ledger.Category) (ledger.Entry, *importSkippedRow, error) {
	if len(row.Errors) > 0 {
		return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: strings.Join(row.Errors, "; ")}, nil
	}

	accountID := accountIDByNameCurrency(accounts, row.Account, row.Currency)
	if accountID == "" {
		return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: "account is not mapped"}, nil
	}
	entryType, ok := importEntryType(row.Type)
	if !ok {
		return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: "type is not supported"}, nil
	}
	destinationAccountID := ""
	if entryType == ledger.EntryTypeTransfer {
		destinationAccountID = accountIDByNameCurrency(accounts, row.DestinationAccount, row.Currency)
		if destinationAccountID == "" {
			return ledger.Entry{}, &importSkippedRow{RowNumber: row.RowNumber, Reason: "destination account is not mapped"}, nil
		}
	}
	categoryID := ""
	if strings.TrimSpace(row.Category) != "" && entryType != ledger.EntryTypeTransfer {
		categoryID = categoryIDByPathDirection(categories, row.Category, importCategoryDirection(entryType))
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
		Actor:                actor,
		BookID:               bookID,
		CreatorUserID:        creatorUserID,
		Type:                 entryType,
		AccountID:            accountID,
		DestinationAccountID: destinationAccountID,
		CategoryID:           categoryID,
		AmountCents:          amountCents,
		TransactionCurrency:  row.Currency,
		OccurredAt:           occurredAt,
		Note:                 row.Note,
		Merchant:             row.Merchant,
		Tags:                 row.Tags,
	})
	if err != nil {
		return ledger.Entry{}, nil, errors.Wrap(err, "create imported entry")
	}

	return entry, nil, nil
}

// listAllImportAccounts receives an actor and returns all personal accounts across bounded pages.
func listAllImportAccounts(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor) ([]ledger.Account, error) {
	var accounts []ledger.Account
	for page := 1; ; page++ {
		result, err := ledgerService.ListAccounts(ctx, ledger.ListAccountsRequest{Actor: actor, Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, result.Items...)
		if page*result.PageSize >= result.Total {
			break
		}
	}

	return accounts, nil
}

// listAllImportAccountGroups receives an actor and returns all personal account groups across bounded pages.
func listAllImportAccountGroups(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor) ([]ledger.AccountGroup, error) {
	var groups []ledger.AccountGroup
	for page := 1; ; page++ {
		result, err := ledgerService.ListAccountGroups(ctx, ledger.ListAccountGroupsRequest{Actor: actor, Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		groups = append(groups, result.Items...)
		if page*result.PageSize >= result.Total {
			break
		}
	}

	return groups, nil
}

// listAllImportCategories receives a book scope and returns all categories across bounded pages.
func listAllImportCategories(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string) ([]ledger.Category, error) {
	var categories []ledger.Category
	for page := 1; ; page++ {
		result, err := ledgerService.ListCategories(ctx, ledger.ListCategoriesRequest{Actor: actor, BookID: bookID, Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		categories = append(categories, result.Items...)
		if page*result.PageSize >= result.Total {
			break
		}
	}

	return categories, nil
}

// ensureWacaiImportAccountGroup receives existing groups and returns a group id for imported accounts.
func ensureWacaiImportAccountGroup(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, groups []ledger.AccountGroup) (string, error) {
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.Name), wacaiImportAccountGroupName) {
			return group.ID, nil
		}
	}

	group, err := ledgerService.CreateAccountGroup(ctx, ledger.CreateAccountGroupRequest{
		Actor:     actor,
		Name:      wacaiImportAccountGroupName,
		SortOrder: len(groups) + 1,
	})
	if err != nil {
		return "", errors.Wrap(err, "create wacai import account group")
	}

	return group.ID, nil
}

// ensureWacaiImportAccount receives source account data and creates a missing account when needed.
func ensureWacaiImportAccount(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, groups []ledger.AccountGroup, groupID string, accounts []ledger.Account, name string, currency string) (string, []ledger.Account, error) {
	name = strings.TrimSpace(name)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if name == "" {
		return groupID, accounts, nil
	}
	if accountIDByNameCurrency(accounts, name, currency) != "" {
		return groupID, accounts, nil
	}
	if groupID == "" {
		var err error
		groupID, err = ensureWacaiImportAccountGroup(ctx, ledgerService, actor, groups)
		if err != nil {
			return "", nil, err
		}
	}

	account, err := ledgerService.CreateAccount(ctx, ledger.CreateAccountRequest{
		Actor:         actor,
		GroupID:       groupID,
		Name:          name,
		Type:          inferWacaiAccountType(name),
		Currency:      currency,
		SharedBookIDs: []string{bookID},
	})
	if err != nil {
		return "", nil, errors.Wrapf(err, "create wacai import account %q", name)
	}

	return groupID, append(accounts, account), nil
}

// accountIDByNameCurrency receives accounts and source data and returns the exact matching account id.
func accountIDByNameCurrency(accounts []ledger.Account, name string, currency string) string {
	name = strings.TrimSpace(name)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	for _, account := range accounts {
		if account.Name == name && account.Currency == currency {
			return account.ID
		}
	}

	return ""
}

// ensureWacaiImportCategory receives a source category path and creates missing category tree nodes.
func ensureWacaiImportCategory(ctx context.Context, ledgerService *ledger.Service, actor ledger.Actor, bookID string, categories []ledger.Category, sourcePath string, direction ledger.CategoryDirection) ([]ledger.Category, error) {
	parts := splitWacaiCategoryPath(sourcePath)
	if len(parts) == 0 {
		return categories, nil
	}

	parentID := ""
	for index, part := range parts {
		categoryID := categoryIDByParentNameDirection(categories, parentID, part, direction)
		if categoryID != "" {
			parentID = categoryID
			continue
		}

		rawSourceName := part
		if index == len(parts)-1 {
			rawSourceName = strings.TrimSpace(sourcePath)
		}
		category, err := ledgerService.CreateCategory(ctx, ledger.CreateCategoryRequest{
			Actor:         actor,
			BookID:        bookID,
			ParentID:      parentID,
			Name:          part,
			Direction:     direction,
			SortOrder:     len(categories) + 1,
			RawSourceName: rawSourceName,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "create wacai import category %q", sourcePath)
		}
		categories = append(categories, category)
		parentID = category.ID
	}

	return categories, nil
}

// categoryIDByPathDirection receives categories and a source path and returns the matching active leaf id.
func categoryIDByPathDirection(categories []ledger.Category, sourcePath string, direction ledger.CategoryDirection) string {
	parentID := ""
	for _, part := range splitWacaiCategoryPath(sourcePath) {
		categoryID := categoryIDByParentNameDirection(categories, parentID, part, direction)
		if categoryID == "" {
			return ""
		}
		parentID = categoryID
	}

	return parentID
}

// categoryIDByParentNameDirection receives category identity fields and returns the matching active id.
func categoryIDByParentNameDirection(categories []ledger.Category, parentID string, name string, direction ledger.CategoryDirection) string {
	parentID = strings.TrimSpace(parentID)
	name = strings.TrimSpace(name)
	for _, category := range categories {
		if category.ParentID == parentID && category.Name == name && category.Direction == direction && !category.Archived {
			return category.ID
		}
	}

	return ""
}

// splitWacaiCategoryPath receives a Wacai category path and returns normalized hierarchy parts.
func splitWacaiCategoryPath(sourcePath string) []string {
	rawParts := strings.FieldsFunc(sourcePath, func(r rune) bool {
		return r == '/' || r == '\\' || r == '／'
	})
	parts := make([]string, 0, len(rawParts))
	for _, part := range rawParts {
		part = strings.TrimSpace(part)
		if part != "" {
			parts = append(parts, part)
		}
	}

	return parts
}

// importCategoryDirection receives an entry type and returns the category direction used for imported paths.
func importCategoryDirection(entryType ledger.EntryType) ledger.CategoryDirection {
	switch entryType {
	case ledger.EntryTypeIncome, ledger.EntryTypeBorrow, ledger.EntryTypeRefund, ledger.EntryTypeReimbursement, ledger.EntryTypeRepayment:
		return ledger.CategoryDirectionIncome
	default:
		return ledger.CategoryDirectionExpense
	}
}

// inferWacaiAccountType receives a source account name and returns a broad account type.
func inferWacaiAccountType(name string) ledger.AccountType {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch {
	case strings.Contains(normalized, "信用卡") || strings.Contains(normalized, "credit"):
		return ledger.AccountTypeCreditCard
	case strings.Contains(normalized, "支付宝") || strings.Contains(normalized, "微信") || strings.Contains(normalized, "paypal"):
		return ledger.AccountTypePaymentPlatform
	case strings.Contains(normalized, "loan") || strings.Contains(normalized, "贷款"):
		return ledger.AccountTypeLoan
	case strings.Contains(normalized, "股票") || strings.Contains(normalized, "基金") || strings.Contains(normalized, "invest"):
		return ledger.AccountTypeInvestment
	case strings.Contains(normalized, "现金") || strings.Contains(normalized, "cash"):
		return ledger.AccountTypeCash
	default:
		return ledger.AccountTypeSavings
	}
}

// importEntryType receives source text and returns the supported ledger entry type.
func importEntryType(raw string) (ledger.EntryType, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "expense":
		return ledger.EntryTypeExpense, true
	case "income":
		return ledger.EntryTypeIncome, true
	case "transfer":
		return ledger.EntryTypeTransfer, true
	case "refund":
		return ledger.EntryTypeRefund, true
	case "reimbursement":
		return ledger.EntryTypeReimbursement, true
	case "borrow":
		return ledger.EntryTypeBorrow, true
	case "lend":
		return ledger.EntryTypeLend, true
	case "repayment":
		return ledger.EntryTypeRepayment, true
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
		respondAPIMessage(c, http.StatusBadRequest, "invalid import input")
	case errors.Is(err, importsvc.ErrNotFound):
		log.Debug("import resource not found", zap.Error(err))
		respondAPIMessage(c, http.StatusNotFound, "import resource not found")
	default:
		log.Debug("import request failed", zap.Error(err))
		respondAPIMessage(c, http.StatusInternalServerError, "import request failed")
	}
}
