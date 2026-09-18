# Changelog

## 0.1.1 — 2026-09-18

* Reject incomplete TypeSafe evaluations and model lists before reporting success.
* Allow interruption while the CLI waits for stdin.
* Isolate CLI provider calls behind a two-method internal adapter. TypeSafe
  remains the only implemented provider; command syntax is unchanged.

## 0.1.0 — 2026-09-18

Initial release:

* Go library for Jev evaluation and model discovery.
* Noul, Choice, Score, and structured JSON instructions/criteria.
* JSON-first CLI with offline validation, schema, and interface discovery.
* Ordered, bounded concurrent JSONL batches with per-record errors.
* Context cancellation, total call deadlines, retry delays, and sanitized errors.
* Agent workflow guide, Go/Python examples, CI, and release packaging.

Validated against documented contracts with local HTTP fixtures. Live TypeSafe
evaluation has not yet been verified with an API key.
