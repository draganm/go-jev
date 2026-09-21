package jev_test

import (
	"context"
	"fmt"
	"log"

	jev "github.com/draganm/go-jev"
)

func Example() {
	client, err := jev.NewClient() // reads TYPESAFE_API_KEY
	if err != nil {
		log.Fatal(err)
	}

	res, err := client.Ask(context.Background(), &jev.Request{
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

	urgent, _ := res.Noul("is_urgent")
	dept, _ := res.Choice("department")
	frustration, _ := res.Score("frustration")
	fmt.Printf("urgent=%.2f department=%s (confidence %.2f) frustration=%.2f\n",
		urgent, dept.Choice, dept.Confidence, frustration.Score)
}
