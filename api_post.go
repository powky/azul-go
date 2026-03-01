package azul

import "context"

// ============================================================================
// Post (ProcessPost — Capture a Hold)
// ============================================================================

// PostRequest contains the parameters to capture (post) a previously approved Hold.
type PostRequest struct {
	// AzulOrderId is the Azul-assigned transaction ID from the original Hold response.
	AzulOrderId string

	// Amount is the amount to capture. Must be equal to or less than the Hold amount.
	Amount float64

	// ITBIS is the tax amount to capture. Set to 0 if exempt.
	ITBIS float64
}

// Post captures a previously approved Hold transaction through Azul's ProcessPost method.
//
// The captured amount can be equal to or less than the original Hold amount.
// Once posted, the transaction will be settled in the next batch and can only
// be voided within 20 minutes.
//
// The endpoint is: base URL + "?ProcessPost"
//
// Example:
//
//	// First, create a Hold
//	holdResp, err := client.Hold(ctx, azul.HoldRequest{
//	    CardNumber:  "4111111111111111",
//	    Expiration:  "202812",
//	    CVC:         "123",
//	    Amount:      2000.00,
//	    ITBIS:       0,
//	    OrderNumber: "ORD-HOLD-1",
//	})
//
//	// Then, capture the Hold (full or partial amount)
//	postResp, err := client.Post(ctx, azul.PostRequest{
//	    AzulOrderId: holdResp.AzulOrderId,
//	    Amount:      2000.00,
//	    ITBIS:       0,
//	})
func (c *APIClient) Post(ctx context.Context, req PostRequest) (*APIResponse, error) {
	payload := map[string]string{
		"Channel":     c.config.Channel,
		"Store":       c.config.Store,
		"AzulOrderId": req.AzulOrderId,
		"Amount":      FormatAmount(req.Amount),
		"Itbis":       FormatAmount(req.ITBIS),
	}

	return c.doRequest(ctx, "ProcessPost", payload)
}
