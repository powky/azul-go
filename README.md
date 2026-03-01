# azul-go

> **v0.1.0** — Breaking changes desde v0.0.2. Ver [Changelog](#changelog).

Cliente Go para **Azul** — la pasarela de pago de República Dominicana.

Dos modos de integración:

- **HPPClient**: Página de Pago Alojada (redirect del browser con formularios HTML + HMAC)
- **APIClient**: Webservices API (server-to-server vía JSON + TLS mutual auth)

## Qué hace esta librería

### HPP (Página de Pago)
- Genera los campos del formulario HTML + el AuthHash HMAC-SHA512 para redirigir a Azul
- Valida el hash de retorno cuando Azul redirige de vuelta (previene fraude)
- Genera los campos para anulaciones (VOID) vía redirect
- Soporta DataVault: guardar tarjetas tokenizadas y pagar con tokens

### API (Webservices server-to-server)
- **Sale**: Cobro directo con tarjeta completa
- **TokenSale**: Cobro con token DataVault (sin datos de tarjeta)
- **Hold**: Pre-autorización (reserva el monto sin capturar)
- **Refund**: Devolución de fondos al tarjetahabiente
- **Void**: Anulación de transacción (dentro de 20 min)
- **VerifyPayment**: Verifica el estado de una transacción anterior
- **CreateToken**: Tokeniza una tarjeta sin hacer cobro
- **DeleteToken**: Elimina un token del DataVault
- Fallback automático al URL secundario en producción
- TLS mutual authentication con certificados de Azul

### Compartido
- Formato de montos: `FormatAmount(1500.00)` → `"150000"`, `ParseAmount("150000")` → `1500.00`
- Generación de números de orden únicos
- Soporte multi-moneda (`"$"` DOP, `"USD"`)

## Qué NO hace

- No persiste datos (tú decides dónde guardar tokens, órdenes, etc.)
- No maneja lógica de negocio (stock, emails, etc.)

---

## Instalación

```bash
go get github.com/powky/azul-go
```

Sin dependencias externas — solo stdlib de Go.

---

## HPP Client (Página de Pago)

### Configuración

```go
client := azul.NewHPPClient(azul.HPPConfig{
    MerchantID:   os.Getenv("AZUL_MERCHANT_ID"),
    AuthKey:      os.Getenv("AZUL_AUTH_KEY"),
    MerchantName: "Mi Tienda",
    MerchantType: "ECommerce",  // default "ECommerce"
    TerminalID:   "00000001",   // default "00000001"
    CurrencyCode: "$",          // default "$" (DOP)
    Environment:  "test",       // "test" o "production"
    ApprovedURL:  "https://mitienda.com/pago/aprobado",
    DeclinedURL:  "https://mitienda.com/pago/rechazado",
    CancelURL:    "https://mitienda.com/pago/cancelado",
})
```

### Generar formulario de pago

```go
result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber: azul.GenerateOrderNumber("ORD"),
    Amount:      1500.00,
    ITBIS:       0,
})

// result.ActionURL  → URL del POST
// result.Fields     → map[string]string con todos los campos hidden
```

### Validar callback

```go
params := azul.CallbackParams{
    OrderNumber:       r.FormValue("OrderNumber"),
    Amount:            r.FormValue("Amount"),
    AuthorizationCode: r.FormValue("AuthorizationCode"),
    DateTime:          r.FormValue("DateTime"),
    ResponseCode:      r.FormValue("ResponseCode"),
    IsoCode:           r.FormValue("ISOCode"),
    ResponseMessage:   r.FormValue("ResponseMessage"),
    ErrorDescription:  r.FormValue("ErrorDescription"),
    RRN:               r.FormValue("RRN"),
    AuthHash:          r.FormValue("AuthHash"),
    CardNumber:        r.FormValue("CardNumber"),
    DataVaultToken:    r.FormValue("DataVaultToken"),
    AzulOrderID:       r.FormValue("AzulOrderId"),
}

if !client.ValidateCallback(params) {
    http.Error(w, "invalid callback", http.StatusForbidden)
    return
}

if client.IsApproved(params) {
    fmt.Println("Aprobado!", params.AuthorizationCode)
    fmt.Println("Tarjeta: ****", params.CardLastFour())
}
```

### DataVault vía HPP

```go
// Guardar tarjeta durante el pago
result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber:     "ORD-1234",
    Amount:          1500.00,
    SaveToDataVault: true,
})

// Pagar con token guardado (Azul solo pide CVV)
result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber:    "ORD-5678",
    Amount:         2000.00,
    DataVaultToken: "FE1525FD-A59B-476A-9EFA-387D510689AB",
})
```

### Anulación (VOID) vía HPP

```go
result := client.BuildVoidForm(azul.VoidRequest{
    OrderNumber: "ORD-1234",
    AzulOrderID: "abc-def-123",
    Amount:      1500.00,
    ITBIS:       0,
})
```

---

## API Client (Webservices server-to-server)

### Configuración

```go
client, err := azul.NewAPIClient(azul.APIConfig{
    Auth1:       os.Getenv("AZUL_AUTH1"),
    Auth2:       os.Getenv("AZUL_AUTH2"),
    CertFile:    "/path/to/azul-cert.pem",  // Certificado TLS emitido por Azul
    KeyFile:     "/path/to/azul-key.pem",
    Store:       os.Getenv("AZUL_STORE"),   // MID asignado por Azul
    Environment: "test",                     // "test" o "production"
    // Opcionales:
    Channel:              "EC",              // default "EC"
    PosInputMode:         "E-Commerce",      // default "E-Commerce"
    CurrencyPosCode:      "$",               // default "$" (DOP)
    CustomerServicePhone: "809-555-1234",
    ECommerceURL:         "https://mitienda.com",
    AltMerchantName:      "MI TIENDA SRL",
})
if err != nil {
    log.Fatal(err)
}
```

### Sale (Venta directa)

```go
resp, err := client.Sale(ctx, azul.SaleRequest{
    CardNumber:    "4111111111111111",
    Expiration:    "202812",
    CVC:           "123",
    Amount:        1500.00,
    ITBIS:         270.00,
    OrderNumber:   "ORD-001",
    CustomOrderId: "ABC123",
})
if err != nil {
    log.Fatal(err)
}

if resp.IsApproved() {
    fmt.Println("Aprobado! AzulOrderId:", resp.AzulOrderId)
} else if resp.HasError() {
    fmt.Println("Error:", resp.ErrorDescription)
} else {
    fmt.Println("Rechazado:", resp.ResponseMessage, "IsoCode:", resp.IsoCode)
}
```

### Sale con DataVault (guardar tarjeta)

```go
resp, err := client.Sale(ctx, azul.SaleRequest{
    CardNumber:      "4111111111111111",
    Expiration:      "202812",
    CVC:             "123",
    Amount:          1500.00,
    ITBIS:           0,
    OrderNumber:     "ORD-002",
    SaveToDataVault: true,
})
if resp.IsApproved() {
    token := resp.DataVaultToken // Guardar para futuros cobros
}
```

### TokenSale (Cobro con token)

```go
resp, err := client.TokenSale(ctx, azul.TokenSaleRequest{
    DataVaultToken: "6EF85D01-B07C-4E67-99F7-4E13A449DCDD",
    Amount:         500.00,
    ITBIS:          0,
    OrderNumber:    "ORD-003",
    CustomOrderId:  "RECURRENTE-001",
})
```

### Hold (Pre-autorización)

```go
resp, err := client.Hold(ctx, azul.HoldRequest{
    CardNumber:  "4111111111111111",
    Expiration:  "202812",
    CVC:         "123",
    Amount:      2000.00,
    ITBIS:       0,
    OrderNumber: "ORD-HOLD-1",
})
```

### Void (Anulación)

```go
resp, err := client.Void(ctx, azul.APIVoidRequest{
    AzulOrderId: "18527",
})
```

### Refund (Devolución)

```go
resp, err := client.Refund(ctx, azul.RefundRequest{
    CardNumber:  "4111111111111111",
    Expiration:  "202812",
    CVC:         "123",
    Amount:      300.00,
    ITBIS:       0,
    OrderNumber: "ORD-REFUND-1",
})
```

### VerifyPayment

```go
resp, err := client.VerifyPayment(ctx, azul.VerifyRequest{
    CustomOrderId: "ABC123",
})
if resp.IsFound() && resp.IsApproved() {
    fmt.Println("Transacción encontrada y aprobada")
}
```

### CreateToken (sin cobro)

```go
resp, err := client.CreateToken(ctx, azul.CreateTokenRequest{
    CardNumber: "4111111111111111",
    Expiration: "202512",
    CVC:        "123",
})
if resp.IsApproved() {
    token := resp.DataVaultToken
    brand := resp.Brand
}
```

### DeleteToken

```go
resp, err := client.DeleteToken(ctx, azul.DeleteTokenRequest{
    DataVaultToken: "6EF85D01-B07C-4E67-99F7-4E13A449DCDD",
})
```

---

## APIResponse

Todos los métodos del APIClient retornan `*APIResponse`:

```go
type APIResponse struct {
    AuthorizationCode   string
    AzulOrderId         string
    CustomOrderId       string
    DateTime            string
    ErrorDescription    string
    IsoCode             string
    LotNumber           string
    RRN                 string
    ResponseCode        string
    ResponseMessage     string
    Ticket              string
    CardNumber          string
    DataVaultToken      string
    DataVaultBrand      string
    DataVaultExpiration string
    // + campos específicos de VerifyPayment y DataVault
}
```

Helpers:
- `resp.IsApproved()` → `IsoCode == "00"`
- `resp.WasProcessed()` → `ResponseCode == "ISO8583"` (el banco respondió)
- `resp.HasError()` → `ResponseCode == "Error"` (no se procesó)
- `resp.IsFound()` → para VerifyPayment
- `resp.CardLastFour()` → últimos 4 dígitos

---

## URLs de Azul

### HPP (Página de Pago)

| Entorno | URL principal | URL alternativa |
|---------|--------------|----------------|
| `test` | `pruebas.azul.com.do/PaymentPage/` | — |
| `production` | `pagos.azul.com.do/PaymentPage/Default.aspx` | `contpagos.azul.com.do/PaymentPage/Default.aspx` |

### API (Webservices JSON)

| Entorno | URL principal | URL alternativa |
|---------|--------------|----------------|
| `test` | `pruebas.azul.com.do/WebServices/JSON/Default.aspx` | — |
| `production` | `pagos.azul.com.do/WebServices/JSON/Default.aspx` | `contpagos.azul.com.do/WebServices/JSON/Default.aspx` |

El APIClient hace fallback automático al URL secundario si el primario falla (solo en producción).

---

## Formato de montos

```go
azul.FormatAmount(0)       // → "000"
azul.FormatAmount(0.50)    // → "050"
azul.FormatAmount(15.00)   // → "1500"
azul.FormatAmount(1500.00) // → "150000"

azul.ParseAmount("150000") // → 1500.00
```

---

## Códigos ISO 8583 comunes

| `IsoCode` | Significado |
|-----------|-------------|
| `00` | Aprobada |
| `05` | No honrar |
| `12` | Transacción inválida |
| `13` | Monto inválido |
| `14` | Número de tarjeta inválido |
| `51` | Fondos insuficientes |
| `54` | Tarjeta expirada |
| `61` | Excede límite de retiro |
| `91` | Emisor no disponible |

---

## Tests

```bash
go test ./... -v
```

54 tests: 26 HPP + 28 API.

---

## Estructura del proyecto

```
azul-go/
├── common.go          # FormatAmount, ParseAmount, GenerateOrderNumber
├── hpp.go             # HPPConfig, HPPClient, BuildPaymentForm, BuildVoidForm,
│                      # ValidateCallback, IsApproved, CallbackParams
├── hmac.go            # HMAC-SHA512 signing/verification (solo HPP)
├── api.go             # APIConfig, APIClient, NewAPIClient, APIResponse, doRequest
├── api_payment.go     # Sale, TokenSale, Hold, Refund
├── api_void.go        # Void
├── api_verify.go      # VerifyPayment
├── api_datavault.go   # CreateToken, DeleteToken
├── hpp_test.go        # 26 tests HPP
├── api_test.go        # 28 tests API
├── go.mod
└── README.md
```

---

## Breaking changes en v0.1.0

Si vienes de v0.0.2, estos son los cambios necesarios:

```go
// Antes (v0.0.2)
client := azul.NewClient(azul.Config{...})

// Después (v0.1.0)
client := azul.NewHPPClient(azul.HPPConfig{...})
```

| v0.0.2 | v0.1.0 |
|--------|--------|
| `azul.Config` | `azul.HPPConfig` |
| `azul.NewClient()` | `azul.NewHPPClient()` |
| `*azul.Client` | `*azul.HPPClient` |
| `azul.TestURL` | `azul.HPPTestURL` (alias `TestURL` aún funciona) |
| `azul.ProductionURL` | `azul.HPPProductionURL` (alias `ProductionURL` aún funciona) |
| `azul.ProductionAltURL` | `azul.HPPProductionAltURL` (alias `ProductionAltURL` aún funciona) |

Las constantes `TestURL`, `ProductionURL`, `ProductionAltURL` siguen disponibles como aliases deprecated.

---

## Changelog

### v0.1.0

- **BREAKING**: `Config` → `HPPConfig`, `Client` → `HPPClient`, `NewClient()` → `NewHPPClient()`
- **APIClient**: Nuevo cliente server-to-server para Azul Webservices JSON API
  - `Sale`: Cobro con tarjeta completa
  - `TokenSale`: Cobro con token DataVault
  - `Hold`: Pre-autorización
  - `Refund`: Devolución
  - `Void`: Anulación vía ProcessVoid
  - `VerifyPayment`: Verificar estado de transacción
  - `CreateToken`: Tokenizar tarjeta sin cobro
  - `DeleteToken`: Eliminar token del DataVault
- **TLS mutual auth**: Soporte de certificados cliente emitidos por Azul
- **Fallback automático**: Si el URL primario falla, intenta el secundario (producción)
- **Context support**: Todos los métodos API aceptan `context.Context`
- Utilidades compartidas extraídas a `common.go`
- 28 tests nuevos para API (54 tests totales)

### v0.0.2

- DataVault: Soporte para guardar tarjetas y pagar con tokens vía HPP
- CardLastFour(): Últimos 4 dígitos del número enmascarado
- Token payment hash con orden de campos correcto
- DataVault callback validation

### v0.0.1

- Release inicial: HPP payment form, VOID, callback validation, HMAC-SHA512

---

## Licencia

MIT
