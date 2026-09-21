package jev

import "fmt"

// Request is the body of POST /v1/systemone.
type Request struct {
	// State is the content to evaluate: a string, or any JSON-marshalable
	// object or array (including json.RawMessage).
	State any `json:"state"`
	// Model selects the model. If empty, the client's default model is used.
	Model string `json:"model"`
	// Questions maps ids you choose to questions. Answers are returned under
	// the same ids. At least one question is required.
	Questions map[string]Question `json:"questions"`
}

// Response is the result of POST /v1/systemone.
type Response struct {
	// Model is the versioned model id that answered, e.g. "jev-1.13.0".
	Model string `json:"model"`
	// Answers holds one answer per question, keyed by question id.
	Answers map[string]Answer `json:"answers"`
	// Usage reports token usage for the request.
	Usage Usage `json:"usage"`
	// RequestID is the value of the x-typesafe-request-id response header.
	RequestID string `json:"-"`
}

// Answer is the answer to one question. Which fields are set depends on Type.
type Answer struct {
	Type QuestionType `json:"type"`

	// Noul is the yes/no answer on a scale from 0 (no) to 1 (yes). Noul only.
	Noul float64 `json:"noul,omitempty"`

	// Choice is the highest-probability option. Choice only.
	Choice string `json:"choice,omitempty"`

	// Score is the probability-weighted level index; it can land between
	// levels. Score only.
	Score float64 `json:"score,omitempty"`
	// Legend maps each level index (as a string) back to its description.
	// Score only.
	Legend map[string]string `json:"legend,omitempty"`

	// Probabilities maps each option (Choice) or level index as a string
	// (Score) to its probability.
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	// Confidence is how certain the model is, between 0 and 1. Choice and
	// Score only.
	Confidence float64 `json:"confidence,omitempty"`
}

// Usage is token usage for a request.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Noul returns the noul value answering question id.
func (r *Response) Noul(id string) (float64, error) {
	a, err := r.answer(id, TypeNoul)
	return a.Noul, err
}

// Choice returns the answer to the choice question id.
func (r *Response) Choice(id string) (Answer, error) {
	return r.answer(id, TypeChoice)
}

// Score returns the answer to the score question id.
func (r *Response) Score(id string) (Answer, error) {
	return r.answer(id, TypeScore)
}

func (r *Response) answer(id string, t QuestionType) (Answer, error) {
	a, ok := r.Answers[id]
	if !ok {
		return Answer{}, fmt.Errorf("no answer for question %q", id)
	}
	if a.Type != t {
		return Answer{}, fmt.Errorf("answer for question %q has type %q, not %q", id, a.Type, t)
	}
	return a, nil
}

// ModelCard describes a model or alias available to the account.
type ModelCard struct {
	// Name is the model id or alias, as accepted by [Request.Model].
	Name string `json:"name"`
	// Description says what the model is for.
	Description string `json:"description"`
	// ReleaseDate is when the model or alias was released.
	ReleaseDate string `json:"release_date"`
}
