# Ignore macOS ZIP Metadata Design

## Goal

Allow knowledge-video imports created by macOS archive tools without requiring users to manually remove platform metadata.

## Behavior

During ZIP inspection, silently skip entries when any path component is `__MACOSX`, `.DS_Store`, or starts with `._`. Skipped entries do not participate in video-name matching and do not count toward expanded-size accounting.

All other validation remains unchanged: unsafe paths, symbolic links, unsupported file extensions, duplicate video basenames, archive limits, dictionary mismatches, and missing mapped videos are still rejected.

An archive containing metadata plus valid mapped videos succeeds. An archive containing only metadata still fails through the existing mapping-mismatch validation because its mapped video is absent.

## Testing

Add table-driven coverage for all three metadata forms alongside a valid mapped video. Keep the existing unsafe-path, symbolic-link, and unsupported-extension tests to prove the security boundary remains intact.
