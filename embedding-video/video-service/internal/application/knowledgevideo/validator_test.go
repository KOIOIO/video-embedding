package knowledgevideo

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestValidatorAcceptsValidArchiveAndMapping(t *testing.T) {
	v := Validator{Dictionary: fakeDictionary{9: "一次函数", 10: "二次函数"}, Limits: testValidationLimits()}
	manifest, err := v.Validate(context.Background(), zipArchive(t,
		zipEntry{name: "chapter/lesson.mp4", body: "video-one"},
		zipEntry{name: "second.webm", body: "video-two"},
	), mappingXLSX(t, [][]string{
		{"id", "name", "video_name"},
		{"9", " 一次函数 ", " lesson.mp4 "},
		{"10", "二次函数", "second.webm"},
	}))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(manifest.Rows) != 2 || manifest.Rows[0].KnowledgePointID != 9 || manifest.Rows[0].SourceFileName != "lesson.mp4" || manifest.Rows[0].ArchiveEntryName != "chapter/lesson.mp4" {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestValidatorAllowsMultipleVideosForOneKnowledgePoint(t *testing.T) {
	v := Validator{Dictionary: fakeDictionary{9: "一次函数"}, Limits: testValidationLimits()}
	manifest, err := v.Validate(context.Background(), zipArchive(t,
		zipEntry{name: "first.mp4", body: "video-one"},
		zipEntry{name: "second.mp4", body: "video-two"},
	), mappingXLSX(t, [][]string{
		{"id", "name", "video_name"},
		{"9", "一次函数", "first.mp4"},
		{"9", "一次函数", "second.mp4"},
	}))
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if len(manifest.Rows) != 2 || manifest.Rows[0].KnowledgePointID != 9 || manifest.Rows[1].KnowledgePointID != 9 {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestValidatorRejectsInvalidXLSXRows(t *testing.T) {
	tests := []struct {
		name    string
		rows    [][]string
		row     int
		field   string
		message string
	}{
		{name: "headers", rows: [][]string{{"ID", "name", "video_name"}, {"9", "一次函数", "lesson.mp4"}}, row: 1, field: "id", message: "header must be exactly id"},
		{name: "empty name", rows: [][]string{{"id", "name", "video_name"}, {"9", "", "lesson.mp4"}}, row: 2, field: "name", message: "name is required"},
		{name: "empty video name", rows: [][]string{{"id", "name", "video_name"}, {"9", "一次函数", ""}}, row: 2, field: "video_name", message: "video name is required"},
		{name: "invalid id", rows: [][]string{{"id", "name", "video_name"}, {"abc", "一次函数", "lesson.mp4"}}, row: 2, field: "id", message: "id must be a positive integer"},
		{name: "zero id", rows: [][]string{{"id", "name", "video_name"}, {"0", "一次函数", "lesson.mp4"}}, row: 2, field: "id", message: "id must be a positive integer"},
		{name: "duplicate filename", rows: [][]string{{"id", "name", "video_name"}, {"9", "一次函数", "lesson.mp4"}, {"10", "二次函数", "lesson.mp4"}}, row: 3, field: "video_name", message: "duplicate video name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{Dictionary: fakeDictionary{9: "一次函数", 10: "二次函数"}, Limits: testValidationLimits()}
			_, err := v.Validate(context.Background(), zipArchive(t, zipEntry{name: "lesson.mp4", body: "video"}, zipEntry{name: "other.mp4", body: "video"}), mappingXLSX(t, tt.rows))
			assertValidationIssue(t, err, tt.row, tt.field, tt.message)
		})
	}
}

func TestValidatorRejectsDictionaryMismatch(t *testing.T) {
	tests := []struct {
		name       string
		dictionary fakeDictionary
		message    string
	}{
		{name: "missing id", dictionary: fakeDictionary{}, message: "knowledge point does not exist"},
		{name: "name mismatch", dictionary: fakeDictionary{9: "一次函数"}, message: "knowledge point name does not match dictionary"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := "二次函数"
			if tt.name == "missing id" {
				name = "一次函数"
			}
			v := Validator{Dictionary: tt.dictionary, Limits: testValidationLimits()}
			_, err := v.Validate(context.Background(), zipArchive(t, zipEntry{name: "lesson.mp4", body: "video"}), mappingXLSX(t, [][]string{{"id", "name", "video_name"}, {"9", name, "lesson.mp4"}}))
			assertValidationIssue(t, err, 2, "name", tt.message)
		})
	}
}

func TestValidatorPreservesWorksheetRowInDictionaryIssue(t *testing.T) {
	v := Validator{Dictionary: fakeDictionary{}, Limits: testValidationLimits()}
	_, err := v.Validate(context.Background(), zipArchive(t, zipEntry{name: "lesson.mp4", body: "video"}), mappingXLSX(t, [][]string{
		{"id", "name", "video_name"},
		{"invalid", "一次函数", "invalid.mp4"},
		{"9", "一次函数", "lesson.mp4"},
	}))
	assertValidationIssue(t, err, 3, "name", "knowledge point does not exist")
}

func TestValidatorPropagatesDictionaryLookupError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	v := Validator{Dictionary: failingDictionary{err: wantErr}, Limits: testValidationLimits()}
	_, err := v.Validate(context.Background(), zipArchive(t, zipEntry{name: "lesson.mp4", body: "video"}), mappingXLSX(t, [][]string{
		{"id", "name", "video_name"},
		{"9", "一次函数", "lesson.mp4"},
	}))
	if !errors.Is(err, wantErr) {
		t.Fatalf("Validate() error = %v, want %v", err, wantErr)
	}
}

func TestValidatorRejectsUnsafeOrUnsupportedZIPEntries(t *testing.T) {
	tests := []struct {
		name    string
		entry   zipEntry
		message string
	}{
		{name: "traversal", entry: zipEntry{name: "../lesson.mp4", body: "video"}, message: "archive entry path is unsafe"},
		{name: "absolute", entry: zipEntry{name: "/lesson.mp4", body: "video"}, message: "archive entry path is unsafe"},
		{name: "symlink", entry: zipEntry{name: "lesson.mp4", body: "target", mode: fs.ModeSymlink | 0o777}, message: "archive entry must not be a symbolic link"},
		{name: "unsupported", entry: zipEntry{name: "lesson.txt", body: "video"}, message: "unsupported video extension"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{Dictionary: fakeDictionary{9: "一次函数"}, Limits: testValidationLimits()}
			_, err := v.Validate(context.Background(), zipArchive(t, tt.entry), mappingXLSX(t, [][]string{{"id", "name", "video_name"}, {"9", "一次函数", tt.entry.name}}))
			assertValidationIssue(t, err, 0, "archive", tt.message)
		})
	}
}

