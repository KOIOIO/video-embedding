# Ignore macOS ZIP Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow knowledge-video ZIP uploads containing macOS metadata while preserving all existing archive safety checks.

**Architecture:** Keep metadata recognition in `isMetadataEntry` and change only the archive inspection branch from validation-error creation to silent skipping. Prove the behavior through the public `Validator.Validate` method so matching, dictionary validation, and archive inspection run together.

**Tech Stack:** Go, `archive/zip`, table-driven tests.

---

## File Map

- Modify `internal/application/knowledgevideo/validator.go`: skip recognized macOS metadata entries.
- Modify `internal/application/knowledgevideo/validator_test.go`: accept three metadata forms alongside a valid mapped video while retaining unsafe-entry rejection tests.

### Task 1: Ignore Recognized macOS Metadata

- [ ] **Step 1: Write the failing test**

Replace the metadata rejection case with a table-driven test that builds an archive containing `lesson.mp4` plus one of `__MACOSX/lesson.mp4`, `.DS_Store`, or `._lesson.mp4`, then asserts validation succeeds with exactly one manifest row.

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/application/knowledgevideo -run 'Validator(IgnoresMacOSMetadata|RejectsUnsafe)' -count=1`

Expected: FAIL because metadata entries still append `archive metadata entries are not allowed`.

- [ ] **Step 3: Implement the minimal behavior change**

In `Validator.inspectArchive`, retain the early metadata check but skip the entry without appending an issue:

```go
if isMetadataEntry(clean) {
	continue
}
```

Do not change unsafe-path, symlink, unsupported-extension, size-limit, or basename checks.

- [ ] **Step 4: Verify the focused and package tests**

Run: `go test ./internal/application/knowledgevideo -run 'Validator' -count=1`

Run: `go test ./internal/application/knowledgevideo ./internal/http/handler/knowledgevideos -count=1`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/application/knowledgevideo/validator.go internal/application/knowledgevideo/validator_test.go
git commit -m "fix: ignore macos metadata in knowledge video archives"
```
