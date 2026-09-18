# CLI contract

Flags follow the subcommand. `judgement --help` and `judgement COMMAND --help`
print human-readable usage. All other successful output is JSON or JSONL.

| Command | Input | stdout |
| --- | --- | --- |
| `evaluate` | JSON request on stdin or `--input` file | Complete API JSON response |
| `validate` | Same input as evaluate | `{ "valid": true, "request": ... }` with resolved model |
| `batch` | JSONL on stdin or `--input` file | Ordered result envelopes |
| `models` | None | Complete models response |
| `schema` | None | JSON Schema for requests |
| `describe` | None | Commands, help, schema, authentication, exit codes |
| `version` | None | Version object |

## Input

`--input -` reads stdin and is the default. A request must include `state` and a
nonempty `questions` object. Unknown request/question fields are rejected to
catch agent-generated typos. Nested JSON content remains unrestricted.

`--state-file PATH` loads the entire file as plain text and overrides the request
state. `--questions PATH` loads a reusable question map, requires `--state-file`,
and replaces the need for a request file. Either path may be `-`, but only one
source can consume stdin. For structured state, use a full JSON request.

`--model NAME` overrides the request's model. Without either value, the default
is `jev-latest`. Input files and JSONL lines are limited to 32 MiB; this is a
local memory bound, not the service's token limit. The service enforces context
and account limits.

## Network

| Setting | CLI default |
| --- | --- |
| API key | `TYPESAFE_API_KEY` environment variable |
| API root | `https://api.typesafe.ai/v1` |
| `--base-url` | Overrides `TYPESAFE_BASE_URL`; include `/v1` when appropriate |
| `--timeout` | `60s`, across all attempts and delays for each call |
| `--retries` | `2`, after the initial attempt; accepted range 0–10 |
| `--concurrency` | `4` batch workers; accepted range 1–64 |

Retries cover HTTP 408, 429, 500, 502, 503, 504, and 529. Exponential backoff uses
jitter. `Retry-After` seconds/dates and `retry-after-ms` are supported. If the
server delay will not fit in the remaining deadline, the HTTP error is returned
without retrying early. Transport errors are not automatically retried.

Custom endpoints receive your API key; use only trusted endpoints. HTTPS is
required except for loopback development servers. HTTP redirects are refused.
No key or request-body logs are emitted. Error messages omit remote response
bodies; status and available request ID support diagnosis with the provider.

## Errors and exit status

Single-call failures write one object to stderr:

```json
{"error":{"kind":"rate_limit","message":"TypeSafe API returned HTTP 429","retryable":true,"status":429}}
```

| Status | Meaning |
| --- | --- |
| 0 | Success |
| 1 | Batch completed with one or more failed records |
| 2 | Invalid input/configuration or HTTP 400/422 |
| 3 | HTTP 401/403 authentication/authorization failure |
| 4 | HTTP 429/529 rate limit/overload |
| 5 | Other API, transport, or output failure |
| 6 | Deadline exceeded |
| 130 | Cancellation/interruption |

A missing API key is a local configuration error (2). A key rejected by the
server is an authentication error (3). Batch record failures live in stdout,
use these same error objects, and produce aggregate exit 1; stream input/output
errors and interruption instead use the corresponding top-level exit status.

## Batch envelopes

```json
{"index":0,"id":"ticket-42","result":{"model":"jev-example","answers":{"refund":{"type":"noul","noul":0.98}},"usage":{"input_tokens":50,"output_tokens":5}}}
{"index":1,"id":"ticket-43","error":{"kind":"input","message":"state is required (use null for an explicitly empty state)","retryable":false}}
```

Indices are zero-based physical input line numbers, including malformed or empty
lines. IDs are optional JSON values and are passed through unchanged. The
command retains order within bounded concurrent windows; it does not load the
whole dataset into memory. `--pretty` is unavailable for batches so output stays
one JSON object per line. Batch does not support state/question override files.

## Compatibility

The initial CLI contract is version 0.1. Breaking interface changes will be
called out in the changelog. Pin a release in reproducible environments.
The CLI preserves response JSON fields; the Go typed client exposes known
fields, and its raw methods preserve future additions.
