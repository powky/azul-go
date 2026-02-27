# azul-go

Cliente Go para **Azul Payment Page (HPP)** — la pasarela de pago estándar de Azul en República Dominicana.

## Qué hace esta librería

- Genera los campos del formulario HTML + el AuthHash HMAC-SHA512 para redirigir al usuario a Azul
- Valida el hash de retorno cuando Azul redirige de vuelta a tu app (previene fraude)
- Genera los campos del formulario para anular (VOID) una transacción
- Convierte montos entre formato legible (1500.00) y formato Azul ("150000")
- Soporta múltiples monedas ("$" para DOP, "USD" para dólares)
- Genera números de orden únicos

## Qué NO hace

- No hace requests HTTP (tú manejas el formulario/redirect)
- No maneja lógica de negocio (órdenes, stock, emails, etc.)
- No persiste datos (tú decides dónde guardar)

## Instalación

```bash
go get github.com/ianfdev/azul-go
```

## Inicio rápido

```go
package main

import (
    "fmt"
    "os"

    azul "github.com/ianfdev/azul-go"
)

func main() {
    // 1. Crear cliente
    client := azul.NewClient(azul.Config{
        MerchantID:   os.Getenv("AZUL_MERCHANT_ID"),
        AuthKey:      os.Getenv("AZUL_AUTH_KEY"),
        MerchantName: "Mi Tienda",
        MerchantType: "ECommerce",  // opcional, default "ECommerce"
        TerminalID:   "00000001",   // opcional, default "00000001"
        CurrencyCode: "$",          // opcional, default "$" (DOP). Usa "USD" para dólares
        Environment:  "test",       // "test" o "production"
        ApprovedURL:  "https://mitienda.com/pago/aprobado",
        DeclinedURL:  "https://mitienda.com/pago/rechazado",
        CancelURL:    "https://mitienda.com/pago/cancelado",
    })

    // 2. Generar formulario de pago
    result := client.BuildPaymentForm(azul.PaymentRequest{
        OrderNumber: azul.GenerateOrderNumber("ORD"),
        Amount:      1500.00, // RD$1,500.00
        ITBIS:       0,       // 0 si ya está incluido en Amount
    })

    fmt.Println("POST a:", result.ActionURL)
    fmt.Println("Campos:", result.Fields)
}
```

---

## Configuración

### `azul.Config`

| Campo | Tipo | Requerido | Default | Descripción |
|-------|------|-----------|---------|-------------|
| `MerchantID` | `string` | ✅ | — | ID del comercio asignado por Azul (ej: `"39038540035"`) |
| `AuthKey` | `string` | ✅ | — | Clave HMAC secreta proporcionada por Azul |
| `MerchantName` | `string` | ✅ | — | Nombre que aparece en la página de pago de Azul |
| `MerchantType` | `string` | — | `"ECommerce"` | Tipo de integración |
| `TerminalID` | `string` | — | `"00000001"` | Terminal asignada por Azul |
| `CurrencyCode` | `string` | — | `"$"` | Moneda por defecto: `"$"` (DOP) o `"USD"` |
| `Environment` | `string` | ✅ | — | `"test"` o `"production"` |
| `ApprovedURL` | `string` | ✅ | — | URL de redirección tras pago exitoso |
| `DeclinedURL` | `string` | ✅ | — | URL de redirección tras pago rechazado |
| `CancelURL` | `string` | ✅ | — | URL de redirección si el usuario cancela |
| `VoidCallbackURL` | `string` | — | `ApprovedURL` | URL de redirección tras VOID |

### Variables de entorno recomendadas

```env
AZUL_MERCHANT_ID=39038540035
AZUL_AUTH_KEY=tu-clave-secreta
AZUL_MERCHANT_NAME=Mi Tienda
AZUL_MERCHANT_TYPE=ECommerce
AZUL_TERMINAL_ID=00000001
AZUL_CURRENCY_CODE=$
AZUL_ENVIRONMENT=test
AZUL_APPROVED_URL=https://mitienda.com/pago/aprobado
AZUL_DECLINED_URL=https://mitienda.com/pago/rechazado
AZUL_CANCEL_URL=https://mitienda.com/pago/cancelado
AZUL_VOID_CALLBACK_URL=https://mitienda.com/pago/void
```

