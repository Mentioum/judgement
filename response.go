package judgement

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Validate the TypeSafe response before either the raw CLI path or typed client
// can report success. Unknown fields remain untouched in the original JSON.
func validateEvaluation(raw json.RawMessage, request Request) error {
	var response struct {
		Model   string            `json:"model"`
		Answers map[string]Answer `json:"answers"`
		Usage   *struct {
			Input  *int64 `json:"input_tokens"`
			Output *int64 `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &response); err != nil {
		return errors.New("API returned an invalid evaluation response")
	}
	if response.Model == "" || response.Answers == nil || response.Usage == nil || response.Usage.Input == nil || response.Usage.Output == nil {
		return errors.New("API response must contain model, answers, and token usage")
	}
	if *response.Usage.Input < 0 || *response.Usage.Output < 0 {
		return errors.New("API response contains negative token usage")
	}
	for name, question := range request.Questions {
		answer, ok := response.Answers[name]
		if !ok || answer.Type != question.Type {
			return fmt.Errorf("API response is missing or mismatches answer %q", name)
		}
		switch question.Type {
		case "noul":
			if !probability(answer.Noul) {
				return fmt.Errorf("API response has an invalid noul for %q", name)
			}
		case "choice", "score":
			if !probability(answer.Confidence) || len(answer.Probabilities) == 0 {
				return fmt.Errorf("API response is missing confidence or probabilities for %q", name)
			}
			for _, value := range answer.Probabilities {
				if !probability(&value) {
					return fmt.Errorf("API response has an invalid probability for %q", name)
				}
			}
			if question.Type == "choice" {
				if answer.Choice == nil {
					return fmt.Errorf("API response is missing choice for %q", name)
				}
				if _, ok := answer.Probabilities[*answer.Choice]; !ok {
					return fmt.Errorf("API response choice has no probability for %q", name)
				}
			} else if answer.Score == nil || answer.Legend == nil {
				return fmt.Errorf("API response is missing score or legend for %q", name)
			}
		}
	}
	return nil
}

func probability(value *float64) bool { return value != nil && *value >= 0 && *value <= 1 }
