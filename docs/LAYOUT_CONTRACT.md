# Sphere Layout Authoring and Synchronization Contract

## Purpose

Sphere layouts are executable examples and project starters, not application
frameworks. They must remain easy to understand, regenerate, and update after
a generated project accumulates business code.

The standard, Bun, simple, and Telegram layouts share this contract while
exposing different capabilities. Optional capabilities do not require empty
placeholder directories.

## Stable Structure

- `proto/<domain>/v1`: handwritten API contracts and validation.
- `api/**`: generated Go contracts and transport bindings.
- `internal/biz/<domain>`: business use cases and background tasks.
- `internal/service/<domain>`: implementations of generated interfaces.
- `internal/server/<transport>`: transport construction and route registration.
- `internal/config`: configuration loading and provider fields.
- `internal/pkg`: project-local adapters and infrastructure.
- `cmd/app`: application assembly; Wire output is generated.
- `cmd/tools`: project-local generation or documentation tools.
- `swagger/**`, Ent clients, mapping helpers, and Wire output: generated code.

Proto is the API source of truth. Constructor or provider changes must be
followed by Wire generation. Ent schema changes must be followed by Ent, Proto,
mapping, and documentation generation. Make targets are the public development
workflow and must not silently point at missing scripts.

## Ownership Model

Each layout contains `.sphere/layout.json`. Its path patterns are evaluated in
this order: `generated`, `layout_owned`, `mixed`, then the default
`project_owned` classification. Patterns must not overlap.

- `layout_owned`: stable tooling, CI, generators, and reusable adapters. Apply
  upstream changes when the project copy still equals the recorded base;
  otherwise perform a three-way merge.
- `mixed`: application assembly and built-in feature seams. Always merge the
  base, local, and target versions semantically.
- `project_owned`: business contracts and implementation. Never replace these
  merely because the upstream layout changed.
- `generated`: ignore upstream file contents and regenerate them from the
  merged handwritten sources.

New product code must use project-owned domain paths. A helper that is useful
across unrelated projects and has no project-module imports belongs in a
versioned go-sphere library; it should not be copied into every layout.

## Lock File

`sphere-cli create` writes `.sphere/layout.lock.json` in generated projects:

```json
{
  "schema_version": 1,
    "name": "telegram",
    "repository": "https://github.com/go-sphere/sphere-telegram-layout.git",
  "ref": "master",
    "upstream_module": "github.com/go-sphere/sphere-telegram-layout",
  "base_revision": "full-git-commit-sha"
}
```

Unknown JSON fields are forward-compatible. An agent must stop on an unknown
`schema_version`. Layout source repositories do not contain a lock file.

## AI-Assisted Update Procedure

1. Read the contract and lock. Resolve the requested target revision; when no
   revision is specified, resolve the current commit of the recorded ref.
2. Materialize the base and target upstream commits in temporary directories.
   Do not run their code merely to calculate a diff.
3. Rewrite both snapshots from `upstream_module` to the project's current Go
   module before comparing, so import renaming is not mistaken for a change.
4. Ignore generated paths. Leave project-owned paths unchanged. Apply
   layout-owned paths directly only when local equals base; otherwise use a
   three-way merge. Always use a semantic three-way merge for mixed paths.
5. If a file is both locally and upstream deleted or renamed, resolve ownership
   before recreating it. Never resolve a conflict by discarding project code.
6. Regenerate through the layout Makefile, then run formatting, dependency
   checks, tests, lint, and build. Review the complete diff.
7. Update `base_revision` only after all conflicts are resolved and verification
   succeeds. If blocked, leave the lock unchanged and report the affected paths.

## Adopting a Legacy Project

When no lock exists, inspect the project's earliest template commit and compare
its Git tree with commits from the likely upstream layout. Record a revision
only when exactly one upstream tree matches. If there is no unique match, ask
the user for the originating revision; do not infer a convenient base. Once the
base is confirmed, create the version-1 lock and follow the normal update flow.

## Layout Release Checklist

- Keep README capabilities and `make help` output accurate.
- Keep `.sphere/layout.json` ownership patterns non-overlapping.
- Regenerate all derived outputs after schema, Proto, or Wire changes.
- Run `make check` and `make build` from a clean checkout.
- Verify provider-specific dependencies exist only in provider-specific layouts.
- Document breaking template changes; never apply database deletion migrations
  automatically to downstream applications.
