package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	judgement "github.com/Mentioum/judgement"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	client, err := judgement.NewClient(judgement.Config{APIKey: os.Getenv("TYPESAFE_API_KEY"), Timeout: 30 * time.Second, MaxRetries: 2})
	if err != nil {
		return err
	}
	result, err := client.Evaluate(context.Background(), judgement.Request{
		State: "Please refund the duplicate charge.",
		Questions: map[string]judgement.Question{
			"refund":  judgement.Noul("Is a refund requested?"),
			"team":    judgement.Choice("Which team should handle this?", map[string]any{"billing": "Payments and refunds", "support": "Product problems"}),
			"urgency": judgement.Score("How urgent is this?", []any{"No deadline", "This week", "Today"}),
		},
	})
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}