---

## Flujo completo de pago

### Paso 1: Generar formulario de pago

```go
result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber: "ORD-1234",
    Amount:      2500.00, // RD$2,500.00
    ITBIS:       0,
})

// result.ActionURL    → URL del POST (Azul Payment Page)
// result.AltActionURL → URL de fallback (solo producción, vacío en test)
// result.Fields       → map[string]string con TODOS los campos del form
```

Para cobrar en dólares, pasa `CurrencyCode` en el request:

```go
result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber:  "ORD-1234",
    Amount:       50.00, // USD$50.00
    ITBIS:        0,
    CurrencyCode: "USD", // override del default "$" (DOP)
})
```

Tu frontend debe crear un formulario HTML con action=`result.ActionURL` y un `<input type="hidden">` por cada entrada en `result.Fields`:

```html
<form method="POST" action="{{ .ActionURL }}">
  {{ range $key, $value := .Fields }}
    <input type="hidden" name="{{ $key }}" value="{{ $value }}">
  {{ end }}
  <button type="submit">Pagar con Azul</button>
</form>
```

O si tu frontend es una SPA (React, Vue, etc.), tu API devuelve el JSON y el frontend construye el form dinámicamente:

```json
{
  "action_url": "https://pruebas.azul.com.do/PaymentPage/",
  "alt_action_url": "",
  "fields": {
    "MerchantId": "39038540035",
    "Amount": "250000",
    "CurrencyCode": "$",
    "AuthHash": "a1b2c3...",
    "...": "..."
  }
}
```

### Paso 2: Azul procesa el pago

El usuario completa el pago en la página de Azul. Azul redirige de vuelta a tu `ApprovedURL` o `DeclinedURL` con parámetros en el query string.

### Paso 3: Validar el callback

Cuando Azul redirige de vuelta, tu frontend captura los query params y los envía a tu backend:

```go
// Parsear los parámetros del callback (de query string o JSON body)
params := azul.CallbackParams{
    OrderNumber:       r.FormValue("OrderNumber"),
    Amount:            r.FormValue("Amount"),
    ITBIS:             r.FormValue("ITBIS"),
    AuthorizationCode: r.FormValue("AuthorizationCode"),
    DateTime:          r.FormValue("DateTime"),
    ResponseCode:      r.FormValue("ResponseCode"),
    IsoCode:           r.FormValue("ISOCode"),
    ResponseMessage:   r.FormValue("ResponseMessage"),
    ErrorDescription:  r.FormValue("ErrorDescription"),
    RRN:               r.FormValue("RRN"),
    AuthHash:          r.FormValue("AuthHash"),
    AzulOrderID:       r.FormValue("AzulOrderId"),
}

// Validar que el hash es legítimo (previene callbacks forjados)
if !client.ValidateCallback(params) {
    // HASH INVÁLIDO — posible fraude, rechazar
    http.Error(w, "invalid callback", http.StatusForbidden)
    return
}

// Convertir el monto de vuelta a número legible
amount := azul.ParseAmount(params.Amount) // "250000" → 2500.00

// Verificar si el pago fue aprobado
if client.IsApproved(params) {
    // PAGO EXITOSO
    // - Actualizar orden como "pagada"
    // - Descontar stock
    // - Enviar email de confirmación
    fmt.Println("Pago aprobado! AuthCode:", params.AuthorizationCode)
    fmt.Printf("Monto cobrado: $%.2f\n", amount)
} else {
    // PAGO RECHAZADO
    fmt.Println("Rechazado. Código:", params.IsoCode, "Mensaje:", params.ResponseMessage)
}
```

---

## Anulaciones (VOID)

Para anular una transacción, necesitas el `AzulOrderID` del pago original:

