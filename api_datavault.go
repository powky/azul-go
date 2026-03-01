package azul

import "context"

// ============================================================================
// CreateToken (ProcessDatavault - CREATE)
// ============================================================================

// CreateTokenRequest contains the parameters to tokenize a card in Azul's DataVault.
type CreateTokenRequest struct {
	// CardNumber is the full card number to tokenize.
	CardNumber string

	// Expiration is the card expiration in YYYYMM format (e.g. "202512").
	Expiration string

	// CVC is the card security code. Optional for token creation.
	CVC string
}

// CreateToken creates a DataVault token for a card through Azul's ProcessDatavault method.
//
// This tokenizes the card without making a charge. The token can be used later
// for TokenSale transactions. Azul returns the token in APIResponse.DataVaultToken.
//
// The endpoint is: base URL + "?ProcessDatavault"
//
// Example:
//
//	resp, err := client.CreateToken(ctx, azul.CreateTokenRequest{
//	    CardNumber: "4111111111111111",
//	    Expiration: "202512",
//	    CVC:        "123",
//	})
//	if resp.IsApproved() {
//	    token := resp.DataVaultToken // Save this for future charges
//	}
func (c *APIClient) CreateToken(ctx context.Context, req CreateTokenRequest) (*APIResponse, error) {
	payload := map[string]string{
		"Channel":    c.config.Channel,
		"Store":      c.config.Store,
		"CardNumber": req.CardNumber,
		"Expiration": req.Expiration,
		"CVC":        req.CVC,
		"TrxType":    "CREATE",
	}

	return c.doRequest(ctx, "ProcessDatavault", payload)
}

// ============================================================================
// DeleteToken (ProcessDatavault - DELETE)
// ============================================================================

// DeleteTokenRequest contains the parameters to delete a DataVault token.
type DeleteTokenRequest struct {
	// DataVaultToken is the token to delete.
	DataVaultToken string
}

// DeleteToken removes a DataVault token through Azul's ProcessDatavault method.
//
// Once deleted, the token can no longer be used for transactions.
//
// The endpoint is: base URL + "?ProcessDatavault"
//
// Example:
//
//	resp, err := client.DeleteToken(ctx, azul.DeleteTokenRequest{
//	    DataVaultToken: "6EF85D01-B07C-4E67-99F7-4E13A449DCDD",
//	})
func (c *APIClient) DeleteToken(ctx context.Context, req DeleteTokenRequest) (*APIResponse, error) {
	payload := map[string]string{
		"Channel":        c.config.Channel,
		"Store":          c.config.Store,
		"DataVaultToken": req.DataVaultToken,
		"TrxType":        "DELETE",
	}

	return c.doRequest(ctx, "ProcessDatavault", payload)
}
