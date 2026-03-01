package azul

import (
	"strings"
	"testing"
)

func testClient() *Client {
	return NewClient(Config{
		MerchantID:   "39038540035",
		AuthKey:      "test-secret-key-12345",
		MerchantName: "Test Store",
		MerchantType: "ECommerce",
		TerminalID:   "00000001",
		Environment:  "test",
		ApprovedURL:  "https://myapp.com/approved",
		DeclinedURL:  "https://myapp.com/declined",
		CancelURL:    "https://myapp.com/cancel",
	})
}

// ============================================================================
// Config defaults
// ============================================================================

func TestConfigDefaults(t *testing.T) {
	c := NewClient(Config{
		MerchantID:  "123",
		AuthKey:     "key",
		Environment: "test",
	})
	if c.config.MerchantType != "ECommerce" {
		t.Errorf("expected MerchantType=ECommerce, got %q", c.config.MerchantType)
	}
	if c.config.TerminalID != "00000001" {
		t.Errorf("expected TerminalID=00000001, got %q", c.config.TerminalID)
	}
	if c.config.CurrencyCode != "$" {
		t.Errorf("expected CurrencyCode=$, got %q", c.config.CurrencyCode)
	}
}

// ============================================================================
// BuildPaymentForm
// ============================================================================

func TestBuildPaymentForm(t *testing.T) {
	client := testClient()

	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber: "ORD-123",
		Amount:      1500.00, // RD$1,500.00
		ITBIS:       0,
	})

	// Test URL
	if result.ActionURL != TestURL {
		t.Errorf("expected test URL, got %q", result.ActionURL)
	}
	if result.AltActionURL != "" {
		t.Errorf("expected empty alt URL for test, got %q", result.AltActionURL)
	}

	// Test required fields
	requiredFields := []string{
		"MerchantId", "MerchantName", "MerchantType", "CurrencyCode",
		"OrderNumber", "Amount", "ITBIS", "ApprovedUrl", "DeclinedUrl",
		"CancelUrl", "AuthHash", "TerminalId", "PosInputMode", "CustomOrderId",
	}
	for _, f := range requiredFields {
		if _, ok := result.Fields[f]; !ok {
			t.Errorf("missing field %q", f)
		}
	}

	// Test values
	if result.Fields["MerchantId"] != "39038540035" {
		t.Errorf("unexpected MerchantId: %q", result.Fields["MerchantId"])
	}
	if result.Fields["Amount"] != "150000" {
		t.Errorf("unexpected Amount: %q", result.Fields["Amount"])
	}
	if result.Fields["ITBIS"] != "000" {
		t.Errorf("unexpected ITBIS: %q", result.Fields["ITBIS"])
	}
	if result.Fields["OrderNumber"] != "ORD-123" {
		t.Errorf("unexpected OrderNumber: %q", result.Fields["OrderNumber"])
	}
	if result.Fields["CurrencyCode"] != "$" {
		t.Errorf("unexpected CurrencyCode: %q", result.Fields["CurrencyCode"])
	}

	// AuthHash should be lowercase hex
	hash := result.Fields["AuthHash"]
	if hash == "" {
		t.Fatal("AuthHash is empty")
	}
	if hash != strings.ToLower(hash) {
		t.Errorf("AuthHash should be lowercase, got %q", hash)
	}
}

func TestBuildPaymentFormProduction(t *testing.T) {
	client := NewClient(Config{
		MerchantID:  "123",
		AuthKey:     "key",
		Environment: "production",
		ApprovedURL: "https://myapp.com/ok",
		DeclinedURL: "https://myapp.com/fail",
		CancelURL:   "https://myapp.com/cancel",
	})

	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber: "ORD-1",
		Amount:      10.00,
	})

	if result.ActionURL != ProductionURL {
		t.Errorf("expected production URL, got %q", result.ActionURL)
	}
	if result.AltActionURL != ProductionAltURL {
		t.Errorf("expected production alt URL, got %q", result.AltActionURL)
	}
}

func TestBuildPaymentFormCurrencyOverride(t *testing.T) {
	client := testClient() // default CurrencyCode = "$"

	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber:  "ORD-USD-1",
		Amount:       50.00,
		CurrencyCode: "USD",
	})

	if result.Fields["CurrencyCode"] != "USD" {
		t.Errorf("expected CurrencyCode=USD, got %q", result.Fields["CurrencyCode"])
	}
}

