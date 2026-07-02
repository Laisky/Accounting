package imports

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/Laisky/errors/v2"
)

const maxXLSXWorksheetBytes = 32 * 1024 * 1024

var cellColumnPattern = regexp.MustCompile(`^[A-Z]+`)

type wacaiMetadata struct {
	Book string
}

type xlsxWorksheet struct {
	Rows []xlsxRow `xml:"sheetData>row"`
}

type xlsxRow struct {
	Cells []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	Ref          string         `xml:"r,attr"`
	Type         string         `xml:"t,attr"`
	Value        string         `xml:"v"`
	InlineString xlsxInlineText `xml:"is"`
}

type xlsxInlineText struct {
	Text     string         `xml:"t"`
	RichText []xlsxRichText `xml:"r"`
}

type xlsxRichText struct {
	Text string `xml:"t"`
}

type xlsxSharedStrings struct {
	Items []xlsxSharedString `xml:"si"`
}

type xlsxSharedString struct {
	Text     string         `xml:"t"`
	RichText []xlsxRichText `xml:"r"`
}

// parseWacaiXLSX receives XLSX bytes and returns sheet records plus Wacai workbook metadata.
func parseWacaiXLSX(data []byte) ([][]string, wacaiMetadata, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, wacaiMetadata{}, errors.Wrap(ErrInvalidInput, "open xlsx")
	}

	sharedStrings, err := readXLSXSharedStrings(reader.File)
	if err != nil {
		return nil, wacaiMetadata{}, err
	}
	worksheetFile := findXLSXFile(reader.File, "xl/worksheets/sheet1.xml")
	if worksheetFile == nil {
		return nil, wacaiMetadata{}, errors.Wrap(ErrInvalidInput, "xlsx sheet1 is missing")
	}
	if worksheetFile.UncompressedSize64 > maxXLSXWorksheetBytes {
		return nil, wacaiMetadata{}, errors.Wrap(ErrInvalidInput, "xlsx worksheet is too large")
	}

	worksheet, err := readXLSXWorksheet(worksheetFile)
	if err != nil {
		return nil, wacaiMetadata{}, err
	}

	records := make([][]string, 0, len(worksheet.Rows))
	metadata := wacaiMetadata{}
	for _, row := range worksheet.Rows {
		record := xlsxRecordFromRow(row, sharedStrings)
		records = append(records, record)
		metadata = mergeWacaiMetadata(metadata, record)
	}
	if len(records) < 2 {
		return nil, wacaiMetadata{}, errors.Wrap(ErrInvalidInput, "xlsx requires header and rows")
	}

	return records, metadata, nil
}

// readXLSXSharedStrings receives workbook files and returns shared-string table values.
func readXLSXSharedStrings(files []*zip.File) ([]string, error) {
	file := findXLSXFile(files, "xl/sharedStrings.xml")
	if file == nil {
		return []string{}, nil
	}
	if file.UncompressedSize64 > maxXLSXWorksheetBytes {
		return nil, errors.Wrap(ErrInvalidInput, "xlsx shared strings are too large")
	}

	handle, err := file.Open()
	if err != nil {
		return nil, errors.Wrap(ErrInvalidInput, "open xlsx shared strings")
	}
	defer func() {
		_ = handle.Close()
	}()

	var parsed xlsxSharedStrings
	if err := xml.NewDecoder(io.LimitReader(handle, maxXLSXWorksheetBytes)).Decode(&parsed); err != nil {
		return nil, errors.Wrapf(ErrInvalidInput, "parse xlsx shared strings: %v", err)
	}

	values := make([]string, 0, len(parsed.Items))
	for _, item := range parsed.Items {
		values = append(values, strings.TrimSpace(xlsxSharedStringText(item)))
	}

	return values, nil
}

// readXLSXWorksheet receives a worksheet file and decodes its XML.
func readXLSXWorksheet(file *zip.File) (xlsxWorksheet, error) {
	handle, err := file.Open()
	if err != nil {
		return xlsxWorksheet{}, errors.Wrap(ErrInvalidInput, "open xlsx worksheet")
	}
	defer func() {
		_ = handle.Close()
	}()

	var worksheet xlsxWorksheet
	if err := xml.NewDecoder(io.LimitReader(handle, maxXLSXWorksheetBytes)).Decode(&worksheet); err != nil {
		return xlsxWorksheet{}, errors.Wrapf(ErrInvalidInput, "parse xlsx worksheet: %v", err)
	}

	return worksheet, nil
}

// findXLSXFile receives workbook files and returns a file by exact path.
func findXLSXFile(files []*zip.File, name string) *zip.File {
	for _, file := range files {
		if file.Name == name {
			return file
		}
	}

	return nil
}

// xlsxRecordFromRow receives one worksheet row and returns a dense record with blank cells preserved.
func xlsxRecordFromRow(row xlsxRow, sharedStrings []string) []string {
	record := make([]string, 0, len(row.Cells))
	for _, cell := range row.Cells {
		index := xlsxCellIndex(cell.Ref)
		for len(record) <= index {
			record = append(record, "")
		}
		record[index] = xlsxCellValue(cell, sharedStrings)
	}

	return record
}

// xlsxCellIndex receives an Excel cell reference and returns its zero-based column index.
func xlsxCellIndex(ref string) int {
	column := cellColumnPattern.FindString(strings.ToUpper(strings.TrimSpace(ref)))
	if column == "" {
		return 0
	}

	index := 0
	for _, char := range column {
		index *= 26
		index += int(char-'A') + 1
	}

	return index - 1
}

// xlsxCellValue receives one parsed cell and returns its normalized display value.
func xlsxCellValue(cell xlsxCell, sharedStrings []string) string {
	switch cell.Type {
	case "inlineStr":
		if cell.InlineString.Text != "" {
			return strings.TrimSpace(cell.InlineString.Text)
		}
		return strings.TrimSpace(joinXLSXRichText(cell.InlineString.RichText))
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err != nil || index < 0 || index >= len(sharedStrings) {
			return ""
		}
		return sharedStrings[index]
	default:
		return strings.TrimSpace(cell.Value)
	}
}

// xlsxSharedStringText receives one shared-string item and returns its text value.
func xlsxSharedStringText(item xlsxSharedString) string {
	if item.Text != "" {
		return item.Text
	}

	return joinXLSXRichText(item.RichText)
}

// joinXLSXRichText receives rich text runs and returns their concatenated text.
func joinXLSXRichText(runs []xlsxRichText) string {
	var builder strings.Builder
	for _, run := range runs {
		builder.WriteString(run.Text)
	}

	return builder.String()
}

// mergeWacaiMetadata receives a current metadata value and one row, then returns enriched metadata.
func mergeWacaiMetadata(metadata wacaiMetadata, record []string) wacaiMetadata {
	if len(record) == 0 {
		return metadata
	}

	const bookPrefix = "导出账本："
	first := strings.TrimSpace(record[0])
	if strings.HasPrefix(first, bookPrefix) {
		metadata.Book = strings.TrimSpace(strings.TrimPrefix(first, bookPrefix))
	}

	return metadata
}
