package judgement

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func sampleRequest() Request {
	return Request{State: map[string]any{"ticket": "Please refund the duplicate charge"}, Questions: map[string]Question{
		"refund":  Noul("Is a refund requested?"),
		"team":    Choice(map[string]any{"task": "Choose the team"}, map[string]any{"billing": map[string]any{"scope": []string{"refunds", "payments"}}, "support": nil}),
		"urgency": Score(nil, []any{"can wait", map[string]any{"deadline": "today"}}),
	}}
}

func TestEvaluateWireAndTypedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/systemone" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var got Request
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		if got.Model != DefaultModel || got.Questions["team"].Type != "choice" {
			t.Errorf("incorrect request: %+v", got)
		}
		_, _ = w.Write([]byte(`{"model":"jev-test","answers":{"refund":{"type":"noul","noul":0},"team":{"type":"choice","choice":"billing","probabilities":{"billing":0.9,"support":0.1},"confidence":0.7},"urgency":{"type":"score","score":0,"legend":{"0":{"deadline":"today"}},"probabilities":{"0":1,"1":0},"confidence":1}},"usage":{"input_tokens":120,"output_tokens":5},"future_field":true}`))
	}))
	defer server.Close()
	client, err := NewClient(Config{APIKey: "test-key", BaseURL: server.URL + "/v1"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Evaluate(context.Background(), sampleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if result.Answers["refund"].Noul == nil || *result.Answers["refund"].Noul != 0 || result.Answers["urgency"].Score == nil || result.Usage.InputTokens != 120 {
		t.Fatalf("lost response fields: %+v", result)
	}
	raw, err := client.EvaluateRaw(context.Background(), sampleRequest())
	if err != nil || !strings.Contains(string(raw), "future_field") {
		t.Fatalf("raw response not preserved: %s %v", raw, err)
	}
}

func TestRetriesAndSanitizedErrors(t *testing.T) {
	for _, status := range []int{429, 529, 503, 401, 422} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var count atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				count.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"detail":"secret-key private-state"}`))
			}))
			defer server.Close()
			client, _ := NewClient(Config{APIKey: "secret-key", BaseURL: server.URL, MaxRetries: 2})
			_, err := client.Evaluate(context.Background(), sampleRequest())
			var ae *APIError
			if !errors.As(err, &ae) || ae.StatusCode != status {
				t.Fatalf("wrong error: %v", err)
			}
			want := int32(3)
			if status == 401 || status == 422 {
				want = 1
			}
			if count.Load() != want {
				t.Errorf("attempts %d, want %d", count.Load(), want)
			}
			if strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "private-state") {
				t.Fatal("sensitive body leaked")
			}
		})
	}
}

func TestCancellationDuringBackoff(t *testing.T) {
	called := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(429)
		close(called)
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "test", BaseURL: server.URL, MaxRetries: 2})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-called; cancel() }()
	_, err := client.Evaluate(ctx, sampleRequest())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
}
func TestDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "test", BaseURL: server.URL, Timeout: 20 * time.Millisecond})
	_, err := client.ListModels(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout: %v", err)
	}
}
func TestModelsAndRedirect(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { targetCalls.Add(1) }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			_, _ = w.Write([]byte(`{"models":[{"name":"jev-latest","description":"Latest","release_date":"2026-09-15"}]}`))
			return
		}
		http.Redirect(w, r, target.URL, 307)
	}))
	defer server.Close()
	client, _ := NewClient(Config{APIKey: "test", BaseURL: server.URL + "/v1"})
	models, err := client.ListModels(context.Background())
	if err != nil || len(models.Models) != 1 {
		t.Fatalf("models: %+v %v", models, err)
	}
	_, err = client.Evaluate(context.Background(), sampleRequest())
	if err == nil || targetCalls.Load() != 0 {
		t.Fatal("redirect followed")
	}
}
func TestValidate(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Request)
	}{
		{"numeric state", func(r *Request) { r.State = 12 }},
		{"empty questions", func(r *Request) { r.Questions = nil }},
		{"invalid type", func(r *Request) { r.Questions["x"] = Question{Type: "text"} }},
		{"one score level", func(r *Request) { r.Questions["x"] = Score("x", []any{"one"}) }},
		{"invalid description", func(r *Request) { r.Questions["x"] = Choice("x", map[string]any{"a": 42, "b": nil}) }},
		{"invalid noul key", func(r *Request) { q := Noul("x"); q.Criteria = map[string]any{"yes": "yes"}; r.Questions["x"] = q }},
	}
	if err := sampleRequest().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			r := sampleRequest()
			tt.mutate(&r)
			if r.Validate() == nil {
				t.Fatal("invalid request accepted")
			}
		})
	}
}
func TestRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "2")
	if retryDelay(h, 0) != 2*time.Second {
		t.Fatal("seconds ignored")
	}
	h.Set("retry-after-ms", "30")
	if retryDelay(h, 0) != 30*time.Millisecond {
		t.Fatal("milliseconds ignored")
	}
	h.Del("retry-after-ms")
	h.Set("Retry-After", time.Now().Add(5*time.Second).UTC().Format(http.TimeFormat))
	d := retryDelay(h, 0)
	if d < 3*time.Second || d > 5*time.Second {
		t.Fatalf("date ignored: %v", d)
	}
}

func TestRejectNonObjectResponse(t *testing.T) {
	for _, body := range []string{"null", "[]", "invalid"} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			client, _ := NewClient(Config{APIKey: "test", BaseURL: server.URL})
			if _, err := client.ListModelsRaw(context.Background()); err == nil {
				t.Fatal("invalid API response accepted")
			}
		})
	}
}
