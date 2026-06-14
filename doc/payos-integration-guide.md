# payOS Integration Guide

Sources read:

- https://payos.vn/docs/
- https://payos.vn/docs/api/
- https://payos.vn/docs/checkout/
- https://payos.vn/docs/checkout/how-checkout-works/
- https://payos.vn/docs/du-lieu-tra-ve/return-url/
- https://payos.vn/docs/du-lieu-tra-ve/webhook/
- https://payos.vn/docs/tich-hop-webhook/kiem-tra-du-lieu-voi-signature/
- https://payos.vn/docs/sdks/intro/
- https://payos.vn/docs/sdks/front-end/script-js/
- https://payos.vn/docs/sdks/back-end/node/
- https://payos.vn/docs/moi-truong-test/

## 1. Overview

payOS lets a merchant accept Napas 24/7 bank-transfer payments through a hosted checkout page, embedded checkout iframe, or popup checkout. A typical integration has three server or application responsibilities:

1. Create a payment link for an order.
2. Handle `returnUrl` and `cancelUrl` redirects for the customer-facing result screen.
3. Handle payOS webhooks and update the order state in the database.

Do not rely only on the browser redirect to mark an order paid. Use webhook verification or a server-side payment status lookup as the reliable source for order state.

## 2. Prerequisites

Before integration:

1. Create a payOS account at https://my.payos.vn.
2. Verify an individual or business account.
3. Create a payment channel.
4. Get these credentials from the payment channel:
   - `PAYOS_CLIENT_ID`
   - `PAYOS_API_KEY`
   - `PAYOS_CHECKSUM_KEY`
5. Configure a public HTTPS webhook endpoint in payOS.

Production API base URL:

```text
https://api-merchant.payos.vn
```

payOS currently does not provide a separate sandbox or staging environment. Test with a real account, a linked bank account, and small-value transactions on the production environment.

## 3. Payment Flow

1. Customer chooses Napas 24/7 bank transfer on the merchant site or app.
2. Backend creates a payment link by calling payOS.
3. payOS returns payment data, including `checkoutUrl`, `paymentLinkId`, QR data, amount, and status.
4. Frontend redirects the customer to `checkoutUrl`, opens it in a popup, or embeds it in an iframe.
5. Customer pays by scanning the VietQR code in a banking app.
6. Browser returns to `returnUrl` on successful payment or `cancelUrl` on cancellation, with query params.
7. payOS sends a webhook to the merchant server with full payment details.
8. Merchant verifies the webhook signature and updates the order status.

## 4. API Authentication

For payment-link APIs, send:

```http
x-client-id: <PAYOS_CLIENT_ID>
x-api-key: <PAYOS_API_KEY>
Content-Type: application/json
```

If participating in the payOS integration partner program, include:

```http
x-partner-code: <PARTNER_CODE>
```

## 5. Create Payment Link

Endpoint:

```http
POST /v2/payment-requests
```

Full URL:

```text
https://api-merchant.payos.vn/v2/payment-requests
```

Required body fields:

- `orderCode`: merchant order code, integer.
- `amount`: payment amount, integer in VND.
- `description`: payment description. The docs note a 9-character limit for bank accounts not linked through payOS.
- `cancelUrl`: URL to receive the customer after cancellation.
- `returnUrl`: URL to receive the customer after successful payment.
- `signature`: HMAC-SHA256 signature created with the checksum key.

Common optional fields:

- `buyerName`
- `buyerEmail`
- `buyerPhone`
- `items`
- `expiredAt`
- `invoice`

Signature input for creating a payment request:

```text
amount=<amount>&cancelUrl=<cancelUrl>&description=<description>&orderCode=<orderCode>&returnUrl=<returnUrl>
```

The string must be sorted alphabetically by key and signed with HMAC-SHA256 using `PAYOS_CHECKSUM_KEY`.

Example request shape:

```json
{
  "orderCode": 123456,
  "amount": 50000,
  "description": "ORDER123",
  "items": [
    {
      "name": "Product A",
      "quantity": 1,
      "price": 50000
    }
  ],
  "cancelUrl": "https://your-domain.com/payment/cancel",
  "returnUrl": "https://your-domain.com/payment/return",
  "signature": "<hmac_sha256_signature>"
}
```

Important response fields:

- `data.checkoutUrl`: URL used by frontend checkout.
- `data.paymentLinkId`: payOS payment link ID.
- `data.orderCode`: merchant order code.
- `data.status`: initial status, usually `PENDING`.
- `data.qrCode`: VietQR payload.
- top-level `signature`: response signature.

## 6. Other Payment Link APIs

Get payment link information:

```http
GET /v2/payment-requests/{id}
```

