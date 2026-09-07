# AI Agent Guide

## Layout Profile

This is the Telegram Sphere layout. It combines Protobuf/Buf, generated HTTP
handlers, Ent, Wire, Swagger, a dashboard with admin authentication and local
file upload/download routes, and a Telegram Bot transport example. It
intentionally has no WeChat dependency.

## Ownership Rules

Read `.sphere/layout.json` before changing files. Paths are classified as
`layout_owned`, `mixed`, or `generated`; every unmatched path is
`project_owned`. Never edit generated files by hand. Treat mixed files as
integration seams and preserve both layout wiring and project additions.

Application contracts belong in `proto/<domain>/v1`, business logic in
`internal/biz/<domain>`, HTTP implementations in `internal/service/<domain>`,
and Ent schemas in `internal/pkg/database/schema`. Do not put product-specific
logic into layout-owned helpers.

See `docs/LAYOUT_CONTRACT.md` for the complete authoring and synchronization
protocol, including legacy-project adoption and conflict handling.

## Workflow

Use the Makefile as the workflow contract:

- `make gen/all` regenerates Ent, Proto, Swagger, Wire, and mapping outputs.
- `make test` runs the Go tests.
- `make lint` runs non-mutating Go and Buf checks.
- `make check` verifies dependency, formatting, lint, and test state.
- `make build` builds the local binary.

After changing Proto, schemas, constructors, or provider sets, run
`make gen/all` before tests. A completed change must pass `make check` and
`make build`, and tracked generated files must have no unexplained drift.

## Dashboard Authentication

Admin authentication lives in `internal/service/dash/auth.go`: password login
issues a JWT access token plus a refresh token backed by an `AdminSession`
row, and refresh rotates both. The response contract is snake_case
(`access_token`, `refresh_token`, `expires_at`) over `POST /api/auth/login`
and `POST /api/auth/refresh`; it is not pure-admin compatible. File upload
and download routes are registered on the dash server under `/files`.

Telegram contracts live in `proto/bot`, transport codecs in
`internal/server/bot`, and handlers in `internal/service/bot`. Bot startup must
remain part of the application task group so generated routes are reachable.
