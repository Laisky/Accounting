package imports

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Laisky/errors/v2"
	"github.com/google/uuid"
)

const (
	sourceWacai     = "wacai"
	parserVersion   = "wacai-csv-preview-v1"
	maxPreviewBytes = 5 * 1024 * 1024
	maxPreviewRows  = 500
	defaultCurrency = "CNY"
)

// Service owns import preview use cases and coordinates parser and store access.
type Service struct {
	store Store
}

// NewService receives an import store and returns a Service.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// PreviewWacaiCSV receives an uploaded CSV file, parses preview rows, and stores an idempotent batch.
func (s *Service) PreviewWacaiCSV(ctx context.Context, request PreviewRequest) (Batch, error) {
	if request.Actor.UserID == "" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if strings.TrimSpace(request.Filename) == "" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "filename is required")
	}
	if len(request.Data) == 0 {
		return Batch{}, errors.Wrap(ErrInvalidInput, "import file is empty")
	}
	if len(request.Data) > maxPreviewBytes {
		return Batch{}, errors.Wrap(ErrInvalidInput, "import file is too large")
	}
	if strings.ToLower(filepath.Ext(request.Filename)) != ".csv" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "only csv preview is supported")
	}

	sourceHash := sourceHash(request.Data)
	if existing, err := s.store.BatchByHash(ctx, request.Actor.UserID, sourceWacai, sourceHash); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrNotFound) {
		return Batch{}, errors.Wrap(err, "load import batch by hash")
	}

	rows, schema, detected, warningCount, errorCount, err := parseWacaiCSV(request.Data)
	if err != nil {
		return Batch{}, err
	}

	now := time.Now().UTC()
	batch := Batch{
		ID:             uuid.NewString(),
		UserID:         request.Actor.UserID,
		Source:         sourceWacai,
		Filename:       strings.TrimSpace(request.Filename),
		ContentType:    strings.TrimSpace(request.ContentType),
		SourceHash:     sourceHash,
		ParserVersion:  parserVersion,
		Status:         BatchStatusPreview,
		DetectedSchema: schema,
		Rows:           rows,
		Detected:       detected,
		ErrorCount:     errorCount,
		WarningCount:   warningCount,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	created, err := s.store.SaveBatch(ctx, batch)
	if err != nil {
		return Batch{}, errors.Wrap(err, "save import batch")
	}

	return created, nil
}

// Batch receives an actor and batch id, verifies ownership, and returns the stored import batch.
func (s *Service) Batch(ctx context.Context, request BatchRequest) (Batch, error) {
	if request.Actor.UserID == "" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if strings.TrimSpace(request.BatchID) == "" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "batch id is required")
	}

	batch, err := s.store.Batch(ctx, request.Actor.UserID, strings.TrimSpace(request.BatchID))
	if err != nil {
		return Batch{}, errors.Wrap(err, "load import batch")
	}

	return batch, nil
}

// MarkApplied receives commit metadata, verifies ownership, and stores an applied import batch.
func (s *Service) MarkApplied(ctx context.Context, request MarkAppliedRequest) (Batch, error) {
	if request.Actor.UserID == "" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "actor user id is required")
	}
	if strings.TrimSpace(request.BatchID) == "" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "batch id is required")
	}
	if strings.TrimSpace(request.BookID) == "" {
		return Batch{}, errors.Wrap(ErrInvalidInput, "book id is required")
	}

	entryIDs := normalizeAppliedEntryIDs(request.EntryIDs)
	if len(entryIDs) == 0 {
		return Batch{}, errors.Wrap(ErrInvalidInput, "applied entry ids are required")
	}

	batch, err := s.store.Batch(ctx, request.Actor.UserID, strings.TrimSpace(request.BatchID))
	if err != nil {
		return Batch{}, errors.Wrap(err, "load import batch")
	}
	if batch.Status == BatchStatusApplied {
		if batch.AppliedBookID != strings.TrimSpace(request.BookID) {
			return Batch{}, errors.Wrap(ErrInvalidInput, "import batch already applied to another book")
		}

		return batch, nil
	}

	now := time.Now().UTC()
	batch.Status = BatchStatusApplied
	batch.AppliedBookID = strings.TrimSpace(request.BookID)
	batch.AppliedEntryIDs = entryIDs
	batch.AppliedSkippedRows = cloneAppliedSkippedRows(request.SkippedRows)
	batch.AppliedAt = &now
	batch.UpdatedAt = now

	updated, err := s.store.SaveBatch(ctx, batch)
	if err != nil {
		return Batch{}, errors.Wrap(err, "save applied import batch")
	}

	return updated, nil
}

