package cli

import (
	"context"
	"encoding/json"
	"os"

	judgement "github.com/Mentioum/judgement"
)

// backend is the only provider boundary the CLI needs. Implementations handle
// their wire format, authentication, response validation, and retries. Keeping
// it here lets file handling and batches stay independent of the HTTP client.
type backend interface {
	EvaluateRaw(context.Context, judgement.Request) (json.RawMessage, error)
	ListModelsRaw(context.Context) (json.RawMessage, error)
}

type backendFactory func(options) (backend, error)

// TypeSafe is the only implemented provider. Add another adapter when there is
// a concrete API to support, rather than guessing a universal provider protocol.
func newBackend(o options) (backend, error) {
	return judgement.NewClient(judgement.Config{
		APIKey: os.Getenv("TYPESAFE_API_KEY"), BaseURL: o.base,
		Timeout: o.timeout, MaxRetries: o.retries,
	})
}
