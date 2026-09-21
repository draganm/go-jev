package jev

import (
	"errors"
	"fmt"
	"maps"
)

// RequestBuilder builds a [Request] fluently:
//
//	req, err := jev.NewRequest("Help! My payouts have been failing for 3 days.").
//		Noul("is_urgent", "Does this convey urgency?").
//		Choice("department", "Which team should handle this?",
//			jev.Opt("billing", "Payments, invoicing, refunds"),
//			jev.Opt("technical", "Bugs, outages, integrations"),
//		).
//		Score("frustration", "How frustrated is the customer?", "Calm", "Frustrated", "Very angry").
//		Build()
//
// Errors such as duplicate question ids or invalid questions are collected
// and returned by [RequestBuilder.Build].
type RequestBuilder struct {
	req  Request
	errs []error
}

// NewRequest starts a request for the given state: a string, or any
// JSON-marshalable object or array.
func NewRequest(state any) *RequestBuilder {
	return &RequestBuilder{req: Request{State: state, Questions: map[string]Question{}}}
}

// Model sets the model. If not called, the client's default model is used.
func (b *RequestBuilder) Model(model string) *RequestBuilder {
	b.req.Model = model
	return b
}

// Question adds a question under id.
func (b *RequestBuilder) Question(id string, q Question) *RequestBuilder {
	switch {
	case id == "":
		b.errs = append(b.errs, errors.New("question id must not be empty"))
	case q == nil:
		b.errs = append(b.errs, fmt.Errorf("question %q is nil", id))
	default:
		if _, dup := b.req.Questions[id]; dup {
			b.errs = append(b.errs, fmt.Errorf("duplicate question id %q", id))
			return b
		}
		if err := q.validate(); err != nil {
			b.errs = append(b.errs, fmt.Errorf("question %q: %w", id, err))
		}
		b.req.Questions[id] = q
	}
	return b
}

// Noul adds a yes/no question.
func (b *RequestBuilder) Noul(id string, instructions any) *RequestBuilder {
	return b.Question(id, Noul{Instructions: instructions})
}

// NoulWithCriteria adds a yes/no question with descriptions of what a yes
// and a no mean. Either description may be nil.
func (b *RequestBuilder) NoulWithCriteria(id string, instructions, whenTrue, whenFalse any) *RequestBuilder {
	return b.Question(id, Noul{Instructions: instructions, Criteria: &NoulCriteria{True: whenTrue, False: whenFalse}})
}

// Choice adds a question that picks one of options, in the given order.
// Build options with [Opt], or from plain names with [ChoiceOptions]:
//
//	b.Choice("color", "Which color?", jev.ChoiceOptions("red", "green")...)
func (b *RequestBuilder) Choice(id string, instructions any, options ...ChoiceOption) *RequestBuilder {
	return b.Question(id, Choice{Instructions: instructions, Options: options})
}

// Score adds a question that rates against levels, lowest first. Each level
// is a string or any JSON-marshalable value.
func (b *RequestBuilder) Score(id string, instructions any, levels ...any) *RequestBuilder {
	return b.Question(id, Score{Instructions: instructions, Levels: levels})
}

// Build returns the request, or all errors collected while building it.
// The builder can be reused afterwards without affecting the returned request.
func (b *RequestBuilder) Build() (*Request, error) {
	errs := b.errs
	if len(b.req.Questions) == 0 {
		errs = append(errs[:len(errs):len(errs)], errors.New("at least one question is required"))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, fmt.Errorf("typesafe: building request: %w", err)
	}
	req := b.req
	req.Questions = maps.Clone(b.req.Questions)
	return &req, nil
}

// Opt is a shorthand for a [ChoiceOption] with a description.
func Opt(name string, description any) ChoiceOption {
	return ChoiceOption{Name: name, Description: description}
}
