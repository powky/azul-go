// Package azul provides a Go client for Azul Payment Page (HPP) — the standard
// payment gateway used in the Dominican Republic.
//
// It handles:
//   - Building the hidden-form fields + HMAC-SHA512 AuthHash for redirecting to Azul
//   - Validating the callback hash when Azul redirects back
//   - Building VOID (cancellation) form fields
//   - Amount formatting (cents → Azul format)
//
// This package is transport-agnostic — it generates form data but does NOT make
// HTTP requests. Your application is responsible for rendering the HTML form or
// submitting the POST to Azul.
package azul

import (
	"fmt"
	"strings"
	"time"
)

// ============================================================================
// URLs
// ============================================================================

const (
	TestURL          = "https://pruebas.azul.com.do/PaymentPage/"
	ProductionURL    = "https://pagos.azul.com.do/PaymentPage/Default.aspx"
	ProductionAltURL = "https://contpagos.azul.com.do/PaymentPage/Default.aspx"
)

// ============================================================================
// Config
// ============================================================================

// Config holds all Azul Payment Page configuration.
//
// Required fields: MerchantID, AuthKey, MerchantName, ApprovedURL, DeclinedURL, CancelURL.
// Optional: MerchantType defaults to "ECommerce", TerminalID defaults to "00000001".
type Config struct {
	// MerchantID is the merchant identifier assigned by Azul (e.g. "39038540035").
	MerchantID string

	// AuthKey is the HMAC secret key provided by Azul for signing requests.
	AuthKey string

	// MerchantName is displayed on the Azul payment page (e.g. "Mi Tienda").
	MerchantName string

	// MerchantType identifies the integration type. Defaults to "ECommerce".
	MerchantType string

	// TerminalID is assigned by Azul (e.g. "00000001"). Defaults to "00000001".
	TerminalID string

	// Environment selects the Azul endpoint: "test" or "production".
	Environment string

	// ApprovedURL is where Azul redirects after a successful payment.
	ApprovedURL string

	// DeclinedURL is where Azul redirects after a declined payment.
	DeclinedURL string

	// CancelURL is where Azul redirects if the user cancels.
	CancelURL string

	// VoidCallbackURL is the redirect URL after a VOID transaction.
	// Falls back to ApprovedURL if empty.
	VoidCallbackURL string
}

func (c *Config) defaults() {
	if c.MerchantType == "" {
		c.MerchantType = "ECommerce"
	}
	if c.TerminalID == "" {
		c.TerminalID = "00000001"
	}
}

// ============================================================================
// Client
// ============================================================================

// Client is the main entry point for Azul Payment Page operations.
type Client struct {
	config Config
}

// NewClient creates a new Azul client with the given configuration.
func NewClient(config Config) *Client {
	config.defaults()
	return &Client{config: config}
}

// ============================================================================
// Payment
// ============================================================================

// PaymentRequest contains the parameters needed to build the Azul payment form.
type PaymentRequest struct {
	// OrderNumber is the unique reference for this transaction (e.g. "ORD-1234").
	OrderNumber string

	// Amount is the total amount in cents (e.g. 150000 = RD$1,500.00).
	Amount int64

	// ITBIS is the tax amount in cents. Set to 0 if ITBIS is already included in Amount.
	ITBIS int64
}

// PaymentResult contains everything needed to POST the payment form to Azul.
type PaymentResult struct {
	// ActionURL is the primary Azul Payment Page endpoint.
	ActionURL string

	// AltActionURL is the fallback endpoint (production only, empty for test).
	AltActionURL string

	// Fields is the complete set of hidden form fields to POST, including AuthHash.
	// Iterate this map to generate <input type="hidden"> elements.
	Fields map[string]string
}

