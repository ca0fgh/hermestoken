package dto

import "fmt"

type SubscriptionPaymentRequest struct {
	PlanId   int  `json:"plan_id"`
	Quantity *int `json:"quantity,omitempty"`
}

func (r SubscriptionPaymentRequest) GetQuantity() (int, error) {
	if r.Quantity == nil {
		return 1, nil
	}
	if *r.Quantity <= 0 {
		return 0, fmt.Errorf("quantity must be greater than 0")
	}
	return *r.Quantity, nil
}

type SubscriptionEpayPaymentRequest struct {
	SubscriptionPaymentRequest
	PaymentMethod string `json:"payment_method"`
}