func TestValidatorIgnoresMacOSMetadataEntries(t *testing.T) {
	for _, metadataName := range []string{"__MACOSX/lesson.mp4", ".DS_Store", "._lesson.mp4"} {
		t.Run(metadataName, func(t *testing.T) {
			v := Validator{Dictionary: fakeDictionary{9: "一次函数"}, Limits: testValidationLimits()}
			manifest, err := v.Validate(context.Background(), zipArchive(t,
				zipEntry{name: "lesson.mp4", body: "video"},
				zipEntry{name: metadataName, body: "metadata"},
			), mappingXLSX(t, [][]string{{"id", "name", "video_name"}, {"9", "一次函数", "lesson.mp4"}}))
			if err != nil || len(manifest.Rows) != 1 {
				t.Fatalf("manifest=%+v err=%v", manifest, err)
			}
		})
	}
}

func TestValidatorRejectsArchiveMappingMismatch(t *testing.T) {
	tests := []struct {
		name    string
		entries []zipEntry
		video   string
		message string
	}{
		{name: "duplicate basename", entries: []zipEntry{{name: "a/lesson.mp4", body: "a"}, {name: "b/lesson.mp4", body: "b"}}, video: "lesson.mp4", message: "duplicate archive basename"},
		{name: "missing video", entries: []zipEntry{{name: "other.mp4", body: "a"}}, video: "lesson.mp4", message: "video is missing from archive"},
		{name: "extra video", entries: []zipEntry{{name: "lesson.mp4", body: "a"}, {name: "extra.mp4", body: "b"}}, video: "lesson.mp4", message: "archive video is not referenced by mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{Dictionary: fakeDictionary{9: "一次函数"}, Limits: testValidationLimits()}
			_, err := v.Validate(context.Background(), zipArchive(t, tt.entries...), mappingXLSX(t, [][]string{{"id", "name", "video_name"}, {"9", "一次函数", tt.video}}))
			assertValidationIssueMessage(t, err, tt.message)
		})
	}
}