// BuildPaymentForm generates the form fields and HMAC AuthHash for Azul's Payment Page.
//
// The returned PaymentResult.Fields map contains ALL fields needed for the POST form,
// including the computed AuthHash. Simply iterate the map and create hidden inputs.
//
// Example:
//
//	result := client.BuildPaymentForm(azul.PaymentRequest{
//	    OrderNumber: "ORD-1234",
//	    Amount:      150000,  // RD$1,500.00
//	    ITBIS:       0,
//	})
//	// result.ActionURL  → POST target
//	// result.Fields     → hidden form inputs
func (c *Client) BuildPaymentForm(req PaymentRequest) PaymentResult {
	amount := FormatAmount(req.Amount)
	itbis := FormatAmount(req.ITBIS)

	fields := map[string]string{
		"MerchantId":        c.config.MerchantID,
		"MerchantName":      c.config.MerchantName,
		"MerchantType":      c.config.MerchantType,
		"CurrencyCode":      "$",
		"OrderNumber":       req.OrderNumber,
		"Amount":            amount,
		"ITBIS":             itbis,
		"ApprovedUrl":       c.config.ApprovedURL,
		"DeclinedUrl":       c.config.DeclinedURL,
		"CancelUrl":         c.config.CancelURL,
		"UseCustomField1":   "0",
		"CustomField1Label": "",
		"CustomField1Value": "",
		"UseCustomField2":   "0",
		"CustomField2Label": "",
		"CustomField2Value": "",
	}

	hashUpper := signHPPRequest(c.config.AuthKey, fields)
	fields["AuthHash"] = strings.ToLower(hashUpper)

	// Extra POST fields (NOT included in hash, but required by Azul)
	fields["TerminalId"] = c.config.TerminalID
	fields["PosInputMode"] = c.config.MerchantType
	fields["CustomOrderId"] = req.OrderNumber

	return PaymentResult{
		ActionURL:    c.paymentPageURL(),
		AltActionURL: c.paymentPageAltURL(),
		Fields:       fields,
	}
}

// ============================================================================
// Void
// ============================================================================

// VoidRequest contains the parameters needed to build the Azul VOID form.
type VoidRequest struct {
	// OrderNumber is the original order reference.
	OrderNumber string

	// AzulOrderID is the Azul-assigned transaction ID from the original payment callback.
	AzulOrderID string

	// Amount is the original transaction amount in cents.
	Amount int64

	// ITBIS is the original tax amount in cents.
	ITBIS int64
}

// VoidResult contains everything needed to POST the VOID form to Azul.
type VoidResult struct {
	// ActionURL is the primary Azul Payment Page endpoint.
	ActionURL string

	// AltActionURL is the fallback endpoint (production only, empty for test).
	AltActionURL string

	// Fields is the complete set of hidden form fields to POST, including AuthHash.
	Fields map[string]string
}

// BuildVoidForm generates the form fields for voiding (cancelling) an Azul transaction.
//
// Azul processes VOIDs through the same Payment Page but with TrxType="Void" and
// the AzulOrderId of the original transaction. The user is redirected immediately
// without seeing a payment form.
//
// Example:
//
//	result := client.BuildVoidForm(azul.VoidRequest{
//	    OrderNumber: "ORD-1234",
//	    AzulOrderID: "abc-def-123",
//	    Amount:      150000,
//	    ITBIS:       0,
//	})
func (c *Client) BuildVoidForm(req VoidRequest) VoidResult {
	amount := FormatAmount(req.Amount)
	itbis := FormatAmount(req.ITBIS)

	voidURL := c.config.VoidCallbackURL
	if voidURL == "" {
		voidURL = c.config.ApprovedURL
	}

	fields := map[string]string{
		"MerchantId":        c.config.MerchantID,
		"MerchantName":      c.config.MerchantName,
		"MerchantType":      c.config.MerchantType,
		"CurrencyCode":      "$",
		"OrderNumber":       req.OrderNumber,
		"Amount":            amount,
		"ITBIS":             itbis,
		"ApprovedUrl":       voidURL,
		"DeclinedUrl":       voidURL,
		"CancelUrl":         voidURL,
		"UseCustomField1":   "0",
		"CustomField1Label": "",
		"CustomField1Value": "",
		"UseCustomField2":   "0",
		"CustomField2Label": "",
		"CustomField2Value": "",
	}

	hashUpper := signHPPRequest(c.config.AuthKey, fields)
	fields["AuthHash"] = strings.ToLower(hashUpper)

	fields["TrxType"] = "Void"
	fields["AzulOrderId"] = req.AzulOrderID
	fields["TerminalId"] = c.config.TerminalID
	fields["PosInputMode"] = c.config.MerchantType
	fields["CustomOrderId"] = req.OrderNumber

	return VoidResult{
		ActionURL:    c.paymentPageURL(),
		AltActionURL: c.paymentPageAltURL(),
		Fields:       fields,
	}
}

