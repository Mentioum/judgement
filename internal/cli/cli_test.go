package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const validRequest = `{"state":"hello","questions":{"ok":{"type":"noul","instructions":"Is this a greeting?"}}}`

func run(t *testing.T, args []string, input string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	c := Run(context.Background(), args, strings.NewReader(input), &out, &errOut)
	return c, out.String(), errOut.String()
}

func TestOfflineCommands(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	for _, cmd := range []string{"schema", "describe", "version", "validate"} {
		code, out, err := run(t, []string{cmd}, validRequest)
		if code != 0 || !json.Valid([]byte(out)) || err != "" {
			t.Fatalf("%s: %d %s %s", cmd, code, out, err)
		}
	}
}
func TestRejectMalformedInput(t *testing.T) {
	for _, input := range []string{validRequest + " {}", `{"state":null,"questions":{}}`, `{"state":"x","questions":{"ok":{"type":"noul","instruction":"typo"}}}`} {
		c, out, err := run(t, []string{"validate"}, input)
		if c != 2 || out != "" || !json.Valid([]byte(err)) {
			t.Fatalf("bad validation: %d %s %s", c, out, err)
		}
	}
}
func TestEvaluatePreservesNumbersAndResponse(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body bytes.Buffer
		_, _ = body.ReadFrom(r.Body)
		if !strings.Contains(body.String(), "9007199254740993") {
			t.Error("large integer precision lost")
		}
		_, _ = w.Write([]byte(`{"answers":{"ok":{"type":"noul","noul":0}},"future":9007199254740993}`))
	}))
	defer server.Close()
	c, out, err := run(t, []string{"evaluate", "--base-url", server.URL}, strings.Replace(validRequest, `"hello"`, `{"id":9007199254740993}`, 1))
	if c != 0 || !strings.Contains(out, "9007199254740993") || err != "" {
		t.Fatalf("%d %s %s", c, out, err)
	}
}
func TestBatchOrderFailureAndBoundedConcurrency(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "test")
	var active, maxActive, calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maxActive.Load(); n > old; old = maxActive.Load() {
			if maxActive.CompareAndSwap(old, n) {
				break
			}
		}
		calls.Add(1)
		time.Sleep(10 * time.Millisecond)
		_, _ = w.Write([]byte(`{"answers":{},"model":"test","usage":{}}`))
	}))
	defer server.Close()
	input := `{"id":"first","request":` + validRequest + "}\ninvalid\n" + validRequest + "\n" + validRequest + "\n"
	c, out, err := run(t, []string{"batch", "--base-url", server.URL, "--concurrency", "3"}, input)
	if c != 1 || err != "" || calls.Load() != 3 || maxActive.Load() > 3 {
		t.Fatalf("bad batch: %d %s %s", c, out, err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Fatalf("wrong result count: %s", out)
	}
	for i, line := range lines {
		var r batchResult
		if e := json.Unmarshal([]byte(line), &r); e != nil {
			t.Fatal(e)
		}
		if r.Index != i {
			t.Fatal("out of order")
		}
		if (i == 1) != (r.Error != nil) {
			t.Fatal("incorrect failure record")
		}
		if i == 0 && string(r.ID) != `"first"` {
			t.Fatal("id lost")
		}
	}
}
func TestAuthenticationExitAndRedaction(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "private-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"private-key"}`))
	}))
	defer server.Close()
	c, out, err := run(t, []string{"evaluate", "--base-url", server.URL}, validRequest)
	if c != 3 || out != "" || strings.Contains(err, "private-key") || !strings.Contains(err, `"authentication"`) {
		t.Fatalf("%d %s %s", c, out, err)
	}
}
func TestStateAndQuestionsFiles(t *testing.T) {
	t.Setenv("TYPESAFE_API_KEY", "")
	c, _, _ := run(t, []string{"validate", "--state-file", "-", "--questions", "-"}, validRequest)
	if c != 2 {
		t.Fatal("two stdin consumers accepted")
	}
	path := filepath.Join(t.TempDir(), "questions.json")
	if err := os.WriteFile(path, []byte(`{"ok":{"type":"noul","instructions":{"task":"Is this a greeting?"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, out, err := run(t, []string{"validate", "--state-file", "-", "--questions", path}, "hello\nworld")
	if c != 0 || err != "" || !strings.Contains(out, `hello\nworld`) {
		t.Fatalf("%d %s %s", c, out, err)
	}
}

func TestRejectIgnoredFlagsAndMissingState(t *testing.T) {
	for _, args := range [][]string{{"models", "--input", "file.json"}, {"evaluate", "--concurrency", "2"}, {"batch", "--pretty"}} {
		c, _, _ := run(t, args, validRequest)
		if c != 2 {
			t.Fatalf("ignored flags: %v", args)
		}
	}
	c, _, _ := run(t, []string{"validate"}, `{"questions":{"ok":{"type":"noul"}}}`)
	if c != 2 {
		t.Fatal("missing state accepted")
	}
	c, _, _ = run(t, []string{"validate"}, `{"state":null,"questions":{"ok":{"type":"noul"}}}`)
	if c != 0 {
		t.Fatal("explicit null state rejected")
	}
}