```go
result := client.BuildVoidForm(azul.VoidRequest{
    OrderNumber: "ORD-1234",        // tu referencia original
    AzulOrderID: "abc-def-123",     // de params.AzulOrderID del pago original
    Amount:      2500.00,           // monto original
    ITBIS:       0,                 // ITBIS original
})

// El frontend hace POST igual que con el pago
// Azul redirige a VoidCallbackURL con el resultado
// Validar igual con ValidateCallback + IsApproved
```

> **Nota:** Azul permite VOIDs solo dentro de un plazo limitado después del pago (generalmente el mismo día). Tu app debe verificar la ventana de tiempo antes de intentar el VOID.

---

## Funciones utilitarias

### `azul.GenerateOrderNumber(prefix)`

Genera un número de orden único basado en timestamp:

```go
azul.GenerateOrderNumber("ORD")  // → "ORD-1709234567890"
azul.GenerateOrderNumber("LPJ")  // → "LPJ-1709234567890"
azul.GenerateOrderNumber("INV")  // → "INV-1709234567890"
```

### `azul.FormatAmount(amount)`

Convierte un monto monetario al formato string que espera Azul (sin separador decimal, mínimo 3 chars):

```go
azul.FormatAmount(0)       // → "000"
azul.FormatAmount(0.50)    // → "050"
azul.FormatAmount(15.00)   // → "1500"
azul.FormatAmount(1500.00) // → "150000"
azul.FormatAmount(1234.56) // → "123456"
```

### `azul.ParseAmount(s)`

Inverso de FormatAmount — convierte string de Azul a monto monetario:

```go
azul.ParseAmount("000")    // → 0.00
azul.ParseAmount("050")    // → 0.50
azul.ParseAmount("1500")   // → 15.00
azul.ParseAmount("150000") // → 1500.00
azul.ParseAmount("123456") // → 1234.56
```

---

## Monedas

La librería soporta múltiples monedas. La moneda se puede configurar a nivel global (Config) o por transacción:

### Moneda por defecto en Config

```go
// Comercio que solo opera en pesos
client := azul.NewClient(azul.Config{
    CurrencyCode: "$",   // DOP — este es el default si no lo pasas
    // ...
})

// Comercio que solo opera en dólares
client := azul.NewClient(azul.Config{
    CurrencyCode: "USD",
    // ...
})
```

### Override por transacción

```go
// Cliente configurado en pesos, pero esta orden es en dólares
result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber:  "ORD-USD-1",
    Amount:       50.00,
    CurrencyCode: "USD", // override solo para esta transacción
})
```

| CurrencyCode | Moneda |
|---|---|
| `"$"` | Peso Dominicano (DOP) — default |
| `"USD"` | Dólar Estadounidense |

---

## URLs de Azul

La librería selecciona automáticamente según `Environment`:

| Entorno | URL principal | URL alternativa |
|---------|--------------|----------------|
| `test` | `https://pruebas.azul.com.do/PaymentPage/` | — |
| `production` | `https://pagos.azul.com.do/PaymentPage/Default.aspx` | `https://contpagos.azul.com.do/PaymentPage/Default.aspx` |

La URL alternativa es un fallback que Azul proporciona por si el servidor principal está caído. Tu frontend debería intentar con `AltActionURL` si el POST a `ActionURL` falla (solo en producción).

---

## Sobre el HMAC AuthHash

### Request (tu app → Azul)

Los campos se concatenan en este orden exacto + la AuthKey, y se firma con HMAC-SHA512:

```
MerchantId + MerchantName + MerchantType + CurrencyCode + OrderNumber +
Amount + ITBIS + ApprovedUrl + DeclinedUrl + CancelUrl +
UseCustomField1 + CustomField1Label + CustomField1Value +
UseCustomField2 + CustomField2Label + CustomField2Value + AuthKey
```

El hash se envía en el campo `AuthHash` del formulario en **lowercase hex**.

### Response (Azul → tu app)