`id` can be the merchant `orderCode` or the payOS `paymentLinkId`.

Cancel a payment link:

```http
POST /v2/payment-requests/{id}/cancel
```

Example cancellation body:

```json
{
  "cancellationReason": "Customer requested cancellation"
}
```

Invoice APIs:

```http
GET /v2/payment-requests/{id}/invoices
GET /v2/payment-requests/{id}/invoices/{invoice-id}/download
```

## 7. Checkout Options

### Hosted Page

The simplest option is redirecting the customer to `data.checkoutUrl` returned by the create-payment-link API.

Use this when:

- You want the smallest frontend integration.
- It is acceptable for the customer to leave the site temporarily.

### Popup or Embedded Checkout

payOS provides a JavaScript checkout script:

```html
<script src="https://cdn.payos.vn/payos-checkout/v1/stable/payos-initialize.js"></script>
```

For npm/yarn projects:

```bash
npm install payos-checkout --save
# or
yarn add payos-checkout
```

Basic config shape:

```ts
const payOSConfig = {
  RETURN_URL: "https://your-domain.com/payment/return",
  ELEMENT_ID: "payos-checkout",
  CHECKOUT_URL: checkoutUrl,
  embedded: true,
  onSuccess: (event) => {
    // Show success state, then confirm status on the backend.
  },
  onCancel: (event) => {
    // Show cancellation state.
  },
  onExit: (event) => {
    // Handle popup or iframe exit.
  }
};

const { open, exit } = PayOSCheckout.usePayOS(payOSConfig);
open();
```

For embedded checkout, `RETURN_URL` must match the page that displays the iframe.

If using Content Security Policy, allow payOS CDN/script/frame/connect URLs according to the payOS Script JS documentation.

## 8. Return URL and Cancel URL

After payment or cancellation, payOS redirects the browser to the configured URL and appends query params.

Expected query params:

- `code`: `00` for success, `01` for invalid params.
- `id`: payOS payment link ID.
- `cancel`: `true` if cancelled, `false` otherwise.
- `status`: one of `PAID`, `PENDING`, `PROCESSING`, `CANCELLED`.
- `orderCode`: merchant order code.

Recommended frontend behavior:

1. Read the query params.
2. Show a pending, success, or cancelled screen.
3. Call your backend to fetch the latest order/payment status.
4. Do not permanently mark the order as paid only from query params.

## 9. Webhook Handling

payOS sends payment results to the configured webhook endpoint.

Webhook body fields:

- `code`
- `desc`
- `success`
- `data`
- `signature`

Important fields inside `data`:

- `orderCode`
- `amount`
- `description`
- `accountNumber`
- `reference`
- `transactionDateTime`
- `currency`
- `paymentLinkId`
- `code`
- `desc`
- `counterAccountBankId`
- `counterAccountBankName`
- `counterAccountName`
- `counterAccountNumber`
- `virtualAccountName`
- `virtualAccountNumber`

Webhook endpoint behavior:

1. Receive JSON.
2. Verify `signature` against `data`.
3. Reject invalid signatures with a non-2xx status.
4. Find the order by `orderCode`.
5. Check amount and expected status transition.
6. Mark the order paid only after verification.
7. Return a 2xx response after successful processing.
8. Make the handler idempotent because webhooks can be retried.

Confirm or update webhook URL through the API:

```http
POST /confirm-webhook
```

Full URL:

```text
https://api-merchant.payos.vn/confirm-webhook
```

Body:

```json
{
  "webhookUrl": "https://your-domain.com/api/payos/webhook"
}
```

## 10. Signature Verification

payOS signs data with HMAC-SHA256 and the channel checksum key.

General signing rules:

1. Sort fields alphabetically by key.
2. Build a string in this format:

```text
key1=value1&key2=value2&key3=value3
```

3. Treat `undefined` and `null` values as empty strings.
4. For nested arrays or objects, sort nested keys before JSON serialization.
5. Create HMAC-SHA256 with `PAYOS_CHECKSUM_KEY`.
6. Compare the generated hex digest with the provided signature.

Node.js helper:

```ts
import crypto from "crypto";

function sortObject(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map(sortObject);
  }

  if (value && typeof value === "object") {
    return Object.keys(value as Record<string, unknown>)
      .sort()
      .reduce<Record<string, unknown>>((result, key) => {
        result[key] = sortObject((value as Record<string, unknown>)[key]);
        return result;
      }, {});
  }

  return value;
}

function createPayOSSignature(data: Record<string, unknown>, checksumKey: string): string {
  const signedData = Object.keys(data)
    .sort()
    .map((key) => {
      const rawValue = data[key];
      const value =
        rawValue === null || rawValue === undefined
          ? ""
          : typeof rawValue === "object"
            ? JSON.stringify(sortObject(rawValue))
            : String(rawValue);

      return `${key}=${value}`;
    })
    .join("&");

  return crypto.createHmac("sha256", checksumKey).update(signedData).digest("hex");
}
```