func TestBuildPaymentFormConfigCurrency(t *testing.T) {
	client := NewClient(Config{
		MerchantID:   "123",
		AuthKey:      "key",
		Environment:  "test",
		CurrencyCode: "USD",
		ApprovedURL:  "https://myapp.com/ok",
		DeclinedURL:  "https://myapp.com/fail",
		CancelURL:    "https://myapp.com/cancel",
	})

	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber: "ORD-1",
		Amount:      25.00,
	})

	if result.Fields["CurrencyCode"] != "USD" {
		t.Errorf("expected CurrencyCode=USD from config, got %q", result.Fields["CurrencyCode"])
	}
}

// ============================================================================
// BuildVoidForm
// ============================================================================

func TestBuildVoidForm(t *testing.T) {
	client := testClient()

	result := client.BuildVoidForm(VoidRequest{
		OrderNumber: "ORD-123",
		AzulOrderID: "azul-txn-456",
		Amount:      1500.00,
		ITBIS:       0,
	})

	if result.Fields["TrxType"] != "Void" {
		t.Errorf("expected TrxType=Void, got %q", result.Fields["TrxType"])
	}
	if result.Fields["AzulOrderId"] != "azul-txn-456" {
		t.Errorf("unexpected AzulOrderId: %q", result.Fields["AzulOrderId"])
	}
	if result.Fields["Amount"] != "150000" {
		t.Errorf("unexpected Amount: %q", result.Fields["Amount"])
	}

	// Void uses ApprovedURL as fallback when VoidCallbackURL is empty
	if result.Fields["ApprovedUrl"] != "https://myapp.com/approved" {
		t.Errorf("unexpected void ApprovedUrl: %q", result.Fields["ApprovedUrl"])
	}
}

func TestBuildVoidFormWithCallback(t *testing.T) {
	client := NewClient(Config{
		MerchantID:      "123",
		AuthKey:         "key",
		Environment:     "test",
		ApprovedURL:     "https://myapp.com/ok",
		DeclinedURL:     "https://myapp.com/fail",
		CancelURL:       "https://myapp.com/cancel",
		VoidCallbackURL: "https://myapp.com/void-callback",
	})

	result := client.BuildVoidForm(VoidRequest{
		OrderNumber: "ORD-1",
		AzulOrderID: "azul-1",
		Amount:      10.00,
	})

	// Should use VoidCallbackURL for all three URLs
	if result.Fields["ApprovedUrl"] != "https://myapp.com/void-callback" {
		t.Errorf("expected VoidCallbackURL, got %q", result.Fields["ApprovedUrl"])
	}
	if result.Fields["DeclinedUrl"] != "https://myapp.com/void-callback" {
		t.Errorf("expected VoidCallbackURL for declined, got %q", result.Fields["DeclinedUrl"])
	}
}

func TestBuildVoidFormCurrencyOverride(t *testing.T) {
	client := testClient()

	result := client.BuildVoidForm(VoidRequest{
		OrderNumber:  "ORD-1",
		AzulOrderID:  "azul-1",
		Amount:       100.00,
		CurrencyCode: "USD",
	})

	if result.Fields["CurrencyCode"] != "USD" {
		t.Errorf("expected CurrencyCode=USD, got %q", result.Fields["CurrencyCode"])
	}
}

// ============================================================================
// ValidateCallback + IsApproved
// ============================================================================

func TestValidateCallbackNoHash(t *testing.T) {
	client := testClient()

	// No hash provided → should return true (trust by ResponseCode)
	ok := client.ValidateCallback(CallbackParams{
		OrderNumber: "ORD-123",
		IsoCode:     "00",
	})
	if !ok {
		t.Error("expected ValidateCallback to return true when no hash provided")
	}
}

