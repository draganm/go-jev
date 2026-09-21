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
		log.Fatal(err)
	}

	res, err := client.Ask(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}

	urgent, _ := res.Noul("is_urgent")
	dept, _ := res.Choice("department")
	frustration, _ := res.Score("frustration")
	fmt.Printf("urgent=%.2f department=%s (confidence %.2f) frustration=%.2f\n",
		urgent, dept.Choice, dept.Confidence, frustration.Score)
}
