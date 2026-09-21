// Package jev is a Go client for the TypeSafe System One API
// (https://docs.typesafe.ai), which serves the Jev family of models.
//
// A request carries a state (text or structured JSON) and a set of named,
// typed questions. Each question is one of three primitives:
//
//   - [Noul]: a yes/no question, answered with the probability of yes.
//   - [Choice]: pick one option from a set, answered with the chosen option,
//     a probability per option, and a confidence.
//   - [Score]: rate against ordered levels, answered with a
//     probability-weighted score, a probability per level, and a confidence.
//
// Basic usage:
//
//	client, err := jev.NewClient() // reads TYPESAFE_API_KEY
//	if err != nil {
//		return err
//	}
//	res, err := client.Ask(ctx, &jev.Request{
//		State: "Help! My payouts have been failing for 3 days.",
//		Questions: map[string]jev.Question{
//			"is_urgent": jev.Noul{Instructions: "Does this convey urgency?"},
//		},
//	})
//	if err != nil {
//		return err
//	}
//	fmt.Println(res.Answers["is_urgent"].Noul)
package jev
