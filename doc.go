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
//	req, err := jev.NewRequest("Help! My payouts have been failing for 3 days.").
//		Noul("is_urgent", "Does this convey urgency?").
//		Build()
//	if err != nil {
//		return err
//	}
//	res, err := client.Ask(ctx, req)
//	if err != nil {
//		return err
//	}
//	urgent, err := res.Noul("is_urgent")
//
// Requests can also be built as [Request] struct literals.
package jev
