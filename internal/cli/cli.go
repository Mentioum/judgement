// Package cli implements the script-oriented judgement command.
package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	judgement "github.com/Mentioum/judgement"
)

const Version = "0.1.1"
const help = `judgement — Jev decisions for agents and scripts

Commands:
  evaluate   Evaluate one JSON request (stdin by default)
  batch      Evaluate JSONL requests, one result per line
  validate   Validate one request locally; no API key or network needed
  models     List models available to your account
  schema     Print the request JSON Schema
  describe   Print machine-readable command and exit-code information
  version    Print version as JSON

Flags (after command):
  --input PATH         Request file, or - for stdin (default -)
  --state-file PATH    Override state with a UTF-8 text file (- for stdin)
  --questions PATH     Questions JSON file; combine with --state-file
  --model NAME         Override model (default jev-latest when omitted)
  --timeout DURATION   Total budget per API call, including retries (default 60s)
  --retries N          Retries after initial attempt (default 2; 0 disables)
  --base-url URL       API root including /v1 (default official TypeSafe endpoint)
  --concurrency N      Concurrent batch workers (default 4; max 64)
  --pretty            Indent JSON (single-result commands only)

Authentication: TYPESAFE_API_KEY environment variable. No interactive prompts.
stdout: JSON / JSONL only, except explicit --help. stderr: JSON error objects.
Batch: each line is a request, or {"id":...,"request":{...}}. Results preserve
input order and contain index (zero-based), optional id, and result or error.
Batch continues after individual errors and exits 1 if any record failed.
Exit codes: 0 success, 1 batch failures, 2 input/configuration, 3 authentication,
4 rate limit/overload, 5 other API/transport, 6 deadline, 130 interrupted.
`

type failure struct {
	Kind      string `json:"kind"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
	Status    int    `json:"status,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}
type options struct {
	input, state, questions, model, base string
	timeout                              time.Duration
	retries, concurrency                 int
	pretty                               bool
}

func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	return runWithBackend(ctx, args, in, out, errOut, newBackend)
}

