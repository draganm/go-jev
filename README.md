# go-jev

A Go client for the [TypeSafe](https://docs.typesafe.ai) System One API, which serves the Jev models.

Send a state and typed questions; get structured answers with calibrated probabilities.

```sh
go get github.com/draganm/go-jev
```

## Usage

```go
client, err := jev.NewClient() // reads TYPESAFE_API_KEY
if err != nil {
	log.Fatal(err)
}

res, err := client.Ask(ctx, &jev.Request{
	State: "Help! My payouts have been failing for 3 days.",
	Questions: map[string]jev.Question{
		"is_urgent": jev.Noul{Instructions: "Does this convey urgency?"},
		"department": jev.Choice{
			Instructions: "Which team should handle this?",
			Options: []jev.ChoiceOption{
				{Name: "billing", Description: "Payments, invoicing, refunds"},
				{Name: "technical", Description: "Bugs, outages, integrations"},
				{Name: "sales", Description: "Pricing, upgrades, new accounts"},
			},
		},
		"frustration": jev.Score{
			Instructions: "How frustrated is the customer?",
			Levels:       jev.Levels("Calm", "Frustrated", "Very angry"),
		},
	},
})
if err != nil {
	log.Fatal(err)
}

urgent, _ := res.Noul("is_urgent")      // 0.95
dept, _ := res.Choice("department")     // dept.Choice, dept.Probabilities, dept.Confidence
frustration, _ := res.Score("frustration") // frustration.Score, .Legend, .Probabilities, .Confidence
```

`State`, `Instructions`, option descriptions and score levels accept a string or any
JSON-marshalable value, so you can pass structured data (see
[Advanced: structure](https://docs.typesafe.ai/primitives/advanced)).

List available models:

```go
models, err := client.ListModels(ctx)
```

## Configuration

| Option               | Environment variable     | Default                   |
| -------------------- | ------------------------ | ------------------------- |
| `WithAPIKey`         | `TYPESAFE_API_KEY`       | required                  |
| `WithBaseURL`        | `TYPESAFE_BASE_URL`      | `https://api.typesafe.ai` |
| `WithDefaultModel`   | `TYPESAFE_DEFAULT_MODEL` | `jev-latest`              |
| `WithTimeout`        |                          | 10s per attempt           |
| `WithRetryPolicy`    |                          | `DefaultRetryPolicy()`    |
| `WithHTTPClient`     |                          | `http.DefaultClient`      |
| `WithUserAgent`      |                          | `go-jev`                  |

## Errors and retries

Non-2xx responses are returned as `*jev.APIError`, carrying the status, body and
`x-typesafe-request-id`. They match sentinels via `errors.Is`: `ErrAuthentication`,
`ErrUnprocessable`, `ErrRateLimited`, `ErrOverloaded` (529), `ErrServer`, and so on.
Transport failures and timeouts are returned as `*jev.ConnectionError`.

By default, like the official SDKs, the client retries 408, 429 and 5xx responses and
connection errors up to 2 times. Backoff is exponential (500ms doubling to 5s, 25% jitter),
and the client honors `Retry-After` / `retry-after-ms` up to 60s. Pass
`jev.WithRetryPolicy(jev.NoRetries)` to disable retries.

## Development

```sh
nix develop   # or direnv allow
go test -race ./...
```
