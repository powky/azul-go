package azul

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ============================================================================
// API URLs (Webservices JSON)
// ============================================================================

const (
	APITestURL          = "https://pruebas.azul.com.do/WebServices/JSON/Default.aspx"
	APIProductionURL    = "https://pagos.azul.com.do/WebServices/JSON/Default.aspx"
	APIProductionAltURL = "https://contpagos.azul.com.do/WebServices/JSON/Default.aspx"
)

// ============================================================================
// APIConfig
// ============================================================================

// APIConfig holds all Azul Webservices API configuration.
//
// Required fields: Auth1, Auth2, Store.
// For TLS mutual authentication: CertFile and KeyFile (paths to PEM files).
// Optional: Channel defaults to "EC", PosInputMode defaults to "E-Commerce",
// CurrencyPosCode defaults to "$" (DOP), Timeout defaults to 120s.
type APIConfig struct {
	// Auth1 is the first authentication factor (header "Auth1").
	Auth1 string

	// Auth2 is the second authentication factor (header "Auth2").
	Auth2 string

	// CertFile is the path to the TLS client certificate PEM file issued by Azul.
	// Required for mutual TLS authentication.
	CertFile string

	// KeyFile is the path to the TLS client certificate private key PEM file.
	// Required for mutual TLS authentication.
	KeyFile string

	// Store is the merchant identifier (MID) assigned by Azul (e.g. "39099999999").
	Store string

	// Channel is the payment channel. Defaults to "EC" (E-Commerce).
	Channel string

	// PosInputMode identifies the input mode. Defaults to "E-Commerce".
	PosInputMode string

	// CurrencyPosCode is the default currency for transactions.
	// "$" = DOP (Dominican Peso), "USD" = US Dollar.
	// Defaults to "$".
	CurrencyPosCode string

	// Environment selects the Azul endpoint: "test" or "production".
	Environment string

	// Timeout is the HTTP request timeout. Azul recommends 120 seconds.
	// Defaults to 120s if zero.
	Timeout time.Duration

	// CustomerServicePhone is the merchant's customer service phone number.
	// Optional, included in transactions if set.
	CustomerServicePhone string

	// ECommerceURL is the merchant's website URL.
	// Optional, included in transactions if set.
	ECommerceURL string

	// AltMerchantName is an alternate merchant name for the cardholder's statement.
	// Optional, max 25 characters.
	AltMerchantName string
}

func (c *APIConfig) defaults() {
	if c.Channel == "" {
		c.Channel = "EC"
	}
	if c.PosInputMode == "" {
		c.PosInputMode = "E-Commerce"
	}
	if c.CurrencyPosCode == "" {
		c.CurrencyPosCode = "$"
	}
	if c.Timeout == 0 {
		c.Timeout = 120 * time.Second
	}
}

// ============================================================================
// APIClient
// ============================================================================

// APIClient is the entry point for Azul Webservices API (server-to-server) operations.
//
// It sends JSON POST requests directly to Azul's endpoints using TLS mutual
// authentication. No browser redirect is involved.
//
// Use NewAPIClient to create an instance.
type APIClient struct {
	config     APIConfig
	httpClient *http.Client
	primaryURL string
	altURL     string
}

// NewAPIClient creates a new API client with TLS mutual authentication.
//
// It loads the client certificate from the configured CertFile and KeyFile paths.
// Returns an error if the certificate cannot be loaded.
//
// If CertFile and KeyFile are empty, a client without mutual TLS is created
// (useful for testing with httptest).
func NewAPIClient(config APIConfig) (*APIClient, error) {
	config.defaults()

	var tlsConfig *tls.Config

	if config.CertFile != "" && config.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("azul: failed to load TLS certificate: %w", err)
		}
		tlsConfig = &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
	}

	transport := &http.Transport{}
	if tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}

	client := &APIClient{
		config: config,
		httpClient: &http.Client{
			Timeout:   config.Timeout,
			Transport: transport,
		},
	}

	if isProduction(config.Environment) {
		client.primaryURL = APIProductionURL
		client.altURL = APIProductionAltURL
	} else {
		client.primaryURL = APITestURL
		client.altURL = ""
	}

	return client, nil
}

// newAPIClientWithHTTP creates an API client with a custom http.Client (for testing).
func newAPIClientWithHTTP(config APIConfig, httpClient *http.Client, baseURL string) *APIClient {
	config.defaults()
	return &APIClient{
		config:     config,
		httpClient: httpClient,
		primaryURL: baseURL,
		altURL:     "",
	}
}