func TestValidateCallbackWithValidHash(t *testing.T) {
	client := testClient()

	// Build a payment to get valid fields
	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber: "ORD-HASH-TEST",
		Amount:      500.00,
		ITBIS:       0,
	})

	// Simulate Azul callback — compute hash the same way Azul would
	params := CallbackParams{
		OrderNumber:       result.Fields["OrderNumber"],
		Amount:            result.Fields["Amount"],
		AuthorizationCode: "123456",
		DateTime:          "20250226120000",
		ResponseCode:      "ISO8583",
		IsoCode:           "00",
		ResponseMessage:   "APROBADA",
		ErrorDescription:  "",
		RRN:               "999888777",
	}

	// Compute what Azul would send as AuthHash (using full field order)
	m := map[string]string{
		"OrderNumber": params.OrderNumber, "Amount": params.Amount,
		"AuthorizationCode": params.AuthorizationCode, "DateTime": params.DateTime,
		"ResponseCode": params.ResponseCode, "ISOCode": params.IsoCode,
		"ResponseMessage": params.ResponseMessage, "ErrorDescription": params.ErrorDescription,
		"RRN": params.RRN,
	}
	var sb strings.Builder
	for _, k := range []string{"OrderNumber", "Amount", "AuthorizationCode", "DateTime", "ResponseCode", "ISOCode", "ResponseMessage", "ErrorDescription", "RRN"} {
		sb.WriteString(m[k])
	}
	sb.WriteString(strings.TrimSpace(client.config.AuthKey))
	params.AuthHash = strings.ToLower(hmacSHA512Upper(client.config.AuthKey, []byte(sb.String())))

	if !client.ValidateCallback(params) {
		t.Error("expected ValidateCallback to return true for valid hash")
	}
}

func TestValidateCallbackInvalidHash(t *testing.T) {
	client := testClient()

	ok := client.ValidateCallback(CallbackParams{
		OrderNumber: "ORD-123",
		Amount:      "50000",
		IsoCode:     "00",
		AuthHash:    "deadbeef1234567890invalid",
	})
	if ok {
		t.Error("expected ValidateCallback to return false for invalid hash")
	}
}

func TestIsApproved(t *testing.T) {
	client := testClient()

	approved := CallbackParams{IsoCode: "00"}
	declined := CallbackParams{IsoCode: "51"}
	empty := CallbackParams{}

	if !client.IsApproved(approved) {
		t.Error("IsoCode 00 should be approved")
	}
	if client.IsApproved(declined) {
		t.Error("IsoCode 51 should NOT be approved")
	}
	if client.IsApproved(empty) {
		t.Error("empty IsoCode should NOT be approved")
	}
}

// ============================================================================
// FormatAmount / ParseAmount
// ============================================================================

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		amount   float64
		expected string
	}{
		{0, "000"},
		{0.01, "001"},
		{0.50, "050"},
		{0.99, "099"},
		{1.00, "100"},
		{15.00, "1500"},
		{1500.00, "150000"},
		{10000.00, "1000000"},
		{1234.56, "123456"},
		{0.10, "010"},
	}
	for _, tt := range tests {
		got := FormatAmount(tt.amount)
		if got != tt.expected {
			t.Errorf("FormatAmount(%v) = %q, want %q", tt.amount, got, tt.expected)
		}
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"000", 0},
		{"050", 0.50},
		{"1500", 15.00},
		{"150000", 1500.00},
		{"123456", 1234.56},
		{"", 0},
		{"abc", 0},
	}
	for _, tt := range tests {
		got := ParseAmount(tt.input)
		if got != tt.expected {
			t.Errorf("ParseAmount(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

// FormatAmount and ParseAmount should be inverses
func TestFormatParseRoundTrip(t *testing.T) {
	amounts := []float64{0, 0.01, 0.50, 1.00, 15.00, 1500.00, 99999.99}
	for _, amount := range amounts {
		got := ParseAmount(FormatAmount(amount))
		if got != amount {
			t.Errorf("round trip failed for %v: got %v", amount, got)
		}
	}
}

// ============================================================================
// GenerateOrderNumber
// ============================================================================

func TestGenerateOrderNumber(t *testing.T) {
	on := GenerateOrderNumber("ORD")
	if !strings.HasPrefix(on, "ORD-") {
		t.Errorf("expected prefix ORD-, got %q", on)
	}
	if len(on) < 10 {
		t.Errorf("order number too short: %q", on)
	}
}

// ============================================================================
// CardLastFour
// ============================================================================

func TestCardLastFour(t *testing.T) {
	tests := []struct {
		cardNumber string
		expected   string
	}{
		{"554941...8810", "8810"},
		{"414746*****0117", "0117"},
		{"1234", "1234"},
		{"123", ""},  // too short
		{"", ""},     // empty
		{"4111111111111111", "1111"},
	}
	for _, tt := range tests {
		p := CallbackParams{CardNumber: tt.cardNumber}
		got := p.CardLastFour()
		if got != tt.expected {
			t.Errorf("CardLastFour(%q) = %q, want %q", tt.cardNumber, got, tt.expected)
		}
	}
}

// ============================================================================
// BuildPaymentForm with DataVault
// ============================================================================

func TestBuildPaymentFormWithSaveToDataVault(t *testing.T) {
	client := testClient()

	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber:     "ORD-DV-1",
		Amount:          1500.00,
		ITBIS:           0,
		SaveToDataVault: true,
	})

	if result.Fields["SaveToDataVault"] != "1" {
		t.Errorf("expected SaveToDataVault=1, got %q", result.Fields["SaveToDataVault"])
	}

	// AuthHash should still be present and lowercase
	hash := result.Fields["AuthHash"]
	if hash == "" {
		t.Fatal("AuthHash is empty")
	}
	if hash != strings.ToLower(hash) {
		t.Errorf("AuthHash should be lowercase, got %q", hash)
	}

	// Hash should be identical to a payment without SaveToDataVault (not in hash)
	resultNoVault := client.BuildPaymentForm(PaymentRequest{
		OrderNumber: "ORD-DV-1",
		Amount:      1500.00,
		ITBIS:       0,
	})
	if result.Fields["AuthHash"] != resultNoVault.Fields["AuthHash"] {
		t.Error("SaveToDataVault should NOT affect the AuthHash")
	}
}

func TestBuildPaymentFormWithDataVaultToken(t *testing.T) {
	client := testClient()
	token := "FE1525FD-A59B-476A-9EFA-387D510689AB"

	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber:    "ORD-TOKEN-1",
		Amount:         500.00,
		ITBIS:          0,
		DataVaultToken: token,
	})

	// Token should be in form fields
	if result.Fields["DatavaultToken"] != token {
		t.Errorf("expected DatavaultToken=%q, got %q", token, result.Fields["DatavaultToken"])
	}

	// SaveToDataVault should be forced to 0 for token payments
	if result.Fields["SaveToDataVault"] != "0" {
		t.Errorf("expected SaveToDataVault=0 for token payment, got %q", result.Fields["SaveToDataVault"])
	}

	// Hash should be different from a payment without token
	resultNoToken := client.BuildPaymentForm(PaymentRequest{
		OrderNumber: "ORD-TOKEN-1",
		Amount:      500.00,
		ITBIS:       0,
	})
	if result.Fields["AuthHash"] == resultNoToken.Fields["AuthHash"] {
		t.Error("token payment should produce a different AuthHash than standard payment")
	}

	// AuthHash should be lowercase
	if result.Fields["AuthHash"] != strings.ToLower(result.Fields["AuthHash"]) {
		t.Errorf("AuthHash should be lowercase, got %q", result.Fields["AuthHash"])
	}
}

