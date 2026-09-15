# Asana Extractor

A small Go application for extracting users and projects from one Asana workspace and persisting the returned JSON objects as individual files. It uses the Go standard library and is intentionally sized for a short technical assessment.

## Implemented Requirements

- Personal Access Token authentication.
- User and project extraction for one hardcoded workspace.
- Sequential pagination for users and projects.
- Bounded HTTP 429 retries using `Retry-After`.
- One JSON file per user and project.
- Overwriting of existing files with the same GID.
- `30s` and `5m` polling modes.
- Context cancellation and Ctrl+C shutdown.
- Deterministic unit and integration-style tests.

## Prerequisites

- Go 1.22 or newer.
- An Asana account and access to the target workspace.
- An Asana Personal Access Token.

The test suite uses local HTTP test servers and does not require live Asana access.

## Asana Setup

1. Create or use an Asana workspace.
2. Ensure the PAT user can access the workspace, users, and projects.
3. Create a Personal Access Token in Asana.
4. Keep the PAT outside the repository and provide it through the environment.

The application targets workspace GID `1218492064871302`, defined in `config.go`. The workspace GID is not treated as a secret.

## Configuration

Set the PAT in the environment:

```powershell
$env:ASANA_PAT = "<personal-access-token>"
```

The application reads the polling mode from `ASANA_POLL_INTERVAL`. If it is unset, the default is `30s`.

Supported values are exactly:

- `30s`
- `5m`

## Run

From the `AsanaTestProject` directory:

```powershell
$env:ASANA_POLL_INTERVAL = "30s"
go run .
```

For the five-minute mode:

```powershell
$env:ASANA_POLL_INTERVAL = "5m"
go run .
```

The first extraction runs immediately. Press Ctrl+C to cancel the current context and stop cleanly. The application prints only extracted counts and the output directory; it never prints the PAT.

## Tests and Checks

```powershell
go test ./...
go vet ./...
go build ./...
```

## Output

Paths are relative to the process working directory:

```text
output/
  users/
    <gid>.json
  projects/
    <gid>.json
```

Each file contains the original `json.RawMessage` entity received from the selected Asana list endpoint. This preserves unknown fields without reducing the entity to a typed model. Saving the same GID again overwrites its existing file.

## Architecture

- `config.go`: reads `ASANA_PAT`, the hardcoded workspace GID, and polling mode.
- `asana.go`: authenticated HTTP client, users/projects extraction, pagination, and 429 handling.
- `storage.go`: output directory preparation and per-entity JSON files.
- `polling.go`: supported interval parsing and sequential polling lifecycle.
- `main.go`: creates the reusable HTTP/Asana client and storage, runs extraction cycles, and handles Ctrl+C.

One `http.Client` and one Asana client are created before polling begins and reused across cycles. Extraction is sequential: users and projects are fetched page by page, and polling never starts a new cycle until the previous cycle has completed.

## Pagination

Requests use `limit=100`. When Asana returns `next_page.offset`, the exact opaque offset is sent on the next request. Offsets are never calculated or modified. Extraction stops when `next_page` is `null`. A non-null `next_page` without an offset is treated as an error.

## HTTP 429 Handling

Only HTTP 429 responses are retried. The client reads `Retry-After` as seconds, waits for that duration, and retries the same request with the same path and query parameters. Retries are bounded by `Client.MaxRetries`; the default is three retries after the initial attempt. An injected sleep function is used by tests, while production uses a context-aware timer.

Other HTTP errors, including 5xx responses, are returned without retrying. Proactive rate limiting is not implemented.

## Scalability Decisions

- Pages are limited to 100 records.
- Pagination is sequential intentionally.
- Entities are passed to storage as each page is processed instead of accumulating the full workspace in memory.
- No worker pool, goroutines, or channels are used for extraction.
- A single reusable HTTP client preserves connection reuse.

## Important Design Decisions

- Standard library only; no Asana SDK or framework.
- PAT authentication uses `Authorization: Bearer <token>`.
- The PAT is read only from `ASANA_PAT` and is not stored in source, tests, output, or logs.
- The list endpoint payload is preserved as `json.RawMessage`; a small GID view is used only to choose the filename.
- Existing entity files are overwritten on every later cycle.
- Only one polling interval is active per process.

## Known Limitations and Production Improvements

- The workspace GID is hardcoded for this assessment.
- The list endpoints return the object shape selected by Asana, commonly compact objects; the application preserves that returned payload but does not make additional detail requests.
- Files for entities removed from Asana are not deleted, so stale files may remain.
- Only reactive 429 handling is implemented; there is no proactive rate limiter.
- 5xx and other non-429 responses are not retried.
- Production improvements could include secret management, structured logging, stale-file reconciliation, metrics, and a more complete synchronization strategy.
- OAuth is not implemented because this assessment requires a PAT.