Use this helper for webhook verification by signing the webhook `data` object and comparing it to the webhook `signature`.

## 11. Node.js SDK

Install:

```bash
npm install @payos/node
```

Initialize:

```ts
import { PayOS } from "@payos/node";

const payOS = new PayOS({
  clientId: process.env.PAYOS_CLIENT_ID,
  apiKey: process.env.PAYOS_API_KEY,
  checksumKey: process.env.PAYOS_CHECKSUM_KEY
});
```

Create payment link:

```ts
const paymentLink = await payOS.paymentRequests.create({
  orderCode: 123456,
  amount: 50000,
  description: "ORDER123",
  items: [
    {
      name: "Product A",
      quantity: 1,
      price: 50000
    }
  ],
  cancelUrl: "https://your-domain.com/payment/cancel",
  returnUrl: "https://your-domain.com/payment/return"
});

console.log(paymentLink.checkoutUrl);
```

Verify webhook:

```ts
app.post("/api/payos/webhook", (req, res) => {
  try {
    const webhookData = payOS.webhooks.verify(req.body);
    // Update order state here.
    res.status(200).send("OK");
  } catch {
    res.status(400).send("Invalid webhook");
  }
});
```

## 12. Other SDKs

payOS documents server-side SDKs for:

- Node: `npm install @payos/node`
- Python: `pip install payos` or `pip3 install payos`
- .NET Core: `dotnet add package payOS`
- PHP: `composer require payos/payos`
- Go: `go get github.com/payOSHQ/payos-lib-golang/v2`

Frontend checkout options:

- JavaScript checkout script from `https://cdn.payos.vn`
- React checkout package: `payos-checkout`

## 13. Payout APIs

payOS also documents payout APIs under the "Chi" section. These are separate from payment collection.

Important payout endpoints:

```http
POST /v1/payouts
GET /v1/payouts
POST /v1/payouts/batch
GET /v1/payouts/{payoutId}
POST /v1/payouts/estimate-credit
GET /v1/payouts-account/balance
```

Payout requests require PayOS disbursement-channel credentials and additional
payout headers such as:

- `x-idempotency-key`
- `x-signature`

Use a unique idempotency key for each payout request to prevent duplicate disbursements.
In this project, payouts use separate disbursement-channel credentials:

- `PAYOS_PAYOUT_CLIENT_ID`
- `PAYOS_PAYOUT_API_KEY`
- `PAYOS_PAYOUT_CHECKSUM_KEY`

Do not reuse payment-channel credentials (`PAYOS_CLIENT_ID`, `PAYOS_API_KEY`,
`PAYOS_CHECKSUM_KEY`) for payout calls.

Project status as of 2026-06-11:

- Automatic PayOS payout creation is implemented in `billing-service`.
- Admins can create single payouts, batch payouts, and check payout-account
  balance through billing-service routes.
- Backend payout validation must reject withdrawal amounts unless
  `amount_vnd > 100000`.
- See `doc/billing-service-deployment.md` for the deployment state and payout
  production checklist.

## 14. Testing Checklist

Because there is no separate sandbox:

1. Use a personal payOS account first.
2. Link a real bank account.
3. Use low-value transactions.
4. Test hosted checkout.
5. Test popup or embedded checkout if used.
6. Test `returnUrl`.
7. Test `cancelUrl`.
8. Test webhook delivery.
9. Test invalid webhook signatures.
10. Test duplicate webhook delivery.
11. Test expired or cancelled payment links.
12. Confirm database state transitions for `PENDING`, `PROCESSING`, `PAID`, and `CANCELLED`.
13. Before enabling automatic payout, test that `100000` VND is rejected and
    `100001` VND is accepted by backend validation.

## 15. Implementation Notes

- Store credentials only in environment variables or a secret manager.
- Keep `PAYOS_CHECKSUM_KEY` server-side only.
- Generate `orderCode` as a unique integer.
- Keep `description` short and deterministic.
- Persist `paymentLinkId`, `checkoutUrl`, initial status, and amount when creating a payment link.
- Verify amount and order code before marking an order paid.
- Make webhook processing idempotent.
- Add logs for webhook receipt, signature verification result, order lookup, and state transition.
- Handle HTTP 429 responses with retry and backoff.
- Treat webhook as the authoritative payment update, not the customer redirect.
