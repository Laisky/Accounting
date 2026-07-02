// Package imports contains source-file import preview and staging use cases.
package imports

import (
	"time"

	"github.com/Laisky/errors/v2"
)

// ErrInvalidInput indicates that import input failed validation.
var ErrInvalidInput = errors.New("import input is invalid")

// ErrNotFound indicates that an import resource does not exist.
var ErrNotFound = errors.New("import resource not found")

// PreviewRequest contains actor and uploaded source file data for a Wacai preview.
type PreviewRequest struct {
	Actor       Actor
	Filename    string
	ContentType string
	Data        []byte
}

// Actor carries the authenticated user identity for import ownership.
type Actor struct {
	UserID string
}

// BatchStatus identifies the lifecycle state for an import batch.
type BatchStatus string

const (
	// BatchStatusPreview identifies an import batch that has only been parsed for review.
	BatchStatusPreview BatchStatus = "preview"
)

// Batch contains stored import batch metadata and preview rows.
type Batch struct {
	ID             string         `json:"id"`
	UserID         string         `json:"userId"`
	Source         string         `json:"source"`
	Filename       string         `json:"filename"`
	ContentType    string         `json:"contentType"`
	SourceHash     string         `json:"sourceHash"`
	ParserVersion  string         `json:"parserVersion"`
	Status         BatchStatus    `json:"status"`
	DetectedSchema DetectedSchema `json:"detectedSchema"`
	Rows           []PreviewRow   `json:"rows"`
	Detected       DetectedValues `json:"detected"`
	ErrorCount     int            `json:"errorCount"`
	WarningCount   int            `json:"warningCount"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

// DetectedSchema describes the source header mapping used by the parser.
type DetectedSchema struct {
	Columns map[string]string `json:"columns"`
	Missing []string          `json:"missing,omitempty"`
}

// PreviewRow contains one parsed source row and row-level diagnostics.
type PreviewRow struct {
	RowNumber  int               `json:"rowNumber"`
	Raw        map[string]string `json:"raw"`
	Type       string            `json:"type,omitempty"`
	OccurredAt string            `json:"occurredAt,omitempty"`
	Amount     string            `json:"amount,omitempty"`
	Currency   string            `json:"currency,omitempty"`
	Account    string            `json:"account,omitempty"`
	Category   string            `json:"category,omitempty"`
	Book       string            `json:"book,omitempty"`
	Member     string            `json:"member,omitempty"`
	Merchant   string            `json:"merchant,omitempty"`
	Note       string            `json:"note,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Warnings   []string          `json:"warnings,omitempty"`
	Errors     []string          `json:"errors,omitempty"`
}

// DetectedValues contains unique values discovered during preview parsing.
type DetectedValues struct {
	Books      []string `json:"books,omitempty"`
	Accounts   []string `json:"accounts,omitempty"`
	Categories []string `json:"categories,omitempty"`
	Currencies []string `json:"currencies,omitempty"`
	Members    []string `json:"members,omitempty"`
	Merchants  []string `json:"merchants,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}
