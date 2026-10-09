package imports

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// syntheticWacaiXLSX receives a test and returns a valid synthetic workbook with
// metadata, a row-seven header, transfer accounts, and more rows than the preview limit.
func syntheticWacaiXLSX(t *testing.T) []byte {
	t.Helper()

	type cell struct {
		Reference string `xml:"r,attr"`
		Type      string `xml:"t,attr"`
		Text      string `xml:"is>t"`
	}
	type row struct {
		Number int    `xml:"r,attr"`
		Cells  []cell `xml:"c"`
	}
	type worksheet struct {
		XMLName   xml.Name `xml:"worksheet"`
		Namespace string   `xml:"xmlns,attr"`
		Rows      []row    `xml:"sheetData>row"`
	}
	records := make([][]string, 7, 7+maxPreviewRows+1)
	records[0] = []string{"\u5bfc\u51fa\u8d26\u672c\uff1aFixture household"}
	records[6] = []string{"\u65e5\u671f\u65f6\u95f4", "\u7c7b\u578b", "\u91d1\u989d", "\u5e01\u79cd", "\u8d26\u6237", "\u5206\u7c7b", "\u6210\u5458", "\u5907\u6ce8"}
	for index := range maxPreviewRows + 1 {
		records = append(records, []string{
			"2026-07-01", "\u8f6c\u8d26", "12.30", "\u4eba\u6c11\u5e01",
			"Cash: -12.30|Savings: +12.30", "", "Self", fmt.Sprintf("Synthetic transfer %d", index),
		})
	}
	sheet := worksheet{Namespace: "http://schemas.openxmlformats.org/spreadsheetml/2006/main"}
	for index, record := range records {
		r := row{Number: index + 1}
		for column, value := range record {
			r.Cells = append(r.Cells, cell{
				Reference: fmt.Sprintf("%c%d", 'A'+column, index+1),
				Type:      "inlineStr",
				Text:      value,
			})
		}
		sheet.Rows = append(sheet.Rows, r)
	}
	worksheetXML, err := xml.Marshal(sheet)
	require.NoError(t, err)

	files := []struct {
		name string
		data string
	}{
		{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Synthetic" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/worksheets/sheet1.xml", xml.Header + string(worksheetXML)},
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, file := range files {
		entry, err := archive.Create(file.name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(file.data))
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	return buffer.Bytes()
}