func TestBuildPaymentFormTokenOverridesSaveToDataVault(t *testing.T) {
	client := testClient()

	// Even if SaveToDataVault is true, DataVaultToken should force it to 0
	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber:     "ORD-1",
		Amount:          100.00,
		SaveToDataVault: true,
		DataVaultToken:  "SOME-TOKEN",
	})

	if result.Fields["SaveToDataVault"] != "0" {
		t.Errorf("DataVaultToken should override SaveToDataVault to 0, got %q", result.Fields["SaveToDataVault"])
	}
}

// ============================================================================
// ValidateCallback with DataVault
// ============================================================================

func TestValidateCallbackDataVaultHash(t *testing.T) {
	client := testClient()

	// Simulate a DataVault callback with the DataVault hash format
	params := CallbackParams{
		IsoCode:             "00",
		ResponseMessage:     "APROBADA",
		ErrorDescription:    "",
		CardNumber:          "554941...8810",
		DataVaultToken:      "734016F2-00E5-4611-948C-8179FB2B8C74",
		DataVaultExpiration: "201912",
		DataVaultBrand:      "MASTERCARD",
	}

	// Compute DataVault hash: ISOCode + ResponseMessage + ErrorDescription + CardNumber + DataVaultToken + DataVaultExpiration + DataVaultBrand + AuthKey
	var sb strings.Builder
	for _, v := range []string{params.IsoCode, params.ResponseMessage, params.ErrorDescription, params.CardNumber, params.DataVaultToken, params.DataVaultExpiration, params.DataVaultBrand} {
		sb.WriteString(v)
	}
	sb.WriteString(strings.TrimSpace(client.config.AuthKey))
	params.AuthHash = strings.ToLower(hmacSHA512Upper(client.config.AuthKey, []byte(sb.String())))

	if !client.ValidateCallback(params) {
		t.Error("expected ValidateCallback to return true for valid DataVault hash")
	}
}

