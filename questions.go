package jev

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// QuestionType identifies one of the System One primitives.
type QuestionType string

const (
	TypeNoul   QuestionType = "noul"
	TypeChoice QuestionType = "choice"
	TypeScore  QuestionType = "score"
)

const (
	// MaxChoiceOptions is the maximum number of options the API accepts for a Choice.
	MaxChoiceOptions = 255
	// MinScoreLevels is the minimum number of levels a Score should have.
	MinScoreLevels = 2
	// MaxScoreLevels is the maximum number of levels the API accepts for a Score.
	MaxScoreLevels = 10
)

// Question is one of [Noul], [Choice] or [Score].
//
// Instructions, criteria and descriptions are typed as any: they may be a
// string, or any JSON-marshalable object or array (see
// https://docs.typesafe.ai/primitives/advanced).
type Question interface {
	QuestionType() QuestionType
	validate() error
}

// Noul is a yes/no question. The answer is the probability that the answer is yes.
type Noul struct {
	// Instructions is the yes/no question to evaluate. Required.
	Instructions any
	// Criteria optionally describes what a yes and a no mean.
	Criteria *NoulCriteria
}

// NoulCriteria describes what the two ends of a [Noul] mean.
type NoulCriteria struct {
	// True describes what a yes (value near 1) means.
	True any `json:"true,omitempty"`
	// False describes what a no (value near 0) means.
	False any `json:"false,omitempty"`
}

func (Noul) QuestionType() QuestionType { return TypeNoul }

func (q Noul) validate() error {
	if isEmpty(q.Instructions) {
		return errors.New("noul: instructions are required")
	}
	return nil
}

func (q Noul) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         QuestionType  `json:"type"`
		Instructions any           `json:"instructions"`
		Criteria     *NoulCriteria `json:"criteria,omitempty"`
	}{TypeNoul, q.Instructions, q.Criteria})
}

// Choice selects one option from a defined set.
type Choice struct {
	// Instructions is what the model should decide. Required.
	Instructions any
	// Options are the options to choose from, in order. At least one and at
	// most [MaxChoiceOptions] options are allowed, and names must be unique.
	Options []ChoiceOption
}

// ChoiceOption is a single [Choice] option.
type ChoiceOption struct {
	// Name is the option key, returned in [Answer.Choice] and as a key of
	// [Answer.Probabilities].
	Name string
	// Description optionally describes the option. A nil Description is sent
	// as null.
	Description any
}

// ChoiceOptions builds options without descriptions from names.
func ChoiceOptions(names ...string) []ChoiceOption {
	opts := make([]ChoiceOption, len(names))
	for i, n := range names {
		opts[i] = ChoiceOption{Name: n}
	}
	return opts
}

func (Choice) QuestionType() QuestionType { return TypeChoice }

func (q Choice) validate() error {
	if isEmpty(q.Instructions) {
		return errors.New("choice: instructions are required")
	}
	if len(q.Options) == 0 {
		return errors.New("choice: at least one option is required")
	}
	if len(q.Options) > MaxChoiceOptions {
		return fmt.Errorf("choice: %d options exceeds the maximum of %d", len(q.Options), MaxChoiceOptions)
	}
	seen := make(map[string]struct{}, len(q.Options))
	for _, o := range q.Options {
		if o.Name == "" {
			return errors.New("choice: option name must not be empty")
		}
		if _, dup := seen[o.Name]; dup {
			return fmt.Errorf("choice: duplicate option %q", o.Name)
		}
		seen[o.Name] = struct{}{}
	}
	return nil
}

func (q Choice) MarshalJSON() ([]byte, error) {
	// criteria is a JSON object; build it by hand to keep option order.
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, o := range q.Options {
		if i > 0 {
			buf.WriteByte(',')
		}
		k, err := json.Marshal(o.Name)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(o.Description)
		if err != nil {
			return nil, fmt.Errorf("choice option %q: %w", o.Name, err)
		}
		buf.Write(k)
		buf.WriteByte(':')
		buf.Write(v)
	}
	buf.WriteByte('}')
	return json.Marshal(struct {
		Type         QuestionType    `json:"type"`
		Instructions any             `json:"instructions"`
		Criteria     json.RawMessage `json:"criteria"`
	}{TypeChoice, q.Instructions, buf.Bytes()})
}

// Score rates the state against ordered, descriptive levels.
type Score struct {
	// Instructions is what the model should rate. Required.
	Instructions any
	// Levels are the ordered level descriptions, lowest first. Between
	// [MinScoreLevels] and [MaxScoreLevels] levels are allowed.
	Levels []any
}

// Levels is a convenience for building [Score.Levels] from strings.
func Levels(descriptions ...string) []any {
	levels := make([]any, len(descriptions))
	for i, d := range descriptions {
		levels[i] = d
	}
	return levels
}

func (Score) QuestionType() QuestionType { return TypeScore }

func (q Score) validate() error {
	if isEmpty(q.Instructions) {
		return errors.New("score: instructions are required")
	}
	if n := len(q.Levels); n < MinScoreLevels || n > MaxScoreLevels {
		return fmt.Errorf("score: %d levels given, must be between %d and %d", n, MinScoreLevels, MaxScoreLevels)
	}
	return nil
}

func (q Score) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type         QuestionType `json:"type"`
		Instructions any          `json:"instructions"`
		Criteria     []any        `json:"criteria"`
	}{TypeScore, q.Instructions, q.Levels})
}

func isEmpty(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}