func runWithBackend(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, connect backendFactory) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, help)
		return 0
	}
	command := args[0]
	switch command {
	case "evaluate", "validate", "batch", "models", "schema", "describe", "version":
	default:
		return fail(errOut, "input", errors.New("unknown command; run judgement --help"), 2)
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	o := options{}
	fs.StringVar(&o.input, "input", "-", "")
	fs.StringVar(&o.state, "state-file", "", "")
	fs.StringVar(&o.questions, "questions", "", "")
	fs.StringVar(&o.model, "model", "", "")
	fs.StringVar(&o.base, "base-url", os.Getenv("TYPESAFE_BASE_URL"), "")
	fs.DurationVar(&o.timeout, "timeout", 60*time.Second, "")
	fs.IntVar(&o.retries, "retries", 2, "")
	fs.IntVar(&o.concurrency, "concurrency", 4, "")
	fs.BoolVar(&o.pretty, "pretty", false, "")
	if err := fs.Parse(args[1:]); errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(out, help)
		return 0
	} else if err != nil {
		return fail(errOut, "input", err, 2)
	}
	if fs.NArg() != 0 {
		return fail(errOut, "input", errors.New("unexpected positional arguments"), 2)
	}
	allowed := map[string]bool{"pretty": command != "batch"}
	if command == "evaluate" || command == "validate" || command == "batch" {
		allowed["input"], allowed["model"] = true, true
	}
	if command == "evaluate" || command == "validate" {
		allowed["state-file"], allowed["questions"] = true, true
	}
	if command == "evaluate" || command == "models" || command == "batch" {
		allowed["base-url"], allowed["timeout"], allowed["retries"] = true, true, true
	}
	allowed["concurrency"] = command == "batch"
	var flagErr error
	fs.Visit(func(f *flag.Flag) {
		if !allowed[f.Name] {
			flagErr = fmt.Errorf("--%s is not supported by %s", f.Name, command)
		}
	})
	if flagErr != nil {
		return fail(errOut, "input", flagErr, 2)
	}
	if o.timeout <= 0 || o.retries < 0 || o.retries > 10 || o.concurrency < 1 || o.concurrency > 64 {
		return fail(errOut, "input", errors.New("invalid timeout, retries, or concurrency"), 2)
	}
	emit := func(v any) int {
		if err := writeJSON(out, v, o.pretty); err != nil {
			return fail(errOut, "output", err, 5)
		}
		return 0
	}
	switch command {
	case "version":
		return emit(map[string]string{"version": Version})
	case "schema":
		return emit(json.RawMessage(requestSchema))
	case "describe":
		return emit(map[string]any{"name": "judgement", "version": Version, "commands": []string{"evaluate", "batch", "validate", "models", "schema", "describe", "version"}, "help": help, "request_schema": json.RawMessage(requestSchema), "authentication": map[string]string{"environment": "TYPESAFE_API_KEY"}, "exit_codes": map[string]string{"0": "success", "1": "batch contains failures", "2": "input or configuration", "3": "authentication", "4": "rate limit or overload", "5": "API, transport, or output", "6": "deadline exceeded", "130": "interrupted"}})
	}
	if command == "batch" && (o.pretty || o.state != "" || o.questions != "") {
		return fail(errOut, "input", errors.New("batch does not accept --pretty, --state-file, or --questions"), 2)
	}
	var r judgement.Request
	if command == "evaluate" || command == "validate" {
		var err error
		r, err = loadRequest(o, in)
		if ctx.Err() != nil {
			f, code := classify(ctx.Err())
			_ = writeJSON(errOut, map[string]any{"error": f}, false)
			return code
		}
		if err != nil {
			return fail(errOut, "input", err, 2)
		}
		if command == "validate" {
			return emit(map[string]any{"valid": true, "request": r})
		}
	}
	client, err := connect(o)
	if err != nil {
		return fail(errOut, "configuration", err, 2)
	}
	if command == "batch" {
		return batch(ctx, client, o, in, out, errOut)
	}
	var result json.RawMessage
	if command == "models" {
		result, err = client.ListModelsRaw(ctx)
	} else {
		result, err = client.EvaluateRaw(ctx, r)
	}
	if err != nil {
		f, code := classify(err)
		_ = writeJSON(errOut, map[string]any{"error": f}, false)
		return code
	}
	return emit(result)
}

