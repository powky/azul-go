package azul

import (
	"context"
	"encoding/json"
	"fmt"
)

// ============================================================================
// SearchPayments
// ============================================================================

// SearchRequest contains the parameters to search for transactions in a date range.
type SearchRequest struct {
	// DateFrom is the start date in YYYYMMDD format (e.g. "20250101").
	DateFrom string

	// DateTo is the end date in YYYYMMDD format (e.g. "20250131").
	DateTo string
}

// SearchTransaction represents a single transaction returned by SearchPayments.
type SearchTransaction struct {
	AzulOrderId       string `json:"AzulOrderId"`
	Amount            string `json:"Amount"`
	AuthorizationCode string `json:"AuthorizationCode"`
	CardNumber        string `json:"CardNumber"`
	CurrencyPosCode   string `json:"CurrencyPosCode"`
	CustomOrderId     string `json:"CustomOrderId"`
	DateTime          string `json:"DateTime"`
	ErrorDescription  string `json:"ErrorDescription"`
	Found             string `json:"Found"`
	IsoCode           string `json:"IsoCode"`
	Itbis             string `json:"Itbis"`
	LotNumber         string `json:"LotNumber"`
	OrderNumber       string `json:"OrderNumber"`
	RRN               string `json:"RRN"`
	ResponseCode      string `json:"ResponseCode"`
	Ticket            string `json:"Ticket"`
	TransactionType   string `json:"TransactionType"`
}

// SearchResponse represents the response from Azul's SearchPayments endpoint.
//
// Unlike other API methods that return APIResponse, SearchPayments returns
// a list of transactions in the Transactions field.
type SearchResponse struct {
	ErrorDescription string              `json:"ErrorDescription"`
	ResponseCode     string              `json:"ResponseCode"`
	Transactions     []SearchTransaction `json:"Transactions"`
}

// HasError returns true if Azul could not process the search (ResponseCode "Error").
func (r *SearchResponse) HasError() bool {
	return r.ResponseCode == "Error"
}

// SearchPayments retrieves transactions within a date range through Azul's SearchPayments method.
//
// The endpoint is: base URL + "?SearchPayments"
//
// Returns a SearchResponse containing a list of SearchTransaction objects.
// Note that this method returns *SearchResponse instead of *APIResponse because
// the response structure is different (contains a Transactions array).
//
// Example:
//
//	resp, err := client.SearchPayments(ctx, azul.SearchRequest{
//	    DateFrom: "20250101",
//	    DateTo:   "20250131",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, tx := range resp.Transactions {
//	    fmt.Printf("Order: %s, Amount: %s, Status: %s\n",
//	        tx.AzulOrderId, tx.Amount, tx.IsoCode)
//	}
func (c *APIClient) SearchPayments(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	payload := map[string]string{
		"Channel":  c.config.Channel,
		"Store":    c.config.Store,
		"DateFrom": req.DateFrom,
		"DateTo":   req.DateTo,
	}

	respBody, err := c.doRequestRaw(ctx, "SearchPayments", payload)
	if err != nil {
		return nil, err
	}

	var result SearchResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("azul: failed to decode search response: %w", err)
	}

	return &result, nil
}
