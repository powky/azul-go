package azul

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// Test helpers
// ============================================================================

// azulHandler creates an httptest handler that captures the request and responds
// with the given JSON response body.
func azulHandler(t *testing.T, wantMethod string, wantQuerySuffix string, responseBody string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		// Verify method
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Auth1") == "" {
			t.Error("missing Auth1 header")
		}
		if r.Header.Get("Auth2") == "" {
			t.Error("missing Auth2 header")
		}

		// Verify query string
		if wantQuerySuffix != "" {
			if r.URL.RawQuery != wantQuerySuffix {
				t.Errorf("expected query %q, got %q", wantQuerySuffix, r.URL.RawQuery)
			}
		} else {
			if r.URL.RawQuery != "" {
				t.Errorf("expected no query string, got %q", r.URL.RawQuery)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(responseBody))
	}
}

// newTestAPIClient creates an APIClient pointing to a test server.
func newTestAPIClient(t *testing.T, server *httptest.Server) *APIClient {
	t.Helper()
	return newAPIClientWithHTTP(APIConfig{
		Auth1:       "testauth1",
		Auth2:       "testauth2",
		Store:       "39099999999",
		Environment: "test",
	}, server.Client(), server.URL)
}

// parseRequestBody reads and parses the JSON request body from an http.Request.
func parseRequestBody(t *testing.T, r *http.Request) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("failed to read request body: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	return result
}

// ============================================================================
// APIConfig defaults
// ============================================================================

func TestAPIConfigDefaults(t *testing.T) {
	cfg := APIConfig{
		Auth1: "a1",
		Auth2: "a2",
		Store: "123",
	}
	cfg.defaults()

	if cfg.Channel != "EC" {
		t.Errorf("expected Channel=EC, got %q", cfg.Channel)
	}
	if cfg.PosInputMode != "E-Commerce" {
		t.Errorf("expected PosInputMode=E-Commerce, got %q", cfg.PosInputMode)
	}
	if cfg.CurrencyPosCode != "$" {
		t.Errorf("expected CurrencyPosCode=$, got %q", cfg.CurrencyPosCode)
	}
	if cfg.Timeout != 120*time.Second {
		t.Errorf("expected Timeout=120s, got %v", cfg.Timeout)
	}
}

func TestAPIConfigDefaultsNoOverride(t *testing.T) {
	cfg := APIConfig{
		Auth1:           "a1",
		Auth2:           "a2",
		Store:           "123",
		Channel:         "PP",
		PosInputMode:    "MOTO",
		CurrencyPosCode: "USD",
		Timeout:         60 * time.Second,
	}
	cfg.defaults()

	if cfg.Channel != "PP" {
		t.Errorf("Channel should not be overridden, got %q", cfg.Channel)
	}
	if cfg.PosInputMode != "MOTO" {
		t.Errorf("PosInputMode should not be overridden, got %q", cfg.PosInputMode)
	}
	if cfg.CurrencyPosCode != "USD" {
		t.Errorf("CurrencyPosCode should not be overridden, got %q", cfg.CurrencyPosCode)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout should not be overridden, got %v", cfg.Timeout)
	}
}

// ============================================================================
// NewAPIClient
// ============================================================================

