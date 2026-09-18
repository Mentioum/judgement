package judgement

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRejectIncompleteEvaluations(t *testing.T) {
	cases := map[string]string{
		"empty":               `{}`,
		"missing answer":      `{"model":"test","answers":{},"usage":{"input_tokens":0,"output_tokens":0}}`,
		"wrong answer type":   `{"model":"test","answers":{"ok":{"type":"choice","choice":"yes"}},"usage":{"input_tokens":0,"output_tokens":0}}`,
		"missing probability": `{"model":"test","answers":{"ok":{"type":"noul"}},"usage":{"input_tokens":0,"output_tokens":0}}`,
		"invalid probability": `{"model":"test","answers":{"ok":{"type":"noul","noul":2}},"usage":{"input_tokens":0,"output_tokens":0}}`,
		"missing usage":       `{"model":"test","answers":{"ok":{"type":"noul","noul":0}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer server.Close()
			client, _ := NewClient(Config{APIKey: "test", BaseURL: server.URL})
			request := Request{State: "hello", Questions: map[string]Question{"ok": Noul("Is this a greeting?")}}
			if _, err := client.EvaluateRaw(context.Background(), request); err == nil {
				t.Fatal("raw evaluation accepted invalid response")
			}
			if _, err := client.Evaluate(context.Background(), request); err == nil {
				t.Fatal("typed evaluation accepted invalid response")
			}
		})
	}
}

func TestModelsRequireList(t *testing.T) {
	for _, body := range []string{`{}`, `{"models":null}`, `{"models":[{}]}`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		client, _ := NewClient(Config{APIKey: "test", BaseURL: server.URL})
		_, err := client.ListModelsRaw(context.Background())
		server.Close()
		if err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
