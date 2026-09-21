# go-jev

[![Go Reference](https://pkg.go.dev/badge/github.com/draganm/go-jev.svg)](https://pkg.go.dev/github.com/draganm/go-jev)
[![License: LGPL v3](https://img.shields.io/badge/License-LGPL_v3-blue.svg)](LICENSE)

A Go client for the [TypeSafe](https://docs.typesafe.ai) System One API, which serves the Jev models.

Send a state and typed questions; get structured answers with calibrated probabilities.

- All three question types: **Noul** (yes/no probability), **Choice** (pick one option) and
  **Score** (rate against ordered levels)
- Fluent request builder that collects all validation errors in one place
- Typed answers and errors, with `errors.Is` sentinels for common failures
- Retries with backoff and `Retry-After` support, matching the official SDKs
- No dependencies beyond the Go standard library (requires Go 1.26+)

This is an unofficial client. It is not affiliated with or endorsed by TypeSafe.

```sh
go get github.com/draganm/go-jev
```

## Usage

```go
client, err := jev.NewClient() // reads TYPESAFE_API_KEY
if err != nil {
	log.Fatal(err)
}

req, err := jev.NewRequest("Help! My payouts have been failing for 3 days.").
	Noul("is_urgent", "Does this convey urgency?").
	Choice("department", "Which team should handle this?",
		jev.Opt("billing", "Payments, invoicing, refunds"),
		jev.Opt("technical", "Bugs, outages, integrations"),
		jev.Opt("sales", "Pricing, upgrades, new accounts"),
	).
	Score("frustration", "How frustrated is the customer?", "Calm", "Frustrated", "Very angry").
	Build()
if err != nil {
	log.Fatal(err) // collects every problem: duplicate ids, invalid questions, ...
}

res, err := client.Ask(ctx, req)
if err != nil {
	log.Fatal(err)
}

urgent, _ := res.Noul("is_urgent")         // 0.95
dept, _ := res.Choice("department")        // dept.Choice, dept.Probabilities, dept.Confidence
frustration, _ := res.Score("frustration") // frustration.Score, .Legend, .Probabilities, .Confidence
```

Other builder methods: `Model(...)`, `NoulWithCriteria(id, instructions, whenTrue, whenFalse)`,
`Question(id, q)` for a prebuilt question, and `jev.ChoiceOptions("a", "b")...` for options
without descriptions.

You can also build the request as a struct literal instead:

```go
req := &jev.Request{
	State: "Help! My payouts have been failing for 3 days.",
	Questions: map[string]jev.Question{
		"is_urgent":   jev.Noul{Instructions: "Does this convey urgency?"},
		"department":  jev.Choice{Instructions: "Which team?", Options: jev.ChoiceOptions("billing", "technical")},
		"frustration": jev.Score{Instructions: "How frustrated?", Levels: jev.Levels("Calm", "Angry")},
	},
}
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

## License

Licensed under the [GNU Lesser General Public License v3.0](LICENSE). You can use this
library in proprietary programs; changes to the library itself must be shared under the
same license.