func TestValidateCallbackStandardStillWorks(t *testing.T) {
	client := testClient()

	// Standard callback (same as existing test) should still validate
	result := client.BuildPaymentForm(PaymentRequest{
		OrderNumber: "ORD-STD",
		Amount:      500.00,
		ITBIS:       0,
	})

	params := CallbackParams{
		OrderNumber:       result.Fields["OrderNumber"],
		Amount:            result.Fields["Amount"],
		AuthorizationCode: "123456",
		DateTime:          "20250226120000",
		ResponseCode:      "ISO8583",
		IsoCode:           "00",
		ResponseMessage:   "APROBADA",
		ErrorDescription:  "",
		RRN:               "999888777",
	}

	// Compute standard hash
	m := map[string]string{
		"OrderNumber": params.OrderNumber, "Amount": params.Amount,
		"AuthorizationCode": params.AuthorizationCode, "DateTime": params.DateTime,
		"ResponseCode": params.ResponseCode, "ISOCode": params.IsoCode,
		"ResponseMessage": params.ResponseMessage, "ErrorDescription": params.ErrorDescription,
		"RRN": params.RRN,
	}
	var sb strings.Builder
	for _, k := range []string{"OrderNumber", "Amount", "AuthorizationCode", "DateTime", "ResponseCode", "ISOCode", "ResponseMessage", "ErrorDescription", "RRN"} {
		sb.WriteString(m[k])
	}
	sb.WriteString(strings.TrimSpace(client.config.AuthKey))
	params.AuthHash = strings.ToLower(hmacSHA512Upper(client.config.AuthKey, []byte(sb.String())))

	if !client.ValidateCallback(params) {
		t.Error("standard callback should still validate after DataVault changes")
	}
}

// ============================================================================
// HMAC internals
// ============================================================================

func TestToUTF16LE(t *testing.T) {
	// "AB" in UTF-16LE is: 0x41 0x00 0x42 0x00
	result := toUTF16LE("AB")
	expected := []byte{0x41, 0x00, 0x42, 0x00}
	if len(result) != len(expected) {
		t.Fatalf("length mismatch: got %d, want %d", len(result), len(expected))
	}
	for i := range expected {
		if result[i] != expected[i] {
			t.Errorf("byte %d: got 0x%02x, want 0x%02x", i, result[i], expected[i])
		}
	}
}

func TestHMACConsistency(t *testing.T) {
	// Same input should always produce the same hash
	h1 := hmacSHA512Upper("secret", []byte("data"))
	h2 := hmacSHA512Upper("secret", []byte("data"))
	if h1 != h2 {
		t.Error("HMAC should be deterministic")
	}
	if h1 != strings.ToUpper(h1) {
		t.Error("hmacSHA512Upper should return uppercase")
	}
}

func TestSignPaymentDeterministic(t *testing.T) {
	fields := map[string]string{
		"MerchantId": "123", "MerchantName": "Test", "MerchantType": "ECommerce",
		"CurrencyCode": "$", "OrderNumber": "ORD-1", "Amount": "1000", "ITBIS": "000",
		"ApprovedUrl": "https://ok", "DeclinedUrl": "https://fail", "CancelUrl": "https://cancel",
		"UseCustomField1": "0", "CustomField1Label": "", "CustomField1Value": "",
		"UseCustomField2": "0", "CustomField2Label": "", "CustomField2Value": "",
	}

	h1 := signHPPRequest("secret", fields)
	h2 := signHPPRequest("secret", fields)
	if h1 != h2 {
		t.Error("signHPPRequest should be deterministic")
	}
}

func TestSignTokenPaymentDeterministic(t *testing.T) {
	fields := map[string]string{
		"MerchantId": "123", "MerchantName": "Test", "MerchantType": "ECommerce",
		"CurrencyCode": "$", "OrderNumber": "ORD-1", "Amount": "1000", "ITBIS": "000",
		"ApprovedUrl": "https://ok", "DatavaultToken": "TOKEN-123", "DeclinedUrl": "https://fail", "CancelUrl": "https://cancel",
		"UseCustomField1": "0", "CustomField1Label": "", "CustomField1Value": "",
		"UseCustomField2": "0", "CustomField2Label": "", "CustomField2Value": "",
	}

	h1 := signHPPTokenRequest("secret", fields)
	h2 := signHPPTokenRequest("secret", fields)
	if h1 != h2 {
		t.Error("signHPPTokenRequest should be deterministic")
	}

	// Token hash should differ from standard hash (different field order)
	h3 := signHPPRequest("secret", fields)
	if h1 == h3 {
		t.Error("token payment hash should differ from standard payment hash")
	}
}