// ============================================================================
// APIResponse
// ============================================================================

// APIResponse represents the JSON response from Azul's Webservices API.
//
// All methods (ProcessPayment, ProcessVoid, VerifyPayment, ProcessDatavault)
// return this same response structure, though not all fields are populated
// for every method.
type APIResponse struct {
	AuthorizationCode   string `json:"AuthorizationCode"`
	AzulOrderId         string `json:"AzulOrderId"`
	CustomOrderId       string `json:"CustomOrderId"`
	DateTime            string `json:"DateTime"`
	ErrorDescription    string `json:"ErrorDescription"`
	IsoCode             string `json:"IsoCode"`
	LotNumber           string `json:"LotNumber"`
	RRN                 string `json:"RRN"`
	ResponseCode        string `json:"ResponseCode"`
	ResponseMessage     string `json:"ResponseMessage"`
	Ticket              string `json:"Ticket"`
	CardNumber          string `json:"CardNumber"`
	DataVaultToken      string `json:"DataVaultToken"`
	DataVaultBrand      string `json:"DataVaultBrand"`
	DataVaultExpiration string `json:"DataVaultExpiration"`

	// VerifyPayment-specific fields
	Found           interface{} `json:"Found,omitempty"`
	Amount          string      `json:"Amount,omitempty"`
	Itbis           string      `json:"Itbis,omitempty"`
	CurrencyPosCode string      `json:"CurrencyPosCode,omitempty"`

	// DataVault-specific fields
	Brand      string      `json:"Brand,omitempty"`
	Expiration string      `json:"Expiration,omitempty"`
	HasCVV     interface{} `json:"HasCVV,omitempty"`
}

// IsApproved returns true if the transaction was approved (IsoCode "00").
func (r *APIResponse) IsApproved() bool {
	return r.IsoCode == "00"
}

// WasProcessed returns true if Azul processed the transaction (ResponseCode "ISO8583").
// Check IsoCode to determine if it was approved or declined.
func (r *APIResponse) WasProcessed() bool {
	return r.ResponseCode == "ISO8583"
}

// HasError returns true if Azul could not process the transaction (ResponseCode "Error").
// Check ErrorDescription for details.
func (r *APIResponse) HasError() bool {
	return r.ResponseCode == "Error"
}

// IsFound returns true if VerifyPayment found the transaction.
// Handles both bool and string representations from Azul's API.
func (r *APIResponse) IsFound() bool {
	switch v := r.Found.(type) {
	case bool:
		return v
	case string:
		return v == "true"
	default:
		return false
	}
}

// CardLastFour returns the last 4 digits of the masked card number.
// Returns empty string if CardNumber has fewer than 4 characters.
func (r *APIResponse) CardLastFour() string {
	if len(r.CardNumber) < 4 {
		return ""
	}
	return r.CardNumber[len(r.CardNumber)-4:]
}

// ============================================================================
// Internal HTTP helpers
// ============================================================================

// doRequest sends a JSON POST to Azul's API with fallback on network error.
//
// queryMethod is appended as a query parameter (e.g. "?ProcessVoid").
// An empty queryMethod sends to the base URL (ProcessPayment).
func (c *APIClient) doRequest(ctx context.Context, queryMethod string, payload interface{}) (*APIResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("azul: failed to marshal request: %w", err)
	}

	url := c.primaryURL
	if queryMethod != "" {
		url += "?" + queryMethod
	}

	resp, err := c.post(ctx, url, body)
	if err != nil && c.altURL != "" {
		// Fallback to alternate URL on network error (production only)
		altURL := c.altURL
		if queryMethod != "" {
			altURL += "?" + queryMethod
		}
		resp, err = c.post(ctx, altURL, body)
	}

	return resp, err
}

func (c *APIClient) post(ctx context.Context, url string, body []byte) (*APIResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("azul: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Auth1", c.config.Auth1)
	req.Header.Set("Auth2", c.config.Auth2)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azul: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("azul: failed to read response: %w", err)
	}

	var result APIResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("azul: failed to decode response: %w", err)
	}

	return &result, nil
}

// ============================================================================
// Shared helpers
// ============================================================================

func isProduction(env string) bool {
	return env == "production" || env == "Production" || env == "PRODUCTION"
}

func boolToAzul(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