// ============================================================================
// Callback validation
// ============================================================================

// CallbackParams represents the query string parameters Azul sends back to
// ApprovedURL / DeclinedURL / CancelURL after processing a transaction.
//
// Parse these from the redirect URL query string or from a JSON body
// (depending on your frontend integration).
type CallbackParams struct {
	OrderNumber       string
	Amount            string
	ITBIS             string
	AuthorizationCode string
	DateTime          string
	ResponseCode      string
	IsoCode           string
	ResponseMessage   string
	ErrorDescription  string
	RRN               string
	AuthHash          string
	DataVaultToken    string
	DataVaultBrand    string
	AzulOrderID       string
}

// ValidateCallback verifies the HMAC AuthHash returned by Azul to ensure the
// callback was not forged.
//
// Azul's implementation is known to vary the field order and encoding across
// versions, so this function tries 4 field combinations × 2 encodings (UTF-8
// and UTF-16LE) for maximum compatibility.
//
// Returns true if the hash is valid (or if no hash was provided — some Azul
// configurations don't return AuthHash).
func (c *Client) ValidateCallback(params CallbackParams) bool {
	m := map[string]string{
		"OrderNumber":       params.OrderNumber,
		"Amount":            params.Amount,
		"AuthorizationCode": params.AuthorizationCode,
		"DateTime":          params.DateTime,
		"ResponseCode":      params.ResponseCode,
		"ISOCode":           params.IsoCode,
		"ResponseMessage":   params.ResponseMessage,
		"ErrorDescription":  params.ErrorDescription,
		"RRN":               params.RRN,
	}
	return verifyHPPReturn(c.config.AuthKey, m, params.AuthHash)
}

// IsApproved checks if the Azul callback indicates a successful payment.
//
// Azul returns IsoCode "00" for approved transactions. This is the standard
// ISO 8583 response code for "Approved".
func (c *Client) IsApproved(params CallbackParams) bool {
	return params.IsoCode == "00"
}

// ============================================================================
// Order number generation
// ============================================================================

// GenerateOrderNumber creates a unique order number using the given prefix
// and the current Unix timestamp in milliseconds.
//
// Example: GenerateOrderNumber("ORD") → "ORD-1709234567890"
func GenerateOrderNumber(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixMilli())
}

// ============================================================================
// Amount formatting
// ============================================================================

// FormatAmount converts cents (int64) to Azul's amount string format.
//
// Azul expects the amount as a string where the last 2 digits are cents,
// with no decimal separator. Minimum length is 3 characters (zero-padded).
//
// Examples:
//
//	FormatAmount(0)      → "000"
//	FormatAmount(50)     → "050"
//	FormatAmount(1500)   → "1500"    (= $15.00)
//	FormatAmount(150000) → "150000"  (= $1,500.00)
func FormatAmount(cents int64) string {
	s := fmt.Sprintf("%d", cents)
	if len(s) < 3 {
		return fmt.Sprintf("%03d", cents)
	}
	return s
}

// ParseAmount converts an Azul amount string back to cents.
//
// This is the inverse of FormatAmount. Returns 0 if the string is invalid.
//
// Examples:
//
//	ParseAmount("000")    → 0
//	ParseAmount("1500")   → 1500
//	ParseAmount("150000") → 150000
func ParseAmount(s string) int64 {
	var n int64
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ============================================================================
// Private helpers
// ============================================================================

func (c *Client) paymentPageURL() string {
	if strings.EqualFold(c.config.Environment, "production") {
		return ProductionURL
	}
	return TestURL
}

func (c *Client) paymentPageAltURL() string {
	if strings.EqualFold(c.config.Environment, "production") {
		return ProductionAltURL
	}
	return ""
}