// sourceHash receives source bytes and returns a SHA-256 hex digest.
func sourceHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// normalizeAppliedEntryIDs receives entry ids and returns trimmed unique non-empty ids in source order.
func normalizeAppliedEntryIDs(entryIDs []string) []string {
	normalized := make([]string, 0, len(entryIDs))
	for _, entryID := range entryIDs {
		entryID = strings.TrimSpace(entryID)
		if entryID != "" && !slices.Contains(normalized, entryID) {
			normalized = append(normalized, entryID)
		}
	}

	return normalized
}

// parseWacaiCSV receives CSV bytes and returns preview rows, schema, detected values, and diagnostics.
func parseWacaiCSV(data []byte) ([]PreviewRow, DetectedSchema, DetectedValues, int, int, error) {
	// Wacai exports are commonly saved as UTF-8 with a byte-order mark. Strip a
	// leading BOM so the first header cell (usually the date column) matches the
	// alias table; otherwise schema detection misses it and every row fails with
	// "occurredAt is required". The source hash is computed on the raw upload
	// before this call, so trimming here does not affect idempotency.
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, DetectedSchema{}, DetectedValues{}, 0, 0, errors.Wrap(ErrInvalidInput, "parse csv")
	}
	if len(records) < 2 {
		return nil, DetectedSchema{}, DetectedValues{}, 0, 0, errors.Wrap(ErrInvalidInput, "csv requires header and rows")
	}

	headers := normalizeHeaders(records[0])
	schema := detectSchema(headers)
	rows := make([]PreviewRow, 0, min(len(records)-1, maxPreviewRows))
	detected := detectedCollector{}
	warningCount := 0
	errorCount := 0
	for index, record := range records[1:] {
		if index >= maxPreviewRows {
			break
		}
		row := buildPreviewRow(index+2, headers, record, schema)
		validatePreviewRow(&row)
		detected.add(row)
		warningCount += len(row.Warnings)
		errorCount += len(row.Errors)
		rows = append(rows, row)
	}

	return rows, schema, detected.values(), warningCount, errorCount, nil
}

// normalizeHeaders receives raw CSV headers and returns trimmed headers.
func normalizeHeaders(headers []string) []string {
	normalized := make([]string, 0, len(headers))
	for _, header := range headers {
		normalized = append(normalized, strings.TrimSpace(header))
	}

	return normalized
}

// detectSchema receives normalized headers and returns a source-column mapping.
func detectSchema(headers []string) DetectedSchema {
	columns := map[string]string{}
	for field, aliases := range headerAliases() {
		for _, header := range headers {
			if slices.Contains(aliases, strings.ToLower(strings.TrimSpace(header))) {
				columns[field] = header
				break
			}
		}
	}

	missing := make([]string, 0)
	for _, field := range []string{"occurredAt", "amount", "type"} {
		if columns[field] == "" {
			missing = append(missing, field)
		}
	}

	return DetectedSchema{Columns: columns, Missing: missing}
}

// headerAliases returns supported Wacai-style CSV header aliases.
func headerAliases() map[string][]string {
	return map[string][]string{
		"type":       {"type", "transaction type", "bill type", "类型", "收支类型"},
		"occurredAt": {"date", "time", "datetime", "occurred at", "日期", "时间", "交易时间"},
		"amount":     {"amount", "money", "金额"},
		"currency":   {"currency", "币种"},
		"account":    {"account", "账户", "账号"},
		"category":   {"category", "分类"},
		"book":       {"book", "账本"},
		"member":     {"member", "成员"},
		"merchant":   {"merchant", "payee", "商家", "对象"},
		"note":       {"note", "memo", "备注"},
		"tags":       {"tags", "tag", "标签"},
	}
}