func TestValidatorEnforcesArchiveLimits(t *testing.T) {
	tests := []struct {
		name    string
		limits  ValidationLimits
		entries []zipEntry
		message string
	}{
		{name: "entry count", limits: ValidationLimits{MaxEntries: 1, MaxEntryBytes: 100, MaxExpandedBytes: 200}, entries: []zipEntry{{name: "lesson.mp4", body: "123"}, {name: "extra.mp4", body: "123"}}, message: "archive contains too many entries"},
		{name: "entry size", limits: ValidationLimits{MaxEntries: 2, MaxEntryBytes: 2, MaxExpandedBytes: 200}, entries: []zipEntry{{name: "lesson.mp4", body: "123"}}, message: "archive entry exceeds size limit"},
		{name: "expanded size", limits: ValidationLimits{MaxEntries: 2, MaxEntryBytes: 100, MaxExpandedBytes: 5}, entries: []zipEntry{{name: "lesson.mp4", body: "123"}, {name: "extra.mp4", body: "456"}}, message: "archive expanded size exceeds limit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Validator{Dictionary: fakeDictionary{9: "一次函数"}, Limits: tt.limits}
			_, err := v.Validate(context.Background(), zipArchive(t, tt.entries...), mappingXLSX(t, [][]string{{"id", "name", "video_name"}, {"9", "一次函数", "lesson.mp4"}}))
			assertValidationIssueMessage(t, err, tt.message)
		})
	}
}

type fakeDictionary map[uint64]string

func (f fakeDictionary) LookupKnowledgePoints(_ context.Context, ids []uint64) ([]DictionaryKnowledgePoint, error) {
	points := make([]DictionaryKnowledgePoint, 0, len(ids))
	for _, id := range ids {
		if name, ok := f[id]; ok {
			points = append(points, DictionaryKnowledgePoint{ID: id, Name: name})
		}
	}
	return points, nil
}

type failingDictionary struct{ err error }

func (f failingDictionary) LookupKnowledgePoints(context.Context, []uint64) ([]DictionaryKnowledgePoint, error) {
	return nil, f.err
}

func testValidationLimits() ValidationLimits {
	return ValidationLimits{MaxEntries: 10, MaxEntryBytes: 1024, MaxExpandedBytes: 4096}
}

type zipEntry struct {
	name string
	body string
	mode fs.FileMode
}

func zipArchive(t *testing.T, entries ...zipEntry) io.Reader {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
		if entry.mode != 0 {
			header.SetMode(entry.mode)
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(file, entry.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buffer.Bytes())
}

func mappingXLSX(t *testing.T, rows [][]string) io.Reader {
	t.Helper()
	book := excelize.NewFile()
	t.Cleanup(func() { _ = book.Close() })
	sheet := book.GetSheetName(0)
	for rowIndex, row := range rows {
		for columnIndex, value := range row {
			cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
			if err != nil {
				t.Fatal(err)
			}
			if err := book.SetCellStr(sheet, cell, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	var buffer bytes.Buffer
	if err := book.Write(&buffer); err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(buffer.Bytes())
}

func assertValidationIssue(t *testing.T, err error, row int, field, message string) {
	t.Helper()
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
	for _, issue := range validationErr.Issues {
		if issue.Row == row && issue.Field == field && issue.Message == message {
			return
		}
	}
	t.Fatalf("issues = %+v, want row=%d field=%q message=%q", validationErr.Issues, row, field, message)
}

func assertValidationIssueMessage(t *testing.T, err error, message string) {
	t.Helper()
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("error = %v, want ValidationError", err)
	}
	for _, issue := range validationErr.Issues {
		if issue.Message == message {
			return
		}
	}
	t.Fatalf("issues = %+v, want message=%q", validationErr.Issues, message)
}
