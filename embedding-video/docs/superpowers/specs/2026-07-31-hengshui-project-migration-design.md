# Project Migration and Sanitization Design

## Goal

Migrate the complete current working state of the source repository into
`embedding-video` while preserving the target repository's history and
target-only components. Remove business-specific naming and prevent passwords,
keys, tokens, and other secrets from entering tracked files or retained history.

## Baseline and Mapping

The source baseline is its current filesystem state: the two local commits ahead
of `origin/wwy_dev`, every tracked modification, and all untracked design and
plan documents. The target repository remains the Git authority.

| Source | Target |
| --- | --- |
| `hengshui-tablet-video-http/` | `video-service/` |
| `hengshui-tablet-video/` | `legacy-video/` |
| `hls-web/` | `hls-web/` |
| `recbole-training/` | `recbole-training/` |
| `gorse/` | `gorse/` |
| `deployment/` | `deployment/` |
| Root Compose and documentation files | Corresponding target paths |

The source working tree is the functional baseline for mapped files. Existing
target changes are retained when they are target-specific or more generic and
do not conflict with source behavior. Target-only content, including
`two-tower-training/`, is preserved.

## Naming

Tracked paths and text are scanned case-insensitively for business identifiers,
including `hengshui`, `hengtao`, and `yinlihupo`. They are replaced with
neutral terminology:

- active backend: `video-service`;
- legacy backend: `legacy-video`;
- repository or deployment project: `video-embedding` or a context-specific
  neutral service name;
- user-facing labels: functional labels without customer or organization names.

This includes module documentation, build contexts, Compose paths, image and
container names, scripts, generated-documentation inputs, and visible UI text.
Generated files are regenerated from sanitized sources where supported.

## Secret Handling

Real local environment files such as `.env`, `.env.local`, and `.env.deploy`
are not copied into tracked target content. Existing ignored target-local
configuration remains untracked and is not migration input.

Tracked examples use empty values or explicit non-secret placeholders for
passwords, JWT secrets, access keys, secret keys, API keys, and tokens. Runtime
secrets come from ignored environment files or injected environment variables.
Production-capable credential defaults are removed.

Inspection covers the source working tree, the resulting target tree and staged
diff, and all reachable target history. Reports name only the location and
credential type, never the value. Target history is rewritten only if a real
secret is verified. A backup reference is created first, and the changed hashes
and force-push requirement are documented.

## Git Strategy

The source repository remains read-only and its Git metadata is not imported.
Migration occurs on an isolated target branch. Commits are split into:

1. source functionality and mapped files;
2. generic naming and sanitized configuration;
3. verification and operational documentation.

Each commit stages only its own files. Existing target history is preserved.

## Verification

Fresh verification in the target repository includes:

- all Go tests in `video-service`;
- frontend tests and production build;
- RecBole and retained two-tower Python tests;
- deployment layout tests and Compose rendering for supported topologies;
- repository-wide naming and secret scans;
- Docker image builds and complete local-stack startup;
- health checks for the API, frontend, and represented dependencies.

Migration-caused failures are fixed before completion. If unavailable images,
credentials, hardware, or network access blocks a check, the exact command,
exit status, and dependency are recorded instead of reporting success.

## Rollback

Every stage is reviewed with `git diff` before commit. Focused commits allow a
stage to be reverted without discarding earlier target history. The untouched
source tree remains the recovery reference for missing behavior.

## Success Criteria

The migration is complete only when source functionality exists under mapped
target paths, target-only content and history remain intact, business-specific
names are absent from active tracked content, no real secret exists in tracked
content or reachable history, all verification has fresh results, and the
target ends with focused commits and a clean working tree. The design document
may retain source identifiers solely to document the migration mapping.