// buildPreviewRow receives one CSV record and returns its preview row.
func buildPreviewRow(rowNumber int, headers []string, record []string, schema DetectedSchema) PreviewRow {
	raw := make(map[string]string, len(headers))
	for index, header := range headers {
		if index < len(record) {
			raw[header] = strings.TrimSpace(record[index])
			continue
		}
		raw[header] = ""
	}

	row := PreviewRow{
		RowNumber:  rowNumber,
		Raw:        raw,
		Type:       valueFor(schema, raw, "type"),
		OccurredAt: valueFor(schema, raw, "occurredAt"),
		Amount:     valueFor(schema, raw, "amount"),
		Currency:   strings.ToUpper(valueFor(schema, raw, "currency")),
		Account:    valueFor(schema, raw, "account"),
		Category:   valueFor(schema, raw, "category"),
		Book:       valueFor(schema, raw, "book"),
		Member:     valueFor(schema, raw, "member"),
		Merchant:   valueFor(schema, raw, "merchant"),
		Note:       valueFor(schema, raw, "note"),
		Tags:       splitTags(valueFor(schema, raw, "tags")),
	}
	if row.Currency == "" {
		row.Currency = defaultCurrency
		row.Warnings = append(row.Warnings, "currency missing; defaulted to CNY")
	}

	return row
}

// valueFor receives a schema, raw row, and field name and returns the mapped raw value.
func valueFor(schema DetectedSchema, raw map[string]string, field string) string {
	header := schema.Columns[field]
	if header == "" {
		return ""
	}

	return strings.TrimSpace(raw[header])
}

// splitTags receives a raw tag string and returns normalized tags.
func splitTags(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == '，'
	})
	tags := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" && !slices.Contains(tags, field) {
			tags = append(tags, field)
		}
	}

	return tags
}

// validatePreviewRow receives a preview row and appends row-level diagnostics.
func validatePreviewRow(row *PreviewRow) {
	if strings.TrimSpace(row.Type) == "" {
		row.Errors = append(row.Errors, "type is required")
	}
	if strings.TrimSpace(row.Amount) == "" {
		row.Errors = append(row.Errors, "amount is required")
	} else if _, err := strconv.ParseFloat(strings.ReplaceAll(row.Amount, ",", ""), 64); err != nil {
		row.Errors = append(row.Errors, "amount is invalid")
	}
	if strings.TrimSpace(row.OccurredAt) == "" {
		row.Errors = append(row.Errors, "occurredAt is required")
	} else if !canParseDate(row.OccurredAt) {
		row.Warnings = append(row.Warnings, "occurredAt format needs review")
	}
	if strings.TrimSpace(row.Account) == "" {
		row.Warnings = append(row.Warnings, "account is missing")
	}
	if strings.TrimSpace(row.Category) == "" {
		row.Warnings = append(row.Warnings, "category is missing")
	}
}

// canParseDate receives a raw date string and reports whether a common Wacai date layout matches.
func canParseDate(raw string) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02", "2006/01/02 15:04:05", "2006/01/02"} {
		if _, err := time.ParseInLocation(layout, raw, time.UTC); err == nil {
			return true
		}
	}

	return false
}

type detectedCollector struct {
	books      []string
	accounts   []string
	categories []string
	currencies []string
	members    []string
	merchants  []string
	tags       []string
}

// add receives a preview row and records unique detected values.
func (d *detectedCollector) add(row PreviewRow) {
	d.books = appendUnique(d.books, row.Book)
	d.accounts = appendUnique(d.accounts, row.Account)
	d.categories = appendUnique(d.categories, row.Category)
	d.currencies = appendUnique(d.currencies, row.Currency)
	d.members = appendUnique(d.members, row.Member)
	d.merchants = appendUnique(d.merchants, row.Merchant)
	for _, tag := range row.Tags {
		d.tags = appendUnique(d.tags, tag)
	}
}

// values returns the collected detected values.
func (d detectedCollector) values() DetectedValues {
	return DetectedValues{
		Books:      d.books,
		Accounts:   d.accounts,
		Categories: d.categories,
		Currencies: d.currencies,
		Members:    d.members,
		Merchants:  d.merchants,
		Tags:       d.tags,
	}
}

// appendUnique receives a list and value and returns the list with the trimmed value appended once.
func appendUnique(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || slices.Contains(values, value) {
		return values
	}

	return append(values, value)
}
