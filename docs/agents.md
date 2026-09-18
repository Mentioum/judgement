# Using judgement in agent workflows

Judgement makes Jev available as a small, predictable command inside a larger
processing task. Your script owns file access, iteration, transformations, and
actions. Jev supplies bounded judgments about supplied context.

## Discover, prepare, validate, evaluate

1. Run `judgement describe` for the current machine-readable interface.
2. Build a JSON request using a real JSON serializer. Keep task data in `state`.
3. Define one atomic question per decision; batch questions sharing a state into
   the same request. Put domain rules in `instructions` and `criteria`.
4. Run `judgement validate --input request.json` before a paid call.
5. Call `judgement evaluate --input request.json` and parse stdout as JSON.
6. Check both process exit status and answer confidence before taking action.

The CLI never opens an interactive prompt. Obtain `TYPESAFE_API_KEY` from the
execution environment. Do not put it in source code, request JSON, or arguments.
`validate`, `schema`, `describe`, `version`, and `--help` work offline.

## Select the right primitive

Use Noul for a single yes/no proposition. Use Choice for mutually exclusive
categories and include an `other` or `unknown` option when needed. Use Score for
an ordered rubric. For extraction, supply explicit candidates or break the
problem into parts; Jev does not produce arbitrary text spans.

Instructions and criteria can be JSON objects or arrays, allowing taxonomies,
examples, and domain rules to stay structured. Question names correlate answers;
put the actual task meaning in the question's instructions and criteria.

## Compose results in code

```sh
judgement evaluate --input examples/request.json > result.json
jq '{team: .answers.team.choice,
     confidence: .answers.team.confidence,
     refund_probability: .answers.refund_requested.noul}' result.json
```

A Noul probability is not a Choice/Score confidence value. Keep both semantics
visible in downstream records. Thresholds depend on the dataset and the cost of
an incorrect decision; calibrate them on examples with known outcomes. Save the
reported model version and token usage with each result. Pin a model version
when evaluating or deploying a threshold.

See `examples/process.py` for a script that calls the CLI without shell expansion
and parses results. It demonstrates invocation only; it does not automate refunds
or choose a production decision threshold.

## Batch processing and recovery

Use JSONL envelopes with stable IDs for independent records:

```sh
judgement batch --input examples/batch.jsonl --concurrency 4 > results.jsonl
```

Results retain input order. Each record contains either `result` or `error`;
batch exit 1 means at least one record failed. Inspect and retry only failed
records where retrying is appropriate. Do not blindly replay the whole dataset.

The CLI retries selected transient HTTP responses, including rate limits and
overload, and honors server retry delays within the per-call deadline. Network
failures and timeouts are not replayed automatically because a paid request may
already have reached the service. `error.retryable` describes HTTP retry
eligibility, not whether another retry will necessarily succeed.

Concurrency is an upper bound, not a requests-per-minute limiter. Reduce it when
you encounter rate limits. Batches use windows of that many records to bound
buffering and retain ordering; a slow record can delay output from its window.
Empty lines produce errors instead of silently disappearing. No automatic
checkpointing or resume database is provided.

## Output discipline

* Parse JSON; avoid scraping human help text.
* Save stderr separately when capturing results.
* Never infer an answer from a failed invocation.
* Preserve the API's uncertainty; low confidence may require a different model,
  more evidence, or review.
* Use the agent's existing task authorization for downstream actions. Model
  output is evidence for a decision, not permission to perform an action.
