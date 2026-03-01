# azul-go

> **v0.3.0** — Nuevo: CreateSubscription (pagos recurrentes). Ver [Changelog](#changelog).

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
- **TokenHold**: Pre-autorización con token DataVault
- **Post**: Captura de una pre-autorización (Hold → cobro efectivo)
- **Refund**: Devolución de fondos al tarjetahabiente
- **Void**: Anulación de transacción (dentro de 20 min)
- **VerifyPayment**: Verifica el estado de una transacción anterior
- **CreateToken**: Tokeniza una tarjeta sin hacer cobro
- **DeleteToken**: Elimina un token del DataVault
- **SearchPayments**: Búsqueda de transacciones por rango de fechas
- **CreateSubscription**: Suscripciones recurrentes (cobros automáticos diarios, semanales o mensuales)
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

### Limitaciones de HPP

La Página de Pago (HPP) de Azul **no soporta** las siguientes operaciones:

- **Suscripciones (pagos recurrentes)**: Solo disponibles vía Webservices API (`CreateSubscription`)
- **Hold / Post**: Pre-autorizaciones y capturas solo vía API
- **Refund**: Devoluciones solo vía API o backoffice de Azul
- **SearchPayments**: Búsqueda de transacciones solo vía API

HPP solo soporta: pagos directos, DataVault (guardar/pagar con token) y anulaciones (Void).

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

### Hold + Post (Pre-autorización y Captura)

El flujo Hold → Post permite **reservar fondos** en la tarjeta sin cobrar inmediatamente,
y luego **capturar** (cobrar) cuando estés listo (por ejemplo, al despachar un producto).

**Reglas importantes:**
- El Post debe realizarse **antes de 7 días** del Hold, o el banco emisor lo elimina automáticamente
- El monto del Post puede ser **igual o menor** al del Hold (captura parcial)
- Si el Post es menor al Hold, Azul libera automáticamente la diferencia a la tarjeta
- El comercio **no recibe liquidación** hasta que se haga el Post

#### Paso 1: Hold (reservar fondos)

```go
holdResp, err := client.Hold(ctx, azul.HoldRequest{
    CardNumber:  "4111111111111111",
    Expiration:  "202812",
    CVC:         "123",
    Amount:      2000.00,
    ITBIS:       0,
    OrderNumber: "ORD-HOLD-1",
})
if err != nil {
    log.Fatal(err)
}
if !holdResp.IsApproved() {
    log.Fatal("Hold rechazado:", holdResp.ResponseMessage)
}

// Guardar holdResp.AzulOrderId para el Post posterior
fmt.Println("Fondos reservados, AzulOrderId:", holdResp.AzulOrderId)
```

#### Paso 2: Post (capturar el cobro)

```go
// Captura total (mismo monto del Hold)
postResp, err := client.Post(ctx, azul.PostRequest{
    AzulOrderId: holdResp.AzulOrderId, // ID del Hold original
    Amount:      2000.00,              // Igual al Hold = captura total
    ITBIS:       0,
})

// Captura parcial (menor al Hold, Azul libera la diferencia)
postResp, err := client.Post(ctx, azul.PostRequest{
    AzulOrderId: holdResp.AzulOrderId,
    Amount:      1200.00,              // Solo cobra 1200, libera 800
    ITBIS:       0,
})
```

#### Ejemplo completo: e-commerce con Hold + Post

```go
// 1. Al confirmar el pedido → Hold (reserva fondos)
holdResp, _ := client.Hold(ctx, azul.HoldRequest{
    CardNumber:  "4111111111111111",
    Expiration:  "202812",
    CVC:         "123",
    Amount:      3500.00,
    ITBIS:       630.00,
    OrderNumber: "ORD-9876",
})
// Guardar holdResp.AzulOrderId en tu base de datos

// 2. Al despachar el producto → Post (cobra efectivamente)
postResp, _ := client.Post(ctx, azul.PostRequest{
    AzulOrderId: holdResp.AzulOrderId,
    Amount:      3500.00,
    ITBIS:       630.00,
})
if postResp.IsApproved() {
    fmt.Println("Cobro realizado exitosamente")
}

// 3. Si necesitas cancelar antes del Post → Void
voidResp, _ := client.Void(ctx, azul.APIVoidRequest{
    AzulOrderId: holdResp.AzulOrderId,
})
```

#### TokenHold (Pre-autorización con token)

El mismo flujo aplica usando un token DataVault en vez de tarjeta completa:

```go
holdResp, err := client.TokenHold(ctx, azul.TokenHoldRequest{
    DataVaultToken: "6EF85D01-B07C-4E67-99F7-4E13A449DCDD",
    Amount:         2000.00,
    ITBIS:          0,
    OrderNumber:    "ORD-HOLD-TOKEN-1",
})
// Luego usar holdResp.AzulOrderId con Post() igual que arriba
```

### Void vs Refund (Anulación vs Devolución)

Void y Refund son dos formas de "devolver" dinero, pero funcionan en momentos distintos:

| | **Void** | **Refund** |
|---|---|---|
| **Cuándo** | Mismo día, antes del cierre de lote (~20 min) | Después del cierre de lote (días/semanas después) |
| **Qué hace** | Cancela la transacción como si nunca existió | Genera un nuevo movimiento de devolución al tarjetahabiente |
| **Monto** | Siempre el total | Total o parcial |
| **Datos necesarios** | Solo AzulOrderId | Tarjeta completa + AzulOrderId + fecha original |

#### ¿Cuándo usar cada uno?

**Venta directa (Sale):**
- Cliente cancela en los próximos minutos → **Void**
- Cliente pide devolución días después → **Refund**

**Hold + Post:**
- Quieres cancelar el Hold sin cobrar → **Void** del Hold (o simplemente no hacer Post y se libera en 7 días)
- Ya hiciste Post y el cliente cancela de inmediato → **Void** del Post
- Ya hiciste Post y pasaron días → **Refund**

**Hold sin Post:**
- No necesitas Refund. Un Hold sin Post no cobra al cliente. Usa **Void** para liberarlo inmediatamente, o déjalo expirar (7 días).

### Void (Anulación)

```go
// Anular una transacción reciente (Sale, Hold, o Post)
resp, err := client.Void(ctx, azul.APIVoidRequest{
    AzulOrderId: "18527", // ID de la transacción a anular
})
if resp.IsApproved() {
    fmt.Println("Transacción anulada exitosamente")
}
```

### Refund (Devolución)

Para devolver dinero después del cierre de lote. Requiere los datos de la tarjeta
y la información de la transacción original.

```go
// Devolución total
resp, err := client.Refund(ctx, azul.RefundRequest{
    CardNumber:   "4111111111111111",
    Expiration:   "202812",
    CVC:          "123",
    Amount:       1500.00,             // Monto total original
    ITBIS:        270.00,
    OriginalDate: "20250115",          // Fecha de la transacción original (YYYYMMDD)
    AzulOrderId:  "11350",             // AzulOrderId de la transacción original
    OrderNumber:  "ORD-REFUND-1",
})
```

```go
// Devolución parcial (solo parte del monto)
resp, err := client.Refund(ctx, azul.RefundRequest{
    CardNumber:   "4111111111111111",
    Expiration:   "202812",
    CVC:          "123",
    Amount:       500.00,              // Solo devuelve 500 de los 1500 originales
    ITBIS:        90.00,
    OriginalDate: "20250115",
    AzulOrderId:  "11350",
    OrderNumber:  "ORD-REFUND-2",
})
```

#### Ejemplo completo: ciclo de vida con tarjeta

```go
// 1. Venta directa
saleResp, _ := client.Sale(ctx, azul.SaleRequest{
    CardNumber:  "4111111111111111",
    Expiration:  "202812",
    CVC:         "123",
    Amount:      2000.00,
    ITBIS:       360.00,
    OrderNumber: "ORD-500",
})
// Guardar saleResp.AzulOrderId y saleResp.DateTime

// 2a. Si el cliente cancela en los próximos minutos → Void
voidResp, _ := client.Void(ctx, azul.APIVoidRequest{
    AzulOrderId: saleResp.AzulOrderId,
})

// 2b. Si el cliente pide devolución días después → Refund
refundResp, _ := client.Refund(ctx, azul.RefundRequest{
    CardNumber:   "4111111111111111",
    Expiration:   "202812",
    CVC:          "123",
    Amount:       2000.00,
    ITBIS:        360.00,
    OriginalDate: "20250115",              // Fecha del Sale original
    AzulOrderId:  saleResp.AzulOrderId,    // ID del Sale original
    OrderNumber:  "ORD-REFUND-500",
})
```

#### Ejemplo completo: ciclo de vida con token

```go
// 1. Cobro con token (cliente recurrente)
saleResp, _ := client.TokenSale(ctx, azul.TokenSaleRequest{
    DataVaultToken: "6EF85D01-B07C-4E67-99F7-4E13A449DCDD",
    Amount:         2000.00,
    ITBIS:          360.00,
    OrderNumber:    "ORD-TOKEN-500",
    CustomOrderId:  "PEDIDO-500",
})
// Guardar saleResp.AzulOrderId

// 2a. Cancelar de inmediato → Void (igual que con tarjeta, solo necesita AzulOrderId)
voidResp, _ := client.Void(ctx, azul.APIVoidRequest{
    AzulOrderId: saleResp.AzulOrderId,
})

// 2b. Devolución días después → Refund
// NOTA: Refund siempre necesita los datos de la tarjeta original,
// incluso si el cobro se hizo con token. El token no sirve para Refund.
refundResp, _ := client.Refund(ctx, azul.RefundRequest{
    CardNumber:   "4111111111111111",    // Tarjeta original del token
    Expiration:   "202812",
    CVC:          "123",
    Amount:       2000.00,
    ITBIS:        360.00,
    OriginalDate: "20250120",
    AzulOrderId:  saleResp.AzulOrderId,
    OrderNumber:  "ORD-REFUND-TOKEN-500",
})
```

> **Nota importante sobre Refund con tokens:** El endpoint de Refund de Azul requiere
> los datos de la tarjeta física (CardNumber, Expiration, CVC), no acepta DataVaultToken.
> Si cobraste con token y necesitas hacer Refund, debes tener guardados los datos de la
> tarjeta original o usar el backoffice de Azul.

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

### SearchPayments (Búsqueda de transacciones)

```go
resp, err := client.SearchPayments(ctx, azul.SearchRequest{
    DateFrom: "20250101", // YYYYMMDD
    DateTo:   "20250131",
})
if err != nil {
    log.Fatal(err)
}
for _, tx := range resp.Transactions {
    fmt.Printf("Orden: %s, Monto: %s, Tipo: %s, ISO: %s\n",
        tx.AzulOrderId, tx.Amount, tx.TransactionType, tx.IsoCode)
}
```

> **Nota:** `SearchPayments` retorna `*SearchResponse` (con campo `Transactions []SearchTransaction`)
> en vez de `*APIResponse`, ya que la estructura de respuesta de Azul es diferente.

### CreateSubscription (Pagos recurrentes)

Crea suscripciones de cobro automático vía `recurringsubscriptioncreate`.

> **Diferencias importantes con otros métodos:**
> - El monto es decimal: `50.00` = cincuenta pesos (NO centavos como Sale/Hold)
> - La moneda es `"DOP"` (NO `"$"` como otros métodos)
> - La expiración del campo es `CardExpiration` (NO `Expiration`)
> - Azul puede requerir credenciales Auth1/Auth2 distintas para suscripciones en producción

#### Suscripción diaria

```go
resp, err := client.CreateSubscription(ctx, azul.SubscriptionRequest{
    CardNumber:     "5426111111111979",
    CardExpiration: "202512",           // Formato YYYYMM
    CVC:            "123",
    Amount:         50.00,              // Decimal (NO centavos)
    ITBIS:          0,
    Frequency:      "Daily",
    EveryXDays:     "2",                // Cada 2 días
    Month:          "7",
    StartDate:      "2025-7-27",        // Formato YYYY-M-D
    MaxRepeats:     "12",               // 12 cobros (vacío = ilimitado)
    CustomerName:       "Juan Pérez",
    CustomerContract:   "WEB1234",
    CustomerIdentType:  "Cedula",
    CustomerIdentNum:   "00100204566",
})
if resp.WasCreated() {
    fmt.Println("Suscripción creada, próximo cobro:", resp.NextScheduledDate)
}
```

#### Suscripción semanal

```go
resp, err := client.CreateSubscription(ctx, azul.SubscriptionRequest{
    CardNumber:     "5426111111111979",
    CardExpiration: "202512",
    CVC:            "123",
    Amount:         100.00,
    ITBIS:          0,
    Frequency:      "Weekly",
    EveryXWeeks:    "1",              // Cada semana
    Weekdays:       "3",              // Miércoles (1=Lunes ... 7=Domingo)
    Month:          "7",
    StartDate:      "2025-7-27",
    CustomerName:       "María López",
    CustomerContract:   "WEB5678",
    CustomerIdentType:  "Cedula",
    CustomerIdentNum:   "00100204566",
})
```

#### Suscripción mensual

```go
resp, err := client.CreateSubscription(ctx, azul.SubscriptionRequest{
    CardNumber:     "5426111111111979",
    CardExpiration: "202512",
    CVC:            "123",
    Amount:         500.00,
    ITBIS:          90.00,
    Frequency:       "MonthlyByDay",
    EveryXMonths:    "1",             // Cada mes
    DayOfMonth:      "15",            // Día 15 del mes
    Month:           "7",
    StartDate:       "2025-7-15",
    CustomerName:        "Carlos García",
    CustomerContract:    "PREMIUM-001",
    CustomerIdentType:   "Cedula",
    CustomerIdentNum:    "00100204566",
    CustomerEmail:       "carlos@example.com",
    NotifyTransactions:  true,
    NotifyExpired:       true,
    SaveToDataVault:     true,
    Description:         "Plan Premium mensual",
})
```

#### Con tarjetas de respaldo

```go
resp, err := client.CreateSubscription(ctx, azul.SubscriptionRequest{
    CardNumber:     "5426111111111979",
    CardExpiration: "202512",
    CVC:            "123",
    Amount:         200.00,
    ITBIS:          0,
    Frequency:      "MonthlyByDay",
    EveryXMonths:   "1",
    DayOfMonth:     "1",
    Month:          "1",
    StartDate:      "2025-1-1",
    CustomerName:       "Ana Martínez",
    CustomerContract:   "RESPALDO-001",
    CustomerIdentType:  "Cedula",
    CustomerIdentNum:   "00100204566",
    // Azul intenta Card2 si la primaria falla, luego Card3
    Card2Number:     "4111111111111111",
    Card2Expiration: "2512",            // Formato MMYY
    Card3Number:     "4012888888881881",
    Card3Expiration: "2612",
})
```

> **Nota:** `CreateSubscription` retorna `*SubscriptionResponse` (con `CustomSubscriptionId`,
> `NextScheduledDate`, `ResponseCode`). Usa `resp.WasCreated()` para verificar éxito
> (`ResponseCode == "CREATED"`).

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

70 tests: 26 HPP + 44 API.

---

## Estructura del proyecto

```
azul-go/
├── common.go          # FormatAmount, ParseAmount, GenerateOrderNumber
├── hpp.go             # HPPConfig, HPPClient, BuildPaymentForm, BuildVoidForm,
│                      # ValidateCallback, IsApproved, CallbackParams
├── hmac.go            # HMAC-SHA512 signing/verification (solo HPP)
├── api.go             # APIConfig, APIClient, NewAPIClient, APIResponse, doRequest
├── api_payment.go     # Sale, TokenSale, Hold, TokenHold, Refund
├── api_post.go        # Post (captura de Hold)
├── api_void.go        # Void
├── api_verify.go      # VerifyPayment
├── api_datavault.go   # CreateToken, DeleteToken
├── api_search.go      # SearchPayments
├── api_subscription.go # CreateSubscription (pagos recurrentes)
├── hpp_test.go        # 26 tests HPP
├── api_test.go        # 44 tests API
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

### v0.3.0

- `CreateSubscription`: Suscripciones de pagos recurrentes vía `recurringsubscriptioncreate`
  - Soporte para frecuencia diaria, semanal y mensual
  - Tarjetas de respaldo (Card2, Card3) con fallback automático por Azul
  - Notificaciones por email (transacciones, expiración, próxima expiración)
  - Tokenización (SaveToDataVault) durante creación de suscripción
- 7 tests nuevos (70 tests totales)

### v0.2.0

- `Post`: Captura de pre-autorizaciones (Hold → cobro efectivo) vía ProcessPost
- `TokenHold`: Pre-autorización con token DataVault (sin datos de tarjeta)
- `SearchPayments`: Búsqueda de transacciones por rango de fechas
- 9 tests nuevos (63 tests totales)

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
