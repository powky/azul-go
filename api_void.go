package azul

import "context"

// ============================================================================
// Void (ProcessVoid)
// ============================================================================

// APIVoidRequest contains the parameters to void (cancel) an Azul transaction.
type APIVoidRequest struct {
	// AzulOrderId is the Azul-assigned transaction ID from the original payment response.
	AzulOrderId string
}

// Void cancels a previously approved transaction through Azul's ProcessVoid method.
//
// Sales and Posts can be voided within 20 minutes of approval.
// Holds that have not been posted can be voided at any time.
//
// The endpoint is: base URL + "?ProcessVoid"
//
// Example:
//
//	resp, err := client.Void(ctx, azul.APIVoidRequest{
//	    AzulOrderId: "18527",
//	})
func (c *APIClient) Void(ctx context.Context, req APIVoidRequest) (*APIResponse, error) {
	payload := map[string]string{
		"Channel":     c.config.Channel,
		"Store":       c.config.Store,
		"AzulOrderId": req.AzulOrderId,
	}

	return c.doRequest(ctx, "ProcessVoid", payload)
}
