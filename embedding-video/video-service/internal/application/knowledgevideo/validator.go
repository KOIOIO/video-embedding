package knowledgevideo

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type ValidationLimits struct {
	MaxArchiveBytes  int64
	MaxExpandedBytes int64
	MaxEntryBytes    int64
	MaxEntries       int
}

type ValidationIssue struct {
	Row     int    `json:"row,omitempty"`
	Field   string `json:"field,omitempty"`
	Message string `json:"message"`
}

type ValidationError struct {
	Issues []ValidationIssue `json:"issues"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("knowledge video validation failed with %d issue(s)", len(e.Issues))
}

type ValidatedRow struct {
	ImportRow
	ArchiveEntryName string
	UncompressedSize uint64
}

type ValidatedManifest struct {
	Rows []ValidatedRow
}

type Validator struct {
	Dictionary DictionaryLookup
	Limits     ValidationLimits
}

func (v Validator) Validate(ctx context.Context, archive io.Reader, mapping io.Reader) (ValidatedManifest, error) {
	rows, issues := parseMapping(mapping)
	archiveEntries, archiveIssues := v.inspectArchive(archive)
	issues = append(issues, archiveIssues...)

	if len(rows) > 0 && v.Dictionary != nil {
		dictionaryIssues, err := v.validateDictionary(ctx, rows)
		if err != nil {
			return ValidatedManifest{}, err
		}
		issues = append(issues, dictionaryIssues...)
	}
	manifest, mappingIssues := matchArchiveRows(rows, archiveEntries)
	issues = append(issues, mappingIssues...)
	if len(issues) > 0 {
		return ValidatedManifest{}, &ValidationError{Issues: issues}
	}
	return manifest, nil
}

type parsedMappingRow struct {
	ImportRow
	worksheetRow int
}

func parseMapping(reader io.Reader) ([]parsedMappingRow, []ValidationIssue) {
	if reader == nil {
		return nil, []ValidationIssue{{Field: "mapping", Message: "mapping is required"}}
	}
	book, err := excelize.OpenReader(reader)
	if err != nil {
		return nil, []ValidationIssue{{Field: "mapping", Message: "mapping is not a valid XLSX file"}}
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, []ValidationIssue{{Field: "mapping", Message: "mapping must contain a worksheet"}}
	}
	sheet := sheets[0]
	expectedHeaders := []string{"id", "name", "video_name"}
	issues := make([]ValidationIssue, 0)
	for column, expected := range expectedHeaders {
		cell, _ := excelize.CoordinatesToCellName(column+1, 1)
		value, cellErr := book.GetCellValue(sheet, cell)
		if cellErr != nil || value != expected {
			issues = append(issues, ValidationIssue{Row: 1, Field: expected, Message: "header must be exactly " + expected})
		}
	}
	if value, _ := book.GetCellValue(sheet, "D1"); value != "" {
		issues = append(issues, ValidationIssue{Row: 1, Field: "mapping", Message: "mapping must contain exactly three columns"})
	}

	sheetRows, err := book.GetRows(sheet)
	if err != nil {
		return nil, []ValidationIssue{{Field: "mapping", Message: "mapping worksheet cannot be read"}}
	}
	rows := make([]parsedMappingRow, 0, len(sheetRows))
	seenNames := make(map[string]struct{})
	for index := 1; index < len(sheetRows); index++ {
		values := sheetRows[index]
		idValue := mappingCell(values, 0)
		name := strings.TrimSpace(mappingCell(values, 1))
		videoName := strings.TrimSpace(mappingCell(values, 2))
		if idValue == "" && name == "" && videoName == "" {
			continue
		}
		rowNumber := index + 1
		id, parseErr := strconv.ParseUint(strings.TrimSpace(idValue), 10, 64)
		if parseErr != nil || id == 0 {
			issues = append(issues, ValidationIssue{Row: rowNumber, Field: "id", Message: "id must be a positive integer"})
		}
		if name == "" {
			issues = append(issues, ValidationIssue{Row: rowNumber, Field: "name", Message: "name is required"})
		}
		if videoName == "" {
			issues = append(issues, ValidationIssue{Row: rowNumber, Field: "video_name", Message: "video name is required"})
		} else if _, exists := seenNames[videoName]; exists {
			issues = append(issues, ValidationIssue{Row: rowNumber, Field: "video_name", Message: "duplicate video name"})
		} else {
			seenNames[videoName] = struct{}{}
		}
		if parseErr == nil && id > 0 && name != "" && videoName != "" {
			rows = append(rows, parsedMappingRow{
				ImportRow:    ImportRow{KnowledgePointID: id, KnowledgePointName: name, SourceFileName: videoName},
				worksheetRow: rowNumber,
			})
		}
	}
	if len(sheetRows) <= 1 {
		issues = append(issues, ValidationIssue{Field: "mapping", Message: "mapping must contain at least one data row"})
	}
	return rows, issues
}

func mappingCell(row []string, index int) string {
	if index >= len(row) {
		return ""
	}
	return row[index]
}

type inspectedArchiveEntry struct {
	name             string
	base             string
	uncompressedSize uint64
}

func (v Validator) inspectArchive(reader io.Reader) (map[string]inspectedArchiveEntry, []ValidationIssue) {
	if reader == nil {
		return nil, []ValidationIssue{{Field: "archive", Message: "archive is required"}}
	}
	temp, err := os.CreateTemp("", "knowledge-video-*.zip")
	if err != nil {
		return nil, []ValidationIssue{{Field: "archive", Message: "archive cannot be inspected"}}
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	limit := v.Limits.MaxArchiveBytes
	copyReader := reader
	if limit > 0 {
		copyReader = io.LimitReader(reader, limit+1)
	}
	written, copyErr := io.Copy(temp, copyReader)
	closeErr := temp.Close()
	if copyErr != nil || closeErr != nil {
		return nil, []ValidationIssue{{Field: "archive", Message: "archive cannot be read"}}
	}
	if limit > 0 && written > limit {
		return nil, []ValidationIssue{{Field: "archive", Message: "archive exceeds compressed size limit"}}
	}
	archive, err := zip.OpenReader(tempName)
	if err != nil {
		return nil, []ValidationIssue{{Field: "archive", Message: "archive is not a valid ZIP file"}}
	}
	defer archive.Close()

	entries := make(map[string]inspectedArchiveEntry)
	issues := make([]ValidationIssue, 0)
	var expanded uint64
	if v.Limits.MaxEntries > 0 && len(archive.File) > v.Limits.MaxEntries {
		issues = append(issues, ValidationIssue{Field: "archive", Message: "archive contains too many entries"})
	}
	for _, file := range archive.File {
		name := file.Name
		clean := path.Clean(name)
		if strings.HasPrefix(name, "/") || clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(name) || strings.Contains(name, "\\") {
			issues = append(issues, ValidationIssue{Field: "archive", Message: "archive entry path is unsafe"})
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			issues = append(issues, ValidationIssue{Field: "archive", Message: "archive entry must not be a symbolic link"})
			continue
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if isMetadataEntry(clean) {
			continue
		}
		base := path.Base(clean)
		if !isSupportedVideoExtension(path.Ext(base)) {
			issues = append(issues, ValidationIssue{Field: "archive", Message: "unsupported video extension"})
			continue
		}
		if v.Limits.MaxEntryBytes > 0 && file.UncompressedSize64 > uint64(v.Limits.MaxEntryBytes) {
			issues = append(issues, ValidationIssue{Field: "archive", Message: "archive entry exceeds size limit"})
		}
		expanded += file.UncompressedSize64
		if _, exists := entries[base]; exists {
			issues = append(issues, ValidationIssue{Field: "archive", Message: "duplicate archive basename"})
			continue
		}
		entries[base] = inspectedArchiveEntry{name: name, base: base, uncompressedSize: file.UncompressedSize64}
	}
	if v.Limits.MaxExpandedBytes > 0 && expanded > uint64(v.Limits.MaxExpandedBytes) {
		issues = append(issues, ValidationIssue{Field: "archive", Message: "archive expanded size exceeds limit"})
	}
	return entries, issues
}

func isMetadataEntry(name string) bool {
	for _, component := range strings.Split(name, "/") {
		if component == "__MACOSX" || component == ".DS_Store" || strings.HasPrefix(component, "._") {
			return true
		}
	}
	return false
}

func isSupportedVideoExtension(extension string) bool {
	switch extension {
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".m4v":
		return true
	default:
		return false
	}
}

func (v Validator) validateDictionary(ctx context.Context, rows []parsedMappingRow) ([]ValidationIssue, error) {
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.KnowledgePointID)
	}
	points, err := v.Dictionary.LookupKnowledgePoints(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[uint64]string, len(points))
	for _, point := range points {
		names[point.ID] = strings.TrimSpace(point.Name)
	}
	issues := make([]ValidationIssue, 0)
	for _, row := range rows {
		name, exists := names[row.KnowledgePointID]
		if !exists {
			issues = append(issues, ValidationIssue{Row: row.worksheetRow, Field: "name", Message: "knowledge point does not exist"})
		} else if row.KnowledgePointName != name {
			issues = append(issues, ValidationIssue{Row: row.worksheetRow, Field: "name", Message: "knowledge point name does not match dictionary"})
		}
	}
	return issues, nil
}

func matchArchiveRows(rows []parsedMappingRow, entries map[string]inspectedArchiveEntry) (ValidatedManifest, []ValidationIssue) {
	manifest := ValidatedManifest{Rows: make([]ValidatedRow, 0, len(rows))}
	referenced := make(map[string]struct{}, len(rows))
	issues := make([]ValidationIssue, 0)
	for _, row := range rows {
		entry, exists := entries[row.SourceFileName]
		if !exists {
			issues = append(issues, ValidationIssue{Row: row.worksheetRow, Field: "video_name", Message: "video is missing from archive"})
			continue
		}
		referenced[row.SourceFileName] = struct{}{}
		manifest.Rows = append(manifest.Rows, ValidatedRow{ImportRow: row.ImportRow, ArchiveEntryName: entry.name, UncompressedSize: entry.uncompressedSize})
	}
	for base := range entries {
		if _, exists := referenced[base]; !exists {
			issues = append(issues, ValidationIssue{Field: "archive", Message: "archive video is not referenced by mapping"})
		}
	}
	return manifest, issues
}
