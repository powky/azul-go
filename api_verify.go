package azul

import "context"

// ============================================================================
// VerifyPayment
// ============================================================================

// VerifyRequest contains the parameters to verify a previous transaction.
type VerifyRequest struct {
	// CustomOrderId is the merchant-assigned identifier used in the original transaction.
	// This is the identifier Azul uses to look up the transaction.
	CustomOrderId string
}

// VerifyPayment checks the status of a previous transaction identified by CustomOrderId.
//
// This is useful when you need to confirm whether a payment was processed
// (e.g. after a network timeout or ambiguous response). If multiple transactions
// share the same CustomOrderId, Azul returns the most recent one.
//
// The endpoint is: base URL + "?VerifyPayment"
//
// Check APIResponse.IsFound() to determine if the transaction was found.
//
// Example:
//
//	resp, err := client.VerifyPayment(ctx, azul.VerifyRequest{
//	    CustomOrderId: "ABC123",
//	})
//	if resp.IsFound() && resp.IsApproved() {
//	    // Transaction was found and approved
//	}
func (c *APIClient) VerifyPayment(ctx context.Context, req VerifyRequest) (*APIResponse, error) {
	payload := map[string]string{
		"Channel":       c.config.Channel,
		"Store":         c.config.Store,
		"CustomOrderId": req.CustomOrderId,
	}

	return c.doRequest(ctx, "VerifyPayment", payload)
}
