# Contributing

Bug reports and focused pull requests are welcome. Include the command, version,
expected behavior, and a minimal reproduction with synthetic data. Never include
API keys or private request content.

## Local checks

Requires Go 1.23 or newer. No external modules or API key are needed for tests.

```sh
gofmt -w .
go test -race ./...
go vet ./...
go build ./...
```

Tests bind loopback ports. Restricted sandboxes may require permission to do so.
Use local HTTP fixtures for API changes. Keep stdout machine-readable, retain
JSON number precision, preserve batch order, and cover failure paths when
changing transport or concurrency. Update the CLI help, schema, and docs together.

The root package is the reusable client. `internal/cli` owns the CLI contract;
`cmd/judgement` handles process signals. Examples should compile and remain safe
to run on synthetic data. Before adding dependencies, explain why the standard
library is insufficient.

## Releases

Update the CLI version and changelog, run CI, then create a `v*` tag. The release
workflow tests the tag and packages binaries for Linux, macOS, and Windows with
checksums. A maintainer publishes the resulting draft release after review.