func TestNewAPIClientInvalidCert(t *testing.T) {
	_, err := NewAPIClient(APIConfig{
		Auth1:    "a1",
		Auth2:    "a2",
		Store:    "123",
		CertFile: "/nonexistent/cert.pem",
		KeyFile:  "/nonexistent/key.pem",
	})
	if err == nil {
		t.Fatal("expected error for invalid certificate paths")
	}
	if !strings.Contains(err.Error(), "failed to load TLS certificate") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNewAPIClientNoCert(t *testing.T) {
	client, err := NewAPIClient(APIConfig{
		Auth1:       "a1",
		Auth2:       "a2",
		Store:       "123",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("expected no error without cert, got %v", err)
	}
	if client.primaryURL != APITestURL {
		t.Errorf("expected test URL, got %q", client.primaryURL)
	}
	if client.altURL != "" {
		t.Errorf("expected no alt URL for test, got %q", client.altURL)
	}
}

func TestNewAPIClientProduction(t *testing.T) {
	client, err := NewAPIClient(APIConfig{
		Auth1:       "a1",
		Auth2:       "a2",
		Store:       "123",
		Environment: "production",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.primaryURL != APIProductionURL {
		t.Errorf("expected production URL, got %q", client.primaryURL)
	}
	if client.altURL != APIProductionAltURL {
		t.Errorf("expected production alt URL, got %q", client.altURL)
	}
}

// ============================================================================
// APIResponse helpers
// ============================================================================

func TestAPIResponseIsApproved(t *testing.T) {
	tests := []struct {
		isoCode string
		want    bool
	}{
		{"00", true},
		{"51", false},
		{"", false},
		{"99", false},
	}
	for _, tt := range tests {
		r := &APIResponse{IsoCode: tt.isoCode}
		if got := r.IsApproved(); got != tt.want {
			t.Errorf("IsApproved() with IsoCode=%q: got %v, want %v", tt.isoCode, got, tt.want)
		}
	}
}

func TestAPIResponseWasProcessed(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"ISO8583", true},
		{"Error", false},
		{"", false},
	}
	for _, tt := range tests {
		r := &APIResponse{ResponseCode: tt.code}
		if got := r.WasProcessed(); got != tt.want {
			t.Errorf("WasProcessed() with ResponseCode=%q: got %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestAPIResponseHasError(t *testing.T) {
	tests := []struct {
		code string
		want bool
	}{
		{"Error", true},
		{"ISO8583", false},
		{"", false},
	}
	for _, tt := range tests {
		r := &APIResponse{ResponseCode: tt.code}
		if got := r.HasError(); got != tt.want {
			t.Errorf("HasError() with ResponseCode=%q: got %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestAPIResponseIsFound(t *testing.T) {
	tests := []struct {
		name  string
		found interface{}
		want  bool
	}{
		{"bool true", true, true},
		{"bool false", false, false},
		{"string true", "true", true},
		{"string false", "false", false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &APIResponse{Found: tt.found}
			if got := r.IsFound(); got != tt.want {
				t.Errorf("IsFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAPIResponseCardLastFour(t *testing.T) {
	tests := []struct {
		cardNumber string
		want       string
	}{
		{"999999******9999", "9999"},
		{"4111", "4111"},
		{"123", ""},
		{"", ""},
	}
	for _, tt := range tests {
		r := &APIResponse{CardNumber: tt.cardNumber}
		if got := r.CardLastFour(); got != tt.want {
			t.Errorf("CardLastFour(%q) = %q, want %q", tt.cardNumber, got, tt.want)
		}
	}
}

// ============================================================================
// Sale
// ============================================================================

func TestSale(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)

		// Verify no query string (ProcessPayment uses base URL)
		if r.URL.RawQuery != "" {
			t.Errorf("Sale should not have query string, got %q", r.URL.RawQuery)
		}

		// Verify Auth headers
		if r.Header.Get("Auth1") != "testauth1" {
			t.Errorf("expected Auth1=testauth1, got %q", r.Header.Get("Auth1"))
		}
		if r.Header.Get("Auth2") != "testauth2" {
			t.Errorf("expected Auth2=testauth2, got %q", r.Header.Get("Auth2"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"AuthorizationCode": "OK463C",
			"AzulOrderId":       "11350",
			"CustomOrderId":     "ABC123",
			"DateTime":          "20150206120821",
			"ErrorDescription":  "",
			"IsoCode":           "00",
			"LotNumber":         "29",
			"RRN":               "000012003029",
			"ResponseCode":      "ISO8583",
			"ResponseMessage":   "APROBADA",
			"Ticket":            "2809",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.Sale(context.Background(), SaleRequest{
		CardNumber:  "4111111111111111",
		Expiration:  "202512",
		CVC:         "123",
		Amount:      1500.00,
		ITBIS:       270.00,
		OrderNumber: "ORD-001",
		CustomOrderId: "ABC123",
	})
	if err != nil {
		t.Fatalf("Sale() error: %v", err)
	}

	// Verify response
	if !resp.IsApproved() {
		t.Error("expected approved response")
	}
	if !resp.WasProcessed() {
		t.Error("expected processed response")
	}
	if resp.AuthorizationCode != "OK463C" {
		t.Errorf("unexpected AuthorizationCode: %q", resp.AuthorizationCode)
	}

	// Verify request body
	if capturedBody["TrxType"] != "Sale" {
		t.Errorf("expected TrxType=Sale, got %v", capturedBody["TrxType"])
	}
	if capturedBody["CardNumber"] != "4111111111111111" {
		t.Errorf("unexpected CardNumber: %v", capturedBody["CardNumber"])
	}
	if capturedBody["Amount"] != "150000" {
		t.Errorf("unexpected Amount: %v", capturedBody["Amount"])
	}
	if capturedBody["Itbis"] != "27000" {
		t.Errorf("unexpected Itbis: %v", capturedBody["Itbis"])
	}
	if capturedBody["Channel"] != "EC" {
		t.Errorf("unexpected Channel: %v", capturedBody["Channel"])
	}
	if capturedBody["Store"] != "39099999999" {
		t.Errorf("unexpected Store: %v", capturedBody["Store"])
	}
	if capturedBody["CurrencyPosCode"] != "$" {
		t.Errorf("unexpected CurrencyPosCode: %v", capturedBody["CurrencyPosCode"])
	}
	if capturedBody["CustomOrderId"] != "ABC123" {
		t.Errorf("unexpected CustomOrderId: %v", capturedBody["CustomOrderId"])
	}
}

func TestSaleCurrencyOverride(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"ResponseCode": "ISO8583",
			"IsoCode":      "00",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	_, err := client.Sale(context.Background(), SaleRequest{
		CardNumber:      "4111111111111111",
		Expiration:      "202512",
		CVC:             "123",
		Amount:          100.00,
		CurrencyPosCode: "USD",
	})
	if err != nil {
		t.Fatalf("Sale() error: %v", err)
	}

	if capturedBody["CurrencyPosCode"] != "USD" {
		t.Errorf("expected CurrencyPosCode=USD, got %v", capturedBody["CurrencyPosCode"])
	}
}

func TestSaleWithSaveToDataVault(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"ResponseCode":      "ISO8583",
			"IsoCode":           "00",
			"DataVaultToken":    "6EF85D01-B07C-4E67-99F7-4E13A449DCDD",
			"DataVaultBrand":    "VISA",
			"DataVaultExpiration": "202512",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.Sale(context.Background(), SaleRequest{
		CardNumber:      "4111111111111111",
		Expiration:      "202512",
		CVC:             "123",
		Amount:          100.00,
		SaveToDataVault: true,
	})
	if err != nil {
		t.Fatalf("Sale() error: %v", err)
	}

	if capturedBody["SaveToDataVault"] != "1" {
		t.Errorf("expected SaveToDataVault=1, got %v", capturedBody["SaveToDataVault"])
	}
	if resp.DataVaultToken != "6EF85D01-B07C-4E67-99F7-4E13A449DCDD" {
		t.Errorf("unexpected DataVaultToken: %q", resp.DataVaultToken)
	}
}

func TestSaleDeclined(t *testing.T) {
	server := httptest.NewServer(azulHandler(t, "POST", "",
		`{"AuthorizationCode":"","CustomOrderId":"ABC123","DateTime":"20150206120758","ErrorDescription":"","IsoCode":"51","LotNumber":"39","RRN":"000010003194","ResponseCode":"ISO8583","ResponseMessage":"DENEGADA","Ticket":"2972"}`))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.Sale(context.Background(), SaleRequest{
		CardNumber:  "4111111111111111",
		Expiration:  "202512",
		CVC:         "123",
		Amount:      100.00,
	})
	if err != nil {
		t.Fatalf("Sale() error: %v", err)
	}

	if resp.IsApproved() {
		t.Error("declined response should not be approved")
	}
	if !resp.WasProcessed() {
		t.Error("declined response should still be processed")
	}
	if resp.ResponseMessage != "DENEGADA" {
		t.Errorf("unexpected ResponseMessage: %q", resp.ResponseMessage)
	}
}

func TestSaleError(t *testing.T) {
	server := httptest.NewServer(azulHandler(t, "POST", "",
		`{"AuthorizationCode":"","CustomOrderId":"ABC123","DateTime":"20150206120606","ErrorDescription":"MISSING_AUTH_HEADER:Auth1","IsoCode":"","LotNumber":"","RRN":"","ResponseCode":"Error","ResponseMessage":"","Ticket":""}`))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.Sale(context.Background(), SaleRequest{
		CardNumber: "4111111111111111",
		Expiration: "202512",
		CVC:        "123",
		Amount:     100.00,
	})
	if err != nil {
		t.Fatalf("Sale() error: %v", err)
	}

	if !resp.HasError() {
		t.Error("expected error response")
	}
	if resp.IsApproved() {
		t.Error("error response should not be approved")
	}
	if !strings.Contains(resp.ErrorDescription, "MISSING_AUTH_HEADER") {
		t.Errorf("unexpected ErrorDescription: %q", resp.ErrorDescription)
	}
}

// ============================================================================
// TokenSale
// ============================================================================

func TestTokenSale(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"ResponseCode":    "ISO8583",
			"IsoCode":         "00",
			"ResponseMessage": "APROBADA",
			"AzulOrderId":     "99999",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.TokenSale(context.Background(), TokenSaleRequest{
		DataVaultToken: "129BCAAB-742A-4F64-AF54-8A9F1BAD802C",
		Amount:         500.00,
		ITBIS:          0,
		OrderNumber:    "ORD-TOKEN-1",
	})
	if err != nil {
		t.Fatalf("TokenSale() error: %v", err)
	}

	if !resp.IsApproved() {
		t.Error("expected approved response")
	}

	// Verify card data is empty for token payments
	if capturedBody["CardNumber"] != "" {
		t.Errorf("expected empty CardNumber for token sale, got %v", capturedBody["CardNumber"])
	}
	if capturedBody["Expiration"] != "" {
		t.Errorf("expected empty Expiration for token sale, got %v", capturedBody["Expiration"])
	}

	// Token should be set
	if capturedBody["DataVaultToken"] != "129BCAAB-742A-4F64-AF54-8A9F1BAD802C" {
		t.Errorf("unexpected DataVaultToken: %v", capturedBody["DataVaultToken"])
	}

	// SaveToDataVault should be 0 for token payments
	if capturedBody["SaveToDataVault"] != "0" {
		t.Errorf("expected SaveToDataVault=0 for token sale, got %v", capturedBody["SaveToDataVault"])
	}

	// TrxType should still be Sale
	if capturedBody["TrxType"] != "Sale" {
		t.Errorf("expected TrxType=Sale, got %v", capturedBody["TrxType"])
	}
}

// ============================================================================
// Hold
// ============================================================================

func TestHold(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"ResponseCode": "ISO8583",
			"IsoCode":      "00",
			"AzulOrderId":  "55555",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.Hold(context.Background(), HoldRequest{
		CardNumber:  "5544332211001234",
		Expiration:  "202612",
		CVC:         "456",
		Amount:      2000.00,
		ITBIS:       360.00,
		OrderNumber: "ORD-HOLD-1",
	})
	if err != nil {
		t.Fatalf("Hold() error: %v", err)
	}

	if !resp.IsApproved() {
		t.Error("expected approved response")
	}

	if capturedBody["TrxType"] != "Hold" {
		t.Errorf("expected TrxType=Hold, got %v", capturedBody["TrxType"])
	}
	if capturedBody["Amount"] != "200000" {
		t.Errorf("unexpected Amount: %v", capturedBody["Amount"])
	}
}

// ============================================================================
// Refund
// ============================================================================

func TestRefund(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"ResponseCode": "ISO8583",
			"IsoCode":      "00",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	_, err := client.Refund(context.Background(), RefundRequest{
		CardNumber:  "4111111111111111",
		Expiration:  "202512",
		CVC:         "123",
		Amount:      300.00,
		ITBIS:       0,
		OrderNumber: "ORD-REFUND-1",
	})
	if err != nil {
		t.Fatalf("Refund() error: %v", err)
	}

	if capturedBody["TrxType"] != "Refund" {
		t.Errorf("expected TrxType=Refund, got %v", capturedBody["TrxType"])
	}
	if capturedBody["Amount"] != "30000" {
		t.Errorf("unexpected Amount: %v", capturedBody["Amount"])
	}
	// Refund should never save to DataVault
	if capturedBody["SaveToDataVault"] != "0" {
		t.Errorf("expected SaveToDataVault=0 for refund, got %v", capturedBody["SaveToDataVault"])
	}
}

// ============================================================================
// Void
// ============================================================================

func TestVoid(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)

		// Verify query string
		if r.URL.RawQuery != "ProcessVoid" {
			t.Errorf("expected query ProcessVoid, got %q", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"AuthorizationCode": "OK1695",
			"AzulOrderId":       "18539",
			"ResponseCode":      "ISO8583",
			"IsoCode":           "00",
			"ResponseMessage":   "APROBADA",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.Void(context.Background(), APIVoidRequest{
		AzulOrderId: "18527",
	})
	if err != nil {
		t.Fatalf("Void() error: %v", err)
	}

	if !resp.IsApproved() {
		t.Error("expected approved response")
	}

	// Verify request body
	if capturedBody["Channel"] != "EC" {
		t.Errorf("unexpected Channel: %v", capturedBody["Channel"])
	}
	if capturedBody["Store"] != "39099999999" {
		t.Errorf("unexpected Store: %v", capturedBody["Store"])
	}
	if capturedBody["AzulOrderId"] != "18527" {
		t.Errorf("unexpected AzulOrderId: %v", capturedBody["AzulOrderId"])
	}
}

func TestVoidAlreadyVoided(t *testing.T) {
	server := httptest.NewServer(azulHandler(t, "POST", "ProcessVoid",
		`{"AuthorizationCode":"","AzulOrderId":"","CustomOrderId":"","DataVaultBrand":"","DataVaultExpiration":"","DataVaultToken":"","ErrorDescription":"Original transaction is invalid or has already been voided","IsoCode":"","LotNumber":"","RRN":"","ResponseCode":"Error","ResponseMessage":"","Ticket":""}`))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.Void(context.Background(), APIVoidRequest{
		AzulOrderId: "18527",
	})
	if err != nil {
		t.Fatalf("Void() error: %v", err)
	}

	if !resp.HasError() {
		t.Error("expected error response for already-voided transaction")
	}
	if !strings.Contains(resp.ErrorDescription, "already been voided") {
		t.Errorf("unexpected ErrorDescription: %q", resp.ErrorDescription)
	}
}

// ============================================================================
// VerifyPayment
// ============================================================================

func TestVerifyPaymentFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query string
		if r.URL.RawQuery != "VerifyPayment" {
			t.Errorf("expected query VerifyPayment, got %q", r.URL.RawQuery)
		}

		body := parseRequestBody(t, r)
		if body["CustomOrderId"] != "ABC123" {
			t.Errorf("unexpected CustomOrderId: %v", body["CustomOrderId"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"Amount":"1000","AuthorizationCode":"OK919C","CardNumber":"999999******9999","CurrencyPosCode":"$","CustomOrderId":"ABC123","DateTime":"20150206125646","ErrorDescription":"","Found":true,"IsoCode":"00","Itbis":"00","LotNumber":"93","RRN":"000006003745","ResponseCode":"ISO8583","Ticket":"3407"}`))
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.VerifyPayment(context.Background(), VerifyRequest{
		CustomOrderId: "ABC123",
	})
	if err != nil {
		t.Fatalf("VerifyPayment() error: %v", err)
	}

	if !resp.IsFound() {
		t.Error("expected Found=true")
	}
	if !resp.IsApproved() {
		t.Error("expected approved response")
	}
	if resp.Amount != "1000" {
		t.Errorf("unexpected Amount: %q", resp.Amount)
	}
	if resp.CardNumber != "999999******9999" {
		t.Errorf("unexpected CardNumber: %q", resp.CardNumber)
	}
}

func TestVerifyPaymentNotFound(t *testing.T) {
	server := httptest.NewServer(azulHandler(t, "POST", "VerifyPayment",
		`{"Amount":null,"AuthorizationCode":"","CardNumber":null,"CurrencyPosCode":null,"CustomOrderId":"ABC124","DateTime":"20150206142626","ErrorDescription":"","Found":false,"IsoCode":"","Itbis":null,"LotNumber":"","RRN":"","ResponseCode":"","Ticket":""}`))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.VerifyPayment(context.Background(), VerifyRequest{
		CustomOrderId: "ABC124",
	})
	if err != nil {
		t.Fatalf("VerifyPayment() error: %v", err)
	}

	if resp.IsFound() {
		t.Error("expected Found=false")
	}
}

// ============================================================================
// CreateToken
// ============================================================================

func TestCreateToken(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)

		if r.URL.RawQuery != "ProcessDatavault" {
			t.Errorf("expected query ProcessDatavault, got %q", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"IsoCode":         "00",
			"DataVaultToken":  "6EF85D01-B07C-4E67-99F7-4E13A449DCDD",
			"Brand":           "VISA",
			"CardNumber":      "411111******1111",
			"Expiration":      "202512",
			"HasCVV":          true,
			"ResponseMessage": "APROBADA",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.CreateToken(context.Background(), CreateTokenRequest{
		CardNumber: "4111111111111111",
		Expiration: "202512",
		CVC:        "123",
	})
	if err != nil {
		t.Fatalf("CreateToken() error: %v", err)
	}

	if !resp.IsApproved() {
		t.Error("expected approved response")
	}
	if resp.DataVaultToken != "6EF85D01-B07C-4E67-99F7-4E13A449DCDD" {
		t.Errorf("unexpected DataVaultToken: %q", resp.DataVaultToken)
	}
	if resp.Brand != "VISA" {
		t.Errorf("unexpected Brand: %q", resp.Brand)
	}

	// Verify request body
	if capturedBody["TrxType"] != "CREATE" {
		t.Errorf("expected TrxType=CREATE, got %v", capturedBody["TrxType"])
	}
	if capturedBody["CardNumber"] != "4111111111111111" {
		t.Errorf("unexpected CardNumber: %v", capturedBody["CardNumber"])
	}
}

// ============================================================================
// DeleteToken
// ============================================================================

func TestDeleteToken(t *testing.T) {
	var capturedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody = parseRequestBody(t, r)

		if r.URL.RawQuery != "ProcessDatavault" {
			t.Errorf("expected query ProcessDatavault, got %q", r.URL.RawQuery)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"IsoCode":         "00",
			"ResponseMessage": "APROBADA",
		})
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	resp, err := client.DeleteToken(context.Background(), DeleteTokenRequest{
		DataVaultToken: "176D4B51-E562-4584-95E5-10E7ABC36BBC",
	})
	if err != nil {
		t.Fatalf("DeleteToken() error: %v", err)
	}

	if !resp.IsApproved() {
		t.Error("expected approved response")
	}

	if capturedBody["TrxType"] != "DELETE" {
		t.Errorf("expected TrxType=DELETE, got %v", capturedBody["TrxType"])
	}
	if capturedBody["DataVaultToken"] != "176D4B51-E562-4584-95E5-10E7ABC36BBC" {
		t.Errorf("unexpected DataVaultToken: %v", capturedBody["DataVaultToken"])
	}
}

// ============================================================================
// Fallback URL
// ============================================================================

func TestFallbackOnPrimaryFailure(t *testing.T) {
	primaryCalled := false
	altCalled := false

	// Primary server — always fails
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		primaryCalled = true
		// Close connection immediately to simulate network error
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("server doesn't support hijacking")
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer primaryServer.Close()

	// Alt server — returns success
	altServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		altCalled = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"ResponseCode": "ISO8583",
			"IsoCode":      "00",
		})
	}))
	defer altServer.Close()

	// Create client with primary and alt URLs
	cfg := APIConfig{
		Auth1:       "a1",
		Auth2:       "a2",
		Store:       "123",
		Environment: "test",
	}
	cfg.defaults()
	client := &APIClient{
		config:     cfg,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		primaryURL: primaryServer.URL,
		altURL:     altServer.URL,
	}

	resp, err := client.Sale(context.Background(), SaleRequest{
		CardNumber: "4111111111111111",
		Expiration: "202512",
		CVC:        "123",
		Amount:     100.00,
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	if !primaryCalled {
		t.Error("expected primary server to be called")
	}
	if !altCalled {
		t.Error("expected alt server to be called after primary failure")
	}
	if !resp.IsApproved() {
		t.Error("expected approved response from fallback")
	}
}

// ============================================================================
// Context cancellation
// ============================================================================

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"ResponseCode":"ISO8583","IsoCode":"00"}`))
	}))
	defer server.Close()

	client := newTestAPIClient(t, server)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Sale(ctx, SaleRequest{
		CardNumber: "4111111111111111",
		Expiration: "202512",
		CVC:        "123",
		Amount:     100.00,
	})
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// ============================================================================
// boolToAzul helper
// ============================================================================

func TestBoolToAzul(t *testing.T) {
	if boolToAzul(true) != "1" {
		t.Error("boolToAzul(true) should return \"1\"")
	}
	if boolToAzul(false) != "0" {
		t.Error("boolToAzul(false) should return \"0\"")
	}
}

// ============================================================================
// isProduction helper
// ============================================================================

func TestIsProduction(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"production", true},
		{"Production", true},
		{"PRODUCTION", true},
		{"test", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isProduction(tt.env); got != tt.want {
			t.Errorf("isProduction(%q) = %v, want %v", tt.env, got, tt.want)
		}
	}
}
