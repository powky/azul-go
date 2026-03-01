package azul

import (
	"strings"
)

// ============================================================================
// HPP URLs
// ============================================================================

const (
	HPPTestURL          = "https://pruebas.azul.com.do/PaymentPage/"
	HPPProductionURL    = "https://pagos.azul.com.do/PaymentPage/Default.aspx"
	HPPProductionAltURL = "https://contpagos.azul.com.do/PaymentPage/Default.aspx"

	// Deprecated: Use HPPTestURL instead.
	TestURL = HPPTestURL
	// Deprecated: Use HPPProductionURL instead.
	ProductionURL = HPPProductionURL
	// Deprecated: Use HPPProductionAltURL instead.
	ProductionAltURL = HPPProductionAltURL
)

// ============================================================================
// HPPConfig
// ============================================================================

// HPPConfig holds all Azul Payment Page (HPP) configuration.
//
// Required fields: MerchantID, AuthKey, MerchantName, ApprovedURL, DeclinedURL, CancelURL.
// Optional: MerchantType defaults to "ECommerce", TerminalID defaults to "00000001",
// CurrencyCode defaults to "$" (DOP).
type HPPConfig struct {
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

	// CurrencyCode is the default currency for transactions.
	// "$" = DOP (Dominican Peso), "USD" = US Dollar.
	// Defaults to "$". Can be overridden per request.
	CurrencyCode string

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

func (c *HPPConfig) defaults() {
	if c.MerchantType == "" {
		c.MerchantType = "ECommerce"
	}
	if c.TerminalID == "" {
		c.TerminalID = "00000001"
	}
	if c.CurrencyCode == "" {
		c.CurrencyCode = "$"
	}
}

// ============================================================================
// HPPClient
// ============================================================================

// HPPClient is the entry point for Azul Hosted Payment Page (HPP) operations.
//
// It generates form fields and HMAC signatures for browser-redirect payments.
// Use NewHPPClient to create an instance.
type HPPClient struct {
	config HPPConfig
}

// NewHPPClient creates a new HPP client with the given configuration.
func NewHPPClient(config HPPConfig) *HPPClient {
	config.defaults()
	return &HPPClient{config: config}
}

// ============================================================================
// Payment
// ============================================================================

// PaymentRequest contains the parameters needed to build the Azul payment form.
type PaymentRequest struct {
	// OrderNumber is the unique reference for this transaction (e.g. "ORD-1234").
	OrderNumber string

	// Amount is the total amount in the transaction currency (e.g. 1500.00 = RD$1,500.00).
	Amount float64

	// ITBIS is the tax amount in the transaction currency. Set to 0 if already included in Amount.
	ITBIS float64

	// CurrencyCode overrides HPPConfig.CurrencyCode for this request.
	// "$" = DOP, "USD" = US Dollar. Leave empty to use the config default.
	CurrencyCode string

	// SaveToDataVault instructs Azul to tokenize the card for future payments.
	// When true, Azul returns a DataVaultToken in the callback upon approval.
	SaveToDataVault bool

	// DataVaultToken is a previously generated token for paying without full card entry.
	// When set, Azul only asks for the CVV (security code) instead of full card details.
	DataVaultToken string
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
//	    Amount:      1500.00,  // RD$1,500.00
//	    ITBIS:       0,
//	})
//	// result.ActionURL  → POST target
//	// result.Fields     → hidden form inputs
func (c *HPPClient) BuildPaymentForm(req PaymentRequest) PaymentResult {
	amount := FormatAmount(req.Amount)
	itbis := FormatAmount(req.ITBIS)
	currency := req.CurrencyCode
	if currency == "" {
		currency = c.config.CurrencyCode
	}

	fields := map[string]string{
		"MerchantId":        c.config.MerchantID,
		"MerchantName":      c.config.MerchantName,
		"MerchantType":      c.config.MerchantType,
		"CurrencyCode":      currency,
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

	// Token-based payment: include DatavaultToken in hash fields
	if req.DataVaultToken != "" {
		fields["DatavaultToken"] = req.DataVaultToken
	}

	// Sign with the appropriate field order
	var hashUpper string
	if req.DataVaultToken != "" {
		hashUpper = signHPPTokenRequest(c.config.AuthKey, fields)
	} else {
		hashUpper = signHPPRequest(c.config.AuthKey, fields)
	}
	fields["AuthHash"] = strings.ToLower(hashUpper)

	// Extra POST fields (NOT included in hash, but required by Azul)
	fields["TerminalId"] = c.config.TerminalID
	fields["PosInputMode"] = c.config.MerchantType
	fields["CustomOrderId"] = req.OrderNumber

	// DataVault: save card for future use
	if req.SaveToDataVault {
		fields["SaveToDataVault"] = "1"
	}

	// Token payment: ensure SaveToDataVault is 0 (card already saved)
	if req.DataVaultToken != "" {
		fields["SaveToDataVault"] = "0"
	}

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

	// Amount is the original transaction amount in the transaction currency (e.g. 1500.00).
	Amount float64

	// ITBIS is the original tax amount in the transaction currency.
	ITBIS float64

	// CurrencyCode overrides HPPConfig.CurrencyCode for this request.
	// "$" = DOP, "USD" = US Dollar. Leave empty to use the config default.
	CurrencyCode string
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
//	    Amount:      1500.00,
//	    ITBIS:       0,
//	})
func (c *HPPClient) BuildVoidForm(req VoidRequest) VoidResult {
	amount := FormatAmount(req.Amount)
	itbis := FormatAmount(req.ITBIS)
	currency := req.CurrencyCode
	if currency == "" {
		currency = c.config.CurrencyCode
	}

	voidURL := c.config.VoidCallbackURL
	if voidURL == "" {
		voidURL = c.config.ApprovedURL
	}

	fields := map[string]string{
		"MerchantId":        c.config.MerchantID,
		"MerchantName":      c.config.MerchantName,
		"MerchantType":      c.config.MerchantType,
		"CurrencyCode":      currency,
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
	CardNumber          string // Masked card number returned by Azul (e.g. "554941...8810"). Use CardLastFour() to safely extract the last 4 digits.
	DataVaultToken      string
	DataVaultExpiration string // Token expiration in YYYYMM format.
	DataVaultBrand      string
	AzulOrderID         string
}

// CardLastFour returns the last 4 digits of the card number from the Azul callback.
//
// Azul returns the card number in masked form (e.g. "554941...8810").
// This method extracts only the trailing 4 digits for safe display/storage.
// Returns empty string if CardNumber has fewer than 4 characters.
func (p CallbackParams) CardLastFour() string {
	if len(p.CardNumber) < 4 {
		return ""
	}
	return p.CardNumber[len(p.CardNumber)-4:]
}

// ValidateCallback verifies the HMAC AuthHash returned by Azul to ensure the
// callback was not forged.
//
// Azul's implementation is known to vary the field order and encoding across
// versions, so this function tries multiple field combinations × 2 encodings
// (UTF-8 and UTF-16LE) for maximum compatibility.
//
// Returns true if the hash is valid (or if no hash was provided — some Azul
// configurations don't return AuthHash).
func (c *HPPClient) ValidateCallback(params CallbackParams) bool {
	m := map[string]string{
		"OrderNumber":         params.OrderNumber,
		"Amount":              params.Amount,
		"AuthorizationCode":   params.AuthorizationCode,
		"DateTime":            params.DateTime,
		"ResponseCode":        params.ResponseCode,
		"ISOCode":             params.IsoCode,
		"ResponseMessage":     params.ResponseMessage,
		"ErrorDescription":    params.ErrorDescription,
		"RRN":                 params.RRN,
		"CardNumber":          params.CardNumber,
		"DataVaultToken":      params.DataVaultToken,
		"DataVaultExpiration": params.DataVaultExpiration,
		"DataVaultBrand":      params.DataVaultBrand,
	}
	return verifyHPPReturn(c.config.AuthKey, m, params.AuthHash)
}

// IsApproved checks if the Azul callback indicates a successful payment.
//
// Azul returns IsoCode "00" for approved transactions. This is the standard
// ISO 8583 response code for "Approved".
func (c *HPPClient) IsApproved(params CallbackParams) bool {
	return params.IsoCode == "00"
}

// ============================================================================
// Private helpers
// ============================================================================

func (c *HPPClient) paymentPageURL() string {
	if strings.EqualFold(c.config.Environment, "production") {
		return HPPProductionURL
	}
	return HPPTestURL
}

func (c *HPPClient) paymentPageAltURL() string {
	if strings.EqualFold(c.config.Environment, "production") {
		return HPPProductionAltURL
	}
	return ""
}