Azul devuelve un `AuthHash` en el callback. Esta librería lo valida intentando **8 combinaciones**:

- **4 órdenes de campos** (con/sin DateTime, con/sin ErrorDescription)
- **2 encodings** (UTF-8 y UTF-16LE)

Esto es necesario porque la implementación de Azul varía entre versiones y la referencia PHP usa `mb_convert_encoding($str, 'UTF-16LE', 'ASCII')`.

Si Azul no devuelve AuthHash (string vacío), `ValidateCallback` retorna `true` — se confía en el ResponseCode.

---

## Campos del callback de Azul

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `OrderNumber` | `string` | Tu número de orden original |
| `Amount` | `string` | Monto en formato Azul (usa `ParseAmount` para convertir) |
| `ITBIS` | `string` | Impuesto en formato Azul (usa `ParseAmount` para convertir) |
| `AuthorizationCode` | `string` | Código de autorización del banco |
| `DateTime` | `string` | Fecha/hora de la transacción |
| `ResponseCode` | `string` | `"ISO8583"` si el banco respondió |
| `IsoCode` | `string` | `"00"` = aprobado, otros = rechazado |
| `ResponseMessage` | `string` | Mensaje legible (ej: `"APROBADA"`) |
| `ErrorDescription` | `string` | Descripción del error (vacío si aprobado) |
| `RRN` | `string` | Reference Retrieval Number |
| `AuthHash` | `string` | HMAC para validación |
| `AzulOrderID` | `string` | ID de transacción de Azul (necesario para VOIDs) |
| `DataVaultToken` | `string` | Token de tarjeta tokenizada (si aplica) |
| `DataVaultBrand` | `string` | Marca de la tarjeta (Visa, Mastercard, etc.) |

### Códigos ISO 8583 comunes

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
| `65` | Excede límite de frecuencia |
| `91` | Emisor no disponible |

---

## Modo dual (test + producción)

Puedes crear dos clientes para manejar pruebas y producción simultáneamente:

```go
prodClient := azul.NewClient(azul.Config{
    MerchantID:  os.Getenv("AZUL_MERCHANT_ID"),
    AuthKey:     os.Getenv("AZUL_AUTH_KEY"),
    Environment: "production",
    // ...
})

testClient := azul.NewClient(azul.Config{
    MerchantID:  os.Getenv("AZUL_TEST_MERCHANT_ID"),
    AuthKey:     os.Getenv("AZUL_TEST_AUTH_KEY"),
    Environment: "test",
    // ...
})

// Seleccionar cliente según el usuario
func azulForUser(email string) *azul.Client {
    if isTestEmail(email) {
        return testClient
    }
    return prodClient
}
```

---

## Manejo del ITBIS

Azul espera el campo ITBIS por separado, pero hay dos estrategias:

### Opción A: ITBIS incluido en Amount (recomendado)

```go
result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber: "ORD-1",
    Amount:      1180.00, // RD$1,180.00 (incluye ITBIS)
    ITBIS:       0,       // enviar 0 porque ya está incluido
})
```

### Opción B: ITBIS separado

```go
subtotal := 1000.00 // RD$1,000.00
itbis := 180.00     // 18%
total := subtotal + itbis

result := client.BuildPaymentForm(azul.PaymentRequest{
    OrderNumber: "ORD-1",
    Amount:      total, // RD$1,180.00
    ITBIS:       itbis, // RD$180.00
})
```

Ambas funcionan. La opción A es más simple si ya calculas el total con ITBIS incluido en tu backend.

---

## Tests

```bash
go test ./... -v
```

---

## Estructura del proyecto

```
azul-go/
├── azul.go        # Client, Config, BuildPaymentForm, BuildVoidForm,
│                  # ValidateCallback, IsApproved, FormatAmount, ParseAmount
├── hmac.go        # HMAC-SHA512 signing, verification, UTF-16LE encoding
├── azul_test.go   # Tests
├── go.mod
└── README.md
```

---

## Licencia

MIT
