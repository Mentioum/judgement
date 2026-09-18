// Package judgement provides a Go client for TypeSafe AI's System One API.
package judgement

import (
	"bytes"
	"encoding/json"
	"fmt"
)

const DefaultModel = "jev-latest"

// Question supports Noul, Choice and Score. Instructions and descriptions accept
// strings, objects, arrays or nil, including the API's structured rubrics.
type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

func Noul(instructions any) Question { return Question{Type: "noul", Instructions: instructions} }
func Choice(instructions any, options map[string]any) Question {
	return Question{Type: "choice", Instructions: instructions, Criteria: options}
}
func Score(instructions any, levels []any) Question {
	return Question{Type: "score", Instructions: instructions, Criteria: levels}
}

type Request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

// Answer preserves zero probabilities and scores using pointers. Legend values
// may be structured JSON when the request uses structured score criteria.
type Answer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        *string            `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]any     `json:"legend,omitempty"`
}
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}
type Response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}
type Model struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
}
type ModelsResponse struct {
	Models []Model `json:"models"`
}

// Validate checks documented request shapes without restricting nested JSON.
func (r Request) Validate() error {
	b, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("request is not JSON serializable: %w", err)
	}
	var v struct {
		State     json.RawMessage `json:"state"`
		Questions map[string]struct {
			Type         string          `json:"type"`
			Instructions json.RawMessage `json:"instructions"`
			Criteria     json.RawMessage `json:"criteria"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	if !content(v.State, true) {
		return fmt.Errorf("state must be a string, object, array, or null")
	}
	if len(v.Questions) == 0 {
		return fmt.Errorf("questions must contain at least one question")
	}
	for name, q := range v.Questions {
		if !content(q.Instructions, true) {
			return fmt.Errorf("question %q: instructions must be string, object, array, or null", name)
		}
		switch q.Type {
		case "noul", "choice":
			if q.Type == "noul" && (len(q.Criteria) == 0 || bytes.Equal(q.Criteria, []byte("null"))) {
				continue
			}
			var m map[string]json.RawMessage
			if err := json.Unmarshal(q.Criteria, &m); err != nil || m == nil {
				return fmt.Errorf("question %q: criteria must be an object", name)
			}
			if q.Type == "choice" && len(m) == 0 {
				return fmt.Errorf("question %q: choice requires at least one option", name)
			}
			for k, value := range m {
				if q.Type == "noul" && k != "true" && k != "false" {
					return fmt.Errorf("question %q: noul criteria keys must be true or false", name)
				}
				if !content(value, true) {
					return fmt.Errorf("question %q: invalid description for %q", name, k)
				}
			}
		case "score":
			var levels []json.RawMessage
			if err := json.Unmarshal(q.Criteria, &levels); err != nil || len(levels) < 2 {
				return fmt.Errorf("question %q: score requires at least two levels", name)
			}
			for _, value := range levels {
				if !content(value, true) {
					return fmt.Errorf("question %q: invalid level description", name)
				}
			}
		default:
			return fmt.Errorf("question %q: type must be noul, choice, or score", name)
		}
	}
	return nil
}
func content(b []byte, nullable bool) bool {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return false
	}
	return b[0] == '"' || b[0] == '{' || b[0] == '[' || (nullable && bytes.Equal(b, []byte("null")))
}
