# Admin Service — Frontend Integration Guide

This document is the contract between the Admin Service backend and any frontend (web dashboard, internal tools) that needs to call it. It covers every endpoint that is currently wired up, the exact request/response shapes, role requirements, and the error model.

If something here disagrees with the live Swagger UI at `http://<host>:8083/swagger/`, trust Swagger — it is generated directly from the `.proto` definitions.

---

## Table of Contents

1. [Service Overview](#service-overview)
2. [Base URL & Ports](#base-url--ports)
3. [Authentication & Roles](#authentication--roles)
4. [Common Conventions](#common-conventions)
5. [Error Model](#error-model)
6. [Endpoints](#endpoints)
   - [User Reporting (lives on deck-service / auth-service)](#user-reporting)
   - [Reports (admin)](#reports)
   - [Users](#users)
   - [Decks](#decks)
   - [Moderator Management](#moderator-management)
   - [Email Templates](#email-templates)
7. [Data Models (TypeScript)](#data-models-typescript)
8. [Endpoint × Role Matrix](#endpoint--role-matrix)
9. [Implementation Status](#implementation-status)
10. [Frontend Setup Tips](#frontend-setup-tips)

---

## Service Overview

The Admin Service is the moderation and platform-management backend. It powers an internal dashboard used by `admin` and `moderator` roles to:

- Triage and resolve user-submitted reports against decks, users, and notes.
- Manage user accounts (ban/unban, list).
- Hide or delete offending decks.
- Promote users to the `moderator` role (admin only).
- Manage system email templates — list, view, edit, preview, and send a test send (admin only).

Internally the service:

- Talks to **Auth Service** (gRPC, port `9090`) to verify the caller's JWT, look up users, and change roles.
- Talks to **Notification Service** (gRPC, port `9095`) for all email-template operations. The bearer token is forwarded so the notification service can re-verify the caller's role.
- Owns its own `admin_db` (Postgres) with two tables: `reports` and `moderation_logs`.
- **Consumes `report.submitted` events from Pub/Sub** at `POST /internal/pubsub` and inserts them into the `reports` table. End-user report intake itself lives on `deck-service` and `auth-service` — see [User Reporting](#user-reporting).

```
Frontend ──HTTP/JSON──▶ Admin Service :8083 (grpc-gateway)
                              │
                              ▼
                        gRPC :9093
                              │
                ┌─────────────┼─────────────────┐
                ▼             ▼                 ▼
            Postgres    Auth Service       Notification
            admin_db    :9090 (gRPC)       :9095 (gRPC)
```

---

## Base URL & Ports

| Concern            | Local default              | Notes                                                     |
|--------------------|----------------------------|-----------------------------------------------------------|
| HTTP / REST base   | `http://localhost:8083`    | All endpoints documented below live here.                 |
| Swagger UI         | `http://localhost:8083/swagger/` | Live, interactive API explorer.                       |
| gRPC (server-to-server) | `localhost:9093`       | Not for browsers. Use the REST gateway from the frontend. |

Configuration env vars (server side):

| Variable                       | Default            | Purpose                                          |
|--------------------------------|--------------------|--------------------------------------------------|
| `HTTP_SERVER_ADDRESS`          | `:8083`            | HTTP/JSON port.                                  |
| `GRPC_SERVER_ADDRESS`          | `:9093`            | gRPC port.                                       |
| `AUTH_SERVICE_ADDRESS`         | `localhost:9090`   | Where to reach Auth Service.                     |
| `NOTIFICATION_SERVICE_ADDRESS` | `localhost:9095`   | Where to reach Notification Service.             |
| `DATABASE_URL`                 | _required_         | Postgres connection string for `admin_db`.       |

---

## Authentication & Roles

### Bearer token

Every endpoint requires a JWT issued by the Auth Service. Attach it via the standard `Authorization` header:

```
Authorization: Bearer <access_token>
```

The token is verified on every request by calling `AuthService.VerifyToken`. The decoded payload yields `{ user_id, username, role }`, which the handler then checks against the endpoint's role requirement.

### Roles

Two roles can reach this service:

| Role        | Can do                                                                                 |
|-------------|-----------------------------------------------------------------------------------------|
| `admin`     | Everything — reports, users, decks, moderator promotion, email templates.              |
| `moderator` | Everything **except** moderator promotion and email-template management (admin-only). |

Any other role — `user`, missing role, etc. — gets `403 PermissionDenied`. A missing or malformed `Authorization` header returns `401 Unauthenticated`.

See the [Endpoint × Role Matrix](#endpoint--role-matrix) below for a per-endpoint breakdown.

### Login flow

Login itself is not on this service. The frontend:

1. Calls the **Auth Service** login endpoint with email + password.
2. Stores the returned `access_token` (e.g. in memory + `localStorage`, or in an `httpOnly` cookie if you proxy through your own server).
3. Attaches the token to every request to the Admin Service.
4. On `401` or `403`, clears the token and redirects to login.

---

## Common Conventions

- **Content type:** All requests and responses are JSON. `Content-Type: application/json`.
- **Field naming:** The gateway converts `snake_case` proto fields to `camelCase` in JSON. Examples: `report_id` → `reportId`, `template_key` → `templateKey`. The tables below show the JSON form.
- **IDs:** Every ID is a UUID string. Missing/empty optional UUIDs are returned as empty strings, not `null`.
- **Timestamps:** Returned as strings. Reports use Go's default timestamp `String()` format (`2026-05-15 10:24:00 +0000 UTC`), email templates use ISO 8601 strings produced by the notification service. Frontend should parse defensively (try `Date` constructor, fall back to displaying the raw string).
- **Pagination:** List endpoints accept `pageSize` (1–100, default 20) and `pageToken` query params. The response includes a `nextPageToken` — empty string means "no more pages". *Note:* the current `ListReports` implementation always returns an empty `nextPageToken` (offset-based stub); treat that as "no more pages" until real cursor pagination ships.
- **Verb conventions:** `GET` for reads, `PATCH` for partial updates, `POST` for actions, `PUT` for full template replacement.

---

## Error Model

The gateway maps gRPC status codes to HTTP status codes. Error responses always look like:

```json
{
  "code":    7,
  "message": "moderator access required",
  "details": []
}
```

| HTTP | gRPC code            | When it happens                                                   | Suggested UI action                                          |
|------|----------------------|-------------------------------------------------------------------|--------------------------------------------------------------|
| 400  | `InvalidArgument`    | Missing `template_key`/`email`, malformed UUID, unknown `action`. | Show inline form-validation error.                           |
| 401  | `Unauthenticated`    | No / malformed `Authorization` header, invalid or expired token.  | Clear token, redirect to login.                              |
| 403  | `PermissionDenied`   | Role isn't `admin` (or isn't `admin`/`moderator` where allowed).  | Show "you don't have access" or redirect to login.           |
| 404  | `NotFound`           | Report ID doesn't exist.                                          | Show "not found" inline; refresh list.                       |
| 500  | `Internal`           | DB error, auth-service down, notification-service unreachable.    | Show generic error toast; offer retry.                       |
| 501  | `Unimplemented`      | `ListUsers`, `BanUser`, `UpdateDeckStatus` (see status below).     | Disable / grey out the feature, label "coming soon".         |

---

## Endpoints

### User Reporting

> ⚠️ These endpoints **do not live on admin-service**. End-user reports are filed against the service that owns the target — `deck-service` for decks, `auth-service` for users. Each service publishes a `report.submitted` event to Pub/Sub; admin-service consumes the event and persists the report. From the user's perspective, the call is fire-and-forget — the response says "submitted", not "stored".

Flow:

```
                                  publish "report.submitted"
                              ┌──────────────────────────────┐
User ──HTTP──▶ deck-service ──┤                              │
              POST /v1/decks/{deck_id}/reports               │
                                                              ▼
                                                       ┌─────────────┐
                                                       │  Pub/Sub    │
                                                       └──────┬──────┘
User ──HTTP──▶ auth-service ──┐                              │
              POST /v1/users/{user_id}/reports               │
                              └──────────────────────────────┘
                                                              │ push
                                                              ▼
                                                  admin-service /internal/pubsub
                                                              │
                                                              ▼
                                                  INSERT INTO admin_db.reports
```

#### `POST /v1/decks/{deckId}/reports` — Report a deck (deck-service)

Required role: any authenticated user.

Base URL: **deck-service** HTTP gateway, not admin-service.

Path param: `deckId` — UUID of the deck being reported.

Request body:

```json
{
  "reasonCategory": "spam",
  "description":    "Posts ads in every card back."
}
```

| Field            | Required | Allowed values                                                                                  |
|------------------|----------|-------------------------------------------------------------------------------------------------|
| `reasonCategory` | **Yes**  | `inappropriate_content`, `copyright_violation`, `spam`, `harassment`, `misinformation`, `other` |
| `description`    | No       | Free text. Shown to moderators when triaging.                                                   |

The server verifies the deck exists (public decks are reportable by anyone) before publishing the event. Errors:

- `400 InvalidArgument` — invalid UUID or unknown `reasonCategory`.
- `404 NotFound` — deck doesn't exist.
- `500 Internal` — Pub/Sub publish failed.

Response: `200 OK`

```json
{ "message": "report submitted" }
```

`200` means "queued for moderation", not "stored in admin DB". Persistence happens asynchronously when admin-service consumes the event — typically within seconds.

---

#### `POST /v1/users/{userId}/reports` — Report a user (auth-service)

Required role: any authenticated user.

Base URL: **auth-service** HTTP gateway, not admin-service.

Path param: `userId` — UUID of the user being reported. Self-reports return `400 InvalidArgument` ("cannot report yourself").

Request body: same shape as the deck endpoint (`reasonCategory` + optional `description`).

Response: `200 OK`

```json
{ "message": "report submitted" }
```

---

**Frontend notes:**

- **Eventual consistency.** A successful submit does not guarantee the report is visible to moderators yet. Show a "Thanks, we got it" confirmation, not a "view your report" link.
- **No client-visible report ID.** The response doesn't return a persisted ID. If you need one, ask backend to assign and return it pre-publish.
- **Anti-abuse.** No per-user rate limit yet. Disable the submit button after a click; debounce on the client.
- **Note reports.** The `note` target type exists in the admin DB schema but no public endpoint to file note reports has shipped. Adding one would mean a new RPC on deck-service.
- **Possible duplicates.** Pub/Sub delivery is at-least-once. If a producer retries on a transient failure, moderators may see two near-identical reports. Acceptable for now; can be deduped server-side later with a producer-assigned event ID.

---

### Reports

#### `GET /v1/admin/reports` — List reports

Required role: `admin` **or** `moderator`.

Query params:

| Param          | Type    | Default | Notes                                                                    |
|----------------|---------|---------|--------------------------------------------------------------------------|
| `pageSize`     | int     | `20`    | Clamped to 1–100. Anything outside that resets to 20.                    |
| `pageToken`    | string  | —       | Reserved for future cursor pagination. Currently ignored by the server.  |
| `statusFilter` | string  | —       | One of `pending`, `reviewing`, `resolved`, `dismissed`. Empty = all.     |

Response: `200 OK`

```json
{
  "reports": [
    {
      "reportId":       "8c2e…",
      "reporterId":     "f1a3…",
      "targetType":     "deck",
      "targetId":       "b774…",
      "reasonCategory": "spam",
      "description":    "Posts ads in card backs.",
      "status":         "pending",
      "assignedTo":     "",
      "adminNote":      "",
      "resolution":     "",
      "resolvedBy":     "",
      "resolvedAt":     "",
      "createdAt":      "2026-05-12 10:24:00 +0000 UTC",
      "updatedAt":      "2026-05-12 10:24:00 +0000 UTC"
    }
  ],
  "nextPageToken": ""
}
```

Ordered newest-first (by `created_at DESC`).

---

#### `PATCH /v1/admin/reports/{reportId}` — Process report

Required role: `admin` **or** `moderator`.

Path param: `reportId` — UUID.

Request body:

```json
{
  "action":     "resolve",
  "resolution": "banned",
  "adminNote":  "Repeated copyright violations."
}
```

| Field         | Required        | Allowed values                                          | Notes                                                                                  |
|---------------|-----------------|---------------------------------------------------------|----------------------------------------------------------------------------------------|
| `action`      | **Yes**         | `resolve`, `dismiss`, `review`                          | Anything else → `400 InvalidArgument`. Maps to status `resolved`/`dismissed`/`reviewing`. |
| `resolution`  | When resolving  | `banned`, `deleted`, `warned`, `no_action` (free-form string accepted) | Free-text — server does not enum-validate. Convention is one of the four values listed. |
| `adminNote`   | No              | Any string                                              | Stored on the report and visible only to moderators.                                   |

When `action` is `resolve` or `dismiss`, the server sets `resolvedAt = NOW()` and `resolvedBy = <caller's user_id>`. An entry is also written to `moderation_logs` with `action = "process_report"`.

Response: `200 OK`

```json
{
  "report": { /* updated Report (same shape as list item) */ }
}
```

Errors: `400` invalid UUID or invalid `action`, `404` report not found.

---

### Users

#### `GET /v1/admin/users` — List users *(not implemented — returns 501)*

Required role: `admin` **or** `moderator`.

Query params: `pageSize`, `pageToken`, `filterBanned` (bool).

Returns `501 Unimplemented` today. The handler still does the auth check, so a non-moderator caller will get `403` instead.

---

#### `PATCH /v1/admin/users/{userId}/ban` — Ban / unban user *(not implemented — returns 501)*

Required role: `admin` **or** `moderator`.

Request body (planned):

```json
{
  "ban":    true,
  "reason": "Repeated harassment reports."
}
```

---

### Decks

#### `PATCH /v1/admin/decks/{deckId}/status` — Update deck status *(not implemented — returns 501)*

Required role: `admin` **or** `moderator`.

Request body (planned):

```json
{
  "status": "hidden",
  "reason": "Reported for inappropriate content."
}
```

`status` will accept `hidden`, `deleted`, or `active`.

---

### Moderator Management

#### `POST /v1/admin/users/promote-moderator` — Promote user to moderator

Required role: `admin` **only** (moderators are rejected with `403`).

Request body:

```json
{
  "email": "newmod@example.com"
}
```

| Field   | Required | Notes                                                          |
|---------|----------|----------------------------------------------------------------|
| `email` | **Yes**  | Email of the existing user. Empty → `400 InvalidArgument`.     |

The server calls `AuthService.SetUserRole(email, "moderator")`. If the email doesn't exist, the auth service propagates the error (typically surfaces as `500` from the gateway).

Response: `200 OK`

```json
{
  "userId":   "f1a3…",
  "email":    "newmod@example.com",
  "username": "newmod"
}
```

---

### Email Templates

These endpoints are thin proxies over the Notification Service's admin-only template management API. The Admin Service:

- Verifies the caller is `admin` (moderators get `403`).
- Forwards the bearer token so the Notification Service re-checks the role.
- Writes a `moderation_logs` entry for **updates** (not for reads, previews, or test sends).

A template is keyed by `(template_key, locale)`. If `locale` is omitted, the server falls back to the default locale (currently `en`).

The default seeded templates are:

| `templateKey`        | `locale` | Variables                          |
|----------------------|----------|------------------------------------|
| `welcome`            | `en`     | `Username`                         |
| `email_verification` | `en`     | `Username`, `URL`                  |
| `password_reset`     | `en`     | `Username`, `URL`                  |
| `study_reminder`     | `en`     | `Username`, `DueCount`, `URL`      |

Template variables use Go's `text/template` / `html/template` syntax: `{{.Username}}`, `{{.URL}}`, etc. The frontend's preview form should expose those keys as text inputs.

---

#### `GET /v1/admin/email-templates` — List templates

Required role: `admin`.

No query params.

Response: `200 OK`

```json
{
  "templates": [
    {
      "id":          "uuid",
      "templateKey": "welcome",
      "locale":      "en",
      "subject":     "Welcome to MemPan!",
      "htmlBody":    "<!DOCTYPE html>…",
      "textBody":    "Welcome to MemPan, {{.Username}}!\n…",
      "variables":   ["Username"],
      "isActive":    true,
      "version":     1,
      "updatedBy":   "",
      "createdAt":   "2026-05-01T00:00:00Z",
      "updatedAt":   "2026-05-01T00:00:00Z"
    }
  ]
}
```

---

#### `GET /v1/admin/email-templates/{templateKey}` — Get one template

Required role: `admin`.

Path param: `templateKey`.

Query param: `locale` (optional; default locale used when empty).

Response: `200 OK` — single `EmailTemplate` (same shape as the list item).

Errors: `400` empty `templateKey`, `404` template not found (surfaces as the notification-service error).

---

#### `PUT /v1/admin/email-templates/{templateKey}` — Update template content

Required role: `admin`.

Path param: `templateKey`.

Request body:

```json
{
  "locale":   "en",
  "subject":  "Welcome to MemPan, {{.Username}}!",
  "htmlBody": "<!DOCTYPE html><html>…</html>",
  "textBody": "Welcome to MemPan, {{.Username}}!\n…"
}
```

| Field         | Required | Notes                                                                               |
|---------------|----------|-------------------------------------------------------------------------------------|
| `locale`      | No       | Defaults to the system default (`en`).                                              |
| `subject`     | No       | If provided, replaces the current subject.                                          |
| `htmlBody`    | No       | If provided, replaces the current HTML body.                                        |
| `textBody`    | No       | If provided, replaces the current plain-text body.                                  |

Behavior:

- Notification Service bumps `version` and writes a snapshot of the previous content to `email_template_versions`.
- `updatedBy` is set to the caller's `user_id` (forwarded via the bearer token).
- Admin Service writes a `moderation_logs` row with `action = "update_email_template"`, `target_type = "email_template"`, and metadata `{ template_key, locale, version }`.

Response: `200 OK` — the updated `EmailTemplate`.

---

#### `POST /v1/admin/email-templates/{templateKey}:preview` — Render preview

Required role: `admin`.

Path param: `templateKey`.

Request body:

```json
{
  "locale": "en",
  "data": {
    "Username": "Alice",
    "URL":      "https://mempan.example.com/verify?token=xyz"
  }
}
```

The `data` map keys must match the template's variables. Missing keys render as the Go zero value (empty string) — not an error.

Response: `200 OK`

```json
{
  "subject":  "Verify your MemPan email",
  "htmlBody": "<!DOCTYPE html>…rendered…",
  "textBody": "Verify your email, Alice\n…"
}
```

Errors: `400` empty `templateKey`, `500` template parse error.

---

#### `POST /v1/admin/email-templates/{templateKey}:test` — Send a test email

Required role: `admin`.

Path param: `templateKey`.

Request body:

```json
{
  "locale": "en",
  "to":     "qa@example.com",
  "data": {
    "Username": "Alice",
    "URL":      "https://mempan.example.com/verify?token=xyz"
  }
}
```

| Field         | Required | Notes                                                       |
|---------------|----------|-------------------------------------------------------------|
| `locale`      | No       | Default locale used when empty.                             |
| `to`          | **Yes**  | Recipient address. Empty → `400 InvalidArgument`.           |
| `data`        | No       | Same shape as preview.                                      |

Response: `200 OK`

```json
{
  "message": "test email sent to qa@example.com"
}
```

The notification service uses its configured SMTP transport. If SMTP isn't configured locally, expect a `500`. Surface the message text from the response to the operator so they know what happened.

---

## Data Models (TypeScript)

```typescript
// All UUIDs are string. Missing optional UUIDs come back as "" (not null).

export type ReportStatus    = "pending" | "reviewing" | "resolved" | "dismissed";
export type ReportTarget    = "deck" | "user" | "note";
export type ReportCategory  =
  | "inappropriate_content"
  | "copyright_violation"
  | "spam"
  | "harassment"
  | "misinformation"
  | "other";

// Conventional values; server accepts any string.
export type Resolution      = "banned" | "deleted" | "warned" | "no_action";

export interface Report {
  reportId:       string;
  reporterId:     string;
  targetType:     ReportTarget;
  targetId:       string;
  reasonCategory: ReportCategory;
  description:    string;
  status:         ReportStatus;
  assignedTo:     string;     // "" when unassigned
  adminNote:      string;
  resolution:     string;     // "" when unresolved
  resolvedBy:     string;     // "" when unresolved
  resolvedAt:     string;     // "" when unresolved
  createdAt:      string;
  updatedAt:      string;
}

export interface ListReportsResponse {
  reports: Report[];
  nextPageToken: string;      // "" = no more pages
}

// Used for POST /v1/decks/{deckId}/reports on deck-service and
// POST /v1/users/{userId}/reports on auth-service. The target ID lives in
// the URL path, not the body.
export interface SubmitReportPayload {
  reasonCategory: ReportCategory;
  description?:   string;
}

export interface SubmitReportResponse {
  message: string; // "report submitted"
}

export interface ProcessReportPayload {
  action: "resolve" | "dismiss" | "review";
  resolution?: string;        // required when action = "resolve" (by convention)
  adminNote?: string;
}

export interface User {
  id:        string;
  username:  string;
  email:     string;
  isBanned:  boolean;
  createdAt: string;
}

export interface PromoteModeratorResponse {
  userId:   string;
  email:    string;
  username: string;
}

export interface EmailTemplate {
  id:          string;
  templateKey: string;
  locale:      string;
  subject:     string;
  htmlBody:    string;
  textBody:    string;
  variables:   string[];
  isActive:    boolean;
  version:     number;
  updatedBy:   string;        // "" if never updated
  createdAt:   string;
  updatedAt:   string;
}

export interface PreviewEmailTemplatePayload {
  locale?: string;
  data?:   Record<string, string>;
}

export interface PreviewEmailTemplateResponse {
  subject:  string;
  htmlBody: string;
  textBody: string;
}

export interface SendTestEmailPayload {
  locale?: string;
  to:      string;
  data?:   Record<string, string>;
}

export interface SendTestEmailResponse {
  message: string;
}
```

---

## Endpoint × Role Matrix

| Endpoint                                                                  | `admin` | `moderator` | `user` | Status                  |
|---------------------------------------------------------------------------|:-------:|:-----------:|:------:|-------------------------|
| `POST   /v1/decks/{deckId}/reports` *(deck-service)*                      | ✅      | ✅          | ✅     | Implemented (publishes) |
| `POST   /v1/users/{userId}/reports` *(auth-service)*                      | ✅      | ✅          | ✅     | Implemented (publishes) |
| `GET    /v1/admin/reports`                                                | ✅      | ✅          | ❌     | Implemented             |
| `PATCH  /v1/admin/reports/{reportId}`                   | ✅      | ✅          | Implemented         |
| `GET    /v1/admin/users`                                | ✅      | ✅          | **501 — stub**      |
| `PATCH  /v1/admin/users/{userId}/ban`                   | ✅      | ✅          | **501 — stub**      |
| `PATCH  /v1/admin/decks/{deckId}/status`                | ✅      | ✅          | **501 — stub**      |
| `POST   /v1/admin/users/promote-moderator`              | ✅      | ❌          | Implemented         |
| `GET    /v1/admin/email-templates`                      | ✅      | ❌          | Implemented (proxy) |
| `GET    /v1/admin/email-templates/{templateKey}`        | ✅      | ❌          | Implemented (proxy) |
| `PUT    /v1/admin/email-templates/{templateKey}`        | ✅      | ❌          | Implemented (proxy) |
| `POST   /v1/admin/email-templates/{templateKey}:preview`| ✅      | ❌          | Implemented (proxy) |
| `POST   /v1/admin/email-templates/{templateKey}:test`   | ✅      | ❌          | Implemented (proxy) |

---

## Implementation Status

| Area                              | Status                                                                                                       |
|-----------------------------------|--------------------------------------------------------------------------------------------------------------|
| Reports — user submission         | ✅ Live via Pub/Sub. Intake on deck-service (`POST /v1/decks/{id}/reports`) and auth-service (`POST /v1/users/{id}/reports`); admin-service consumes `report.submitted` and writes to `admin_db.reports`. |
| Reports — list & process          | ✅ Live, backed by `admin_db.reports`.                                                                       |
| Moderation audit log              | ✅ Written to `admin_db.moderation_logs` on report process & email-template updates. **No API yet** to read it. |
| Users — list                      | ⚠️ Returns `501`. Wire your UI but disable / grey out.              |
| Users — ban / unban               | ⚠️ Returns `501`.                                                   |
| Decks — update status             | ⚠️ Returns `501`.                                                   |
| Promote moderator                 | ✅ Live.                                                            |
| Email templates — list/get/update/preview/test | ✅ Live (proxied to notification-service).             |

**Recommended frontend build order:**

1. Login + auth wiring (shared with the rest of the app).
2. Reports page — the only fully-featured workflow today.
3. Email templates page — list → edit (with side-by-side preview) → send test.
4. Moderator promotion form (admin-only).
5. Stub Users and Decks pages with a "Coming soon" notice, so navigation is already in place.

---

## Frontend Setup Tips

### Discovering the API

The fastest way to confirm a payload shape is the live Swagger UI:

```
http://<admin-service-host>:8083/swagger/
```

It also generates a downloadable OpenAPI 2.0 JSON document at `/swagger/admin_service.swagger.json`, which you can feed into `openapi-typescript` / `orval` / etc. to auto-generate a typed client.

### CORS

The gateway does **not** add CORS headers today. If your frontend and the Admin Service are on different origins during development, either:

- Proxy via your dev server (Vite example):

  ```ts
  // vite.config.ts
  export default {
    server: {
      proxy: {
        "/v1": "http://localhost:8083",
      },
    },
  };
  ```

- Or ask backend to add `rs/cors` middleware in `cmd/server/main.go`.

The proxy approach is zero-config and is what we use locally.

### Minimal `axios` client

```ts
// src/api/admin.ts
import axios from "axios";

export const adminApi = axios.create({
  baseURL: import.meta.env.VITE_ADMIN_API_BASE_URL ?? "http://localhost:8083",
  headers: { "Content-Type": "application/json" },
});

adminApi.interceptors.request.use((config) => {
  const token = localStorage.getItem("access_token");
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

adminApi.interceptors.response.use(
  (res) => res,
  (err) => {
    const code = err.response?.status;
    if (code === 401 || code === 403) {
      localStorage.removeItem("access_token");
      window.location.href = "/login";
    }
    return Promise.reject(err);
  }
);
```

### Recommended environment variables

| Variable                    | Example                  | Purpose                                |
|-----------------------------|--------------------------|----------------------------------------|
| `VITE_ADMIN_API_BASE_URL`   | `http://localhost:8083`  | Where to send admin requests.          |
| `VITE_AUTH_API_BASE_URL`    | `http://localhost:8080`  | Where to send login / refresh.         |

### Things to watch out for

- **Empty strings instead of `null`** for optional UUIDs and timestamps on `Report`. Treat `""` as "not set".
- **Timestamp formats differ** between report endpoints (Go default) and email-template endpoints (ISO 8601). Don't assume one format.
- **Pagination is partly stubbed:** `ListReports` ignores `pageToken` and always returns `nextPageToken: ""`. Hide the "Next page" button until the server reports a real token.
- **Free-text `resolution` field:** the server doesn't validate the enum. Lock down the values from the frontend with a `<select>`.
- **501 endpoints still authorize:** a non-moderator hitting `/v1/admin/users` will get `403`, not `501`. Don't infer "endpoint exists" from a `501`.
- **Template variables are case-sensitive** Go template fields (`{{.Username}}`, not `{{.username}}`).
