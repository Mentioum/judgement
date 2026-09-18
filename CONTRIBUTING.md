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

## Provider scope

Keep the CLI small. `internal/cli/backend.go` is its provider boundary: evaluate
a request and list models. TypeSafe's HTTP protocol, retries, and response
checks stay in its client. The CLI owns input, JSON output, and batch ordering.
The boundary is tested with an alternate implementation; that fixture is not a
shipped integration.

When a second provider is actually needed, add its adapter, explicit provider
selection, credentials, and model defaults together. Document which question
types it supports. Providers may have different uncertainty semantics: never
fabricate calibrated probabilities or confidence to make an API fit. Reject
unsupported operations explicitly. Preserve provider-specific metadata.

Avoid plugin registries, automatic model routing, fallback chains, or a generic
HTTP framework until concrete use cases justify them. Existing CLI commands and
the public TypeSafe client should keep working when another adapter is added.

## Releases

Packaging uses GoReleaser OSS. Run `goreleaser check` and
`goreleaser release --snapshot --clean` to build all six archives locally without
publishing. CI checks this path and smoke-tests an extracted Linux binary.

Update the default `Version` in `internal/cli/cli.go` and the changelog, run CI,
then create a `v*` tag. GoReleaser injects the tag version into release binaries;
the source default keeps `go install` consistent. The release workflow tests the
tag and creates a draft with binaries and checksums. A maintainer publishes it
after review. See [the release guide](docs/releases.md) for readiness checks and
the package-manager plan.