func loadRequest(o options, in io.Reader) (judgement.Request, error) {
	var r judgement.Request
	if o.questions != "" {
		if o.state == "" || o.input != "-" {
			return r, errors.New("--questions requires --state-file and cannot be combined with --input")
		}
		if o.questions == "-" && o.state == "-" {
			return r, errors.New("only one input can consume stdin")
		}
		b, err := readFile(o.questions, in)
		if err != nil {
			return r, err
		}
		if err = decode(b, &r.Questions); err != nil {
			return r, fmt.Errorf("questions: %w", err)
		}
	} else {
		if o.input == "-" && o.state == "-" {
			return r, errors.New("request and state cannot both consume stdin")
		}
		b, err := readFile(o.input, in)
		if err != nil {
			return r, err
		}
		if err = decode(b, &r); err != nil {
			return r, err
		}
	}
	if o.state != "" {
		b, err := readFile(o.state, in)
		if err != nil {
			return r, err
		}
		r.State = string(b)
	}
	if o.model != "" {
		r.Model = o.model
	}
	if r.Model == "" {
		r.Model = judgement.DefaultModel
	}
	return r, r.Validate()
}
func decode(b []byte, v any) error {
	if _, ok := v.(*judgement.Request); ok {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(b, &fields); err != nil {
			return errors.New("request must be a JSON object")
		}
		if _, present := fields["state"]; !present {
			return errors.New("state is required (use null for an explicitly empty state)")
		}
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fmt.Errorf("invalid request JSON: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("expected exactly one JSON value")
	}
	return nil
}
func readFile(path string, in io.Reader) ([]byte, error) {
	var r io.Reader = in
	if path != "-" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}
	b, err := io.ReadAll(io.LimitReader(r, 32*1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(b) > 32*1024*1024 {
		return nil, errors.New("input exceeded 32 MiB")
	}
	return b, nil
}
func writeJSON(w io.Writer, v any, pretty bool) error {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	if pretty {
		e.SetIndent("", "  ")
	}
	return e.Encode(v)
}
func fail(w io.Writer, kind string, err error, code int) int {
	_ = writeJSON(w, map[string]any{"error": failure{Kind: kind, Message: err.Error()}}, false)
	return code
}
func classify(err error) (failure, int) {
	f := failure{Kind: "api", Message: err.Error()}
	code := 5
	var ae *judgement.APIError
	switch {
	case errors.Is(err, context.Canceled):
		f.Kind = "interrupted"
		code = 130
	case errors.Is(err, context.DeadlineExceeded):
		f.Kind = "timeout"
		code = 6
	case errors.As(err, &ae):
		f.Status = ae.StatusCode
		f.RequestID = ae.RequestID
		f.Retryable = ae.Retryable
		switch ae.StatusCode {
		case 401, 403:
			f.Kind = "authentication"
			code = 3
		case 429, 529:
			f.Kind = "rate_limit"
			code = 4
		case 400, 422:
			f.Kind = "request"
			code = 2
		}
	}
	return f, code
}

type batchResult struct {
	Index  int             `json:"index"`
	ID     json.RawMessage `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *failure        `json:"error,omitempty"`
}

// Bounded windows limit memory and in-flight work while preserving input order.
func batch(ctx context.Context, client backend, o options, in io.Reader, out, errOut io.Writer) int {
	var input io.Reader = in
	if o.input != "-" {
		f, err := os.Open(o.input)
		if err != nil {
			return fail(errOut, "input", err, 2)
		}
		defer f.Close()
		input = f
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 32*1024*1024)
	index, code := 0, 0
	for {
		lines := make([][]byte, 0, o.concurrency)
		for len(lines) < o.concurrency && scanner.Scan() {
			lines = append(lines, bytes.Clone(scanner.Bytes()))
		}
		if ctx.Err() != nil {
			f, c := classify(ctx.Err())
			_ = writeJSON(errOut, map[string]any{"error": f}, false)
			return c
		}
		if len(lines) == 0 {
			break
		}
		results := make([]batchResult, len(lines))
		var wg sync.WaitGroup
		for i, line := range lines {
			wg.Add(1)
			go func(i int, line []byte) {
				defer wg.Done()
				result := batchResult{Index: index + i}
				var envelope map[string]json.RawMessage
				var r judgement.Request
				var err error
				if e := json.Unmarshal(line, &envelope); e != nil {
					err = errors.New("invalid JSONL record")
				} else if req, ok := envelope["request"]; ok {
					result.ID = envelope["id"]
					for key := range envelope {
						if key != "id" && key != "request" {
							err = errors.New("batch envelope accepts only id and request")
						}
					}
					if err == nil {
						err = decode(req, &r)
					}
				} else {
					err = decode(line, &r)
				}
				if o.model != "" {
					r.Model = o.model
				}
				if err == nil {
					err = r.Validate()
				}
				if err != nil {
					result.Error = &failure{Kind: "input", Message: err.Error()}
				} else {
					result.Result, err = client.EvaluateRaw(ctx, r)
					if err != nil {
						f, _ := classify(err)
						result.Error = &f
					}
				}
				results[i] = result
			}(i, line)
		}
		wg.Wait()
		for _, result := range results {
			if result.Error != nil {
				code = 1
			}
			if err := writeJSON(out, result, false); err != nil {
				return fail(errOut, "output", err, 5)
			}
		}
		index += len(lines)
		if ctx.Err() != nil {
			f, c := classify(ctx.Err())
			_ = writeJSON(errOut, map[string]any{"error": f}, false)
			return c
		}
	}
	if err := scanner.Err(); err != nil {
		return fail(errOut, "input", errors.New("could not read JSONL input (maximum line size is 32 MiB)"), 2)
	}
	return code
}
