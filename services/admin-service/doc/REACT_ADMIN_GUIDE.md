# Admin Service — React Web Interface Guide

This guide covers everything needed to build a React frontend for the Admin Service moderation dashboard.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Authentication](#authentication)
3. [API Reference](#api-reference)
4. [Data Models](#data-models)
5. [Project Structure](#project-structure)
6. [Setup](#setup)
7. [API Client](#api-client)
8. [Pages & Components](#pages--components)
9. [State Management](#state-management)
10. [Routing](#routing)
11. [Error Handling](#error-handling)
12. [Environment Variables](#environment-variables)

---

## Architecture Overview

```
React App  ──HTTP──▶  Admin Service (port 8083)
                           │
                    grpc-gateway proxy
                           │
                      gRPC Server (port 9093)
                           │
                  ┌────────┴────────┐
               PostgreSQL        Auth Service (port 9090)
```

The Admin Service exposes a REST/JSON API via grpc-gateway on **port 8083**. All endpoints require a valid admin JWT bearer token, which is obtained from the Auth Service login flow.

Swagger UI is available at `http://localhost:8083/swagger/` for interactive API testing.

---

## Authentication

All requests must include an `Authorization` header with a Bearer token:

```
Authorization: Bearer <access_token>
```

The token is verified against the Auth Service. Only accounts with **role = `admin`** are allowed through. Any other role receives `403 Forbidden`.

### Login Flow

1. User submits credentials to the Auth Service login endpoint (not this service).
2. Store the returned `access_token` in memory or `localStorage`.
3. Attach the token to every request to the Admin Service.
4. On `401 Unauthenticated` or `403 PermissionDenied`, redirect to the login page.

---

## API Reference

Base URL: `http://localhost:8083`

All request/response bodies are JSON. Timestamps are ISO 8601 strings. UUIDs are strings.

### Reports

#### List Reports

```
GET /v1/admin/reports
```

**Query parameters:**

| Parameter      | Type   | Default | Description                                              |
|----------------|--------|---------|----------------------------------------------------------|
| `pageSize`     | int    | 20      | Number of results per page (max 100)                     |
| `pageToken`    | string | —       | Cursor from previous response for next page              |
| `statusFilter` | string | —       | Filter by status: `pending`, `reviewing`, `resolved`, `dismissed` |

**Response:**

```json
{
  "reports": [ /* Report[] */ ],
  "nextPageToken": "string"
}
```

`nextPageToken` is empty when there are no more pages.

---

#### Process Report

```
PATCH /v1/admin/reports/{report_id}
```

**Path parameter:** `report_id` — UUID of the report.

**Request body:**

```json
{
  "action":     "resolve | dismiss | review",
  "resolution": "banned | deleted | warned | no_action",
  "adminNote":  "Internal note visible only to admins"
}
```

| Field        | Required       | Description                                              |
|--------------|----------------|----------------------------------------------------------|
| `action`     | Yes            | What to do with the report                               |
| `resolution` | When resolving | Outcome of the investigation                             |
| `adminNote`  | No             | Free-text note stored on the report                      |

**Response:**

```json
{
  "report": { /* Report */ }
}
```

---

### Users *(not yet implemented — returns 501)*

#### List Users

```
GET /v1/admin/users?pageSize=20&pageToken=&filterBanned=false
```

#### Ban / Unban User

```
PATCH /v1/admin/users/{user_id}/ban
```

```json
{
  "ban":    true,
  "reason": "Reason for the ban"
}
```

---

### Decks *(not yet implemented — returns 501)*

#### Update Deck Status

```
PATCH /v1/admin/decks/{deck_id}/status
```

```json
{
  "status": "hidden | deleted | active",
  "reason": "Reason for the status change"
}
```

---

## Data Models

### Report

```typescript
interface Report {
  reportId:       string;       // UUID
  reporterId:     string;       // UUID of the user who filed the report
  targetType:     "deck" | "user" | "note";
  targetId:       string;       // UUID of the reported entity
  reasonCategory: ReportCategory;
  description:    string;
  status:         ReportStatus;
  assignedTo?:    string;       // UUID of the assigned admin
  adminNote?:     string;
  resolution?:    Resolution;
  resolvedBy?:    string;       // UUID of the resolving admin
  resolvedAt?:    string;       // ISO 8601
  createdAt:      string;       // ISO 8601
  updatedAt:      string;       // ISO 8601
}

type ReportStatus    = "pending" | "reviewing" | "resolved" | "dismissed";
type ReportCategory  = "inappropriate_content" | "copyright_violation" | "spam" | "harassment" | "misinformation" | "other";
type Resolution      = "banned" | "deleted" | "warned" | "no_action";
```

### User

```typescript
interface User {
  id:        string;   // UUID
  username:  string;
  email:     string;
  isBanned:  boolean;
  createdAt: string;   // ISO 8601
}
```

---

## Project Structure

Recommended layout for the React app:

```
admin-web/
├── public/
├── src/
│   ├── api/
│   │   ├── client.ts          # Axios instance with auth interceptors
│   │   ├── reports.ts         # Report API calls
│   │   ├── users.ts           # User API calls (future)
│   │   └── decks.ts           # Deck API calls (future)
│   ├── components/
│   │   ├── layout/
│   │   │   ├── Sidebar.tsx
│   │   │   └── TopBar.tsx
│   │   ├── reports/
│   │   │   ├── ReportTable.tsx
│   │   │   ├── ReportRow.tsx
│   │   │   ├── ReportDetail.tsx
│   │   │   └── ProcessReportModal.tsx
│   │   └── common/
│   │       ├── StatusBadge.tsx
│   │       ├── Pagination.tsx
│   │       └── LoadingSpinner.tsx
│   ├── hooks/
│   │   ├── useReports.ts
│   │   └── useAuth.ts
│   ├── pages/
│   │   ├── LoginPage.tsx
│   │   ├── ReportsPage.tsx
│   │   ├── UsersPage.tsx      # future
│   │   └── DecksPage.tsx      # future
│   ├── store/
│   │   └── authStore.ts       # Auth token storage (Zustand / Context)
│   ├── types/
│   │   └── admin.ts           # TypeScript types (Report, User, etc.)
│   ├── App.tsx
│   └── main.tsx
├── .env
├── .env.example
└── package.json
```

---

## Setup

```bash
# Scaffold (Vite + React + TypeScript)
npm create vite@latest admin-web -- --template react-ts
cd admin-web

# Core dependencies
npm install axios react-router-dom

# Optional but recommended
npm install @tanstack/react-query   # Server state management
npm install zustand                 # Client state (auth token)
npm install @radix-ui/react-dialog  # Accessible modal for process-report form
```

`.env`:

```env
VITE_API_BASE_URL=http://localhost:8083
```

---

## API Client

`src/api/client.ts`

```typescript
import axios from "axios";

export const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL,
  headers: { "Content-Type": "application/json" },
});

// Attach token from storage on every request
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem("access_token");
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

// Redirect to login on auth errors
apiClient.interceptors.response.use(
  (res) => res,
  (error) => {
    if (error.response?.status === 401 || error.response?.status === 403) {
      localStorage.removeItem("access_token");
      window.location.href = "/login";
    }
    return Promise.reject(error);
  }
);
```

`src/api/reports.ts`

```typescript
import { apiClient } from "./client";
import type { Report } from "../types/admin";

export interface ListReportsParams {
  pageSize?: number;
  pageToken?: string;
  statusFilter?: string;
}

export interface ListReportsResponse {
  reports: Report[];
  nextPageToken: string;
}

export const listReports = (params: ListReportsParams) =>
  apiClient
    .get<ListReportsResponse>("/v1/admin/reports", { params })
    .then((r) => r.data);

export interface ProcessReportPayload {
  action: "resolve" | "dismiss" | "review";
  resolution?: "banned" | "deleted" | "warned" | "no_action";
  adminNote?: string;
}

export const processReport = (reportId: string, body: ProcessReportPayload) =>
  apiClient
    .patch<{ report: Report }>(`/v1/admin/reports/${reportId}`, body)
    .then((r) => r.data.report);
```

---

## Pages & Components

### ReportsPage

Responsibilities:
- Render a tab bar for status filters: All / Pending / Reviewing / Resolved / Dismissed
- Fetch reports via `listReports` when the filter or page changes
- Pass reports to `ReportTable`
- Open `ProcessReportModal` when an admin clicks a row action

```typescript
// src/pages/ReportsPage.tsx
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { listReports } from "../api/reports";
import ReportTable from "../components/reports/ReportTable";
import ProcessReportModal from "../components/reports/ProcessReportModal";
import type { Report } from "../types/admin";

const STATUS_FILTERS = ["", "pending", "reviewing", "resolved", "dismissed"] as const;
const FILTER_LABELS  = ["All", "Pending", "Reviewing", "Resolved", "Dismissed"];

export default function ReportsPage() {
  const [statusFilter, setStatusFilter] = useState("");
  const [pageToken, setPageToken]       = useState("");
  const [selected, setSelected]         = useState<Report | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["reports", statusFilter, pageToken],
    queryFn:  () => listReports({ pageSize: 20, pageToken, statusFilter }),
  });

  return (
    <div>
      <h1>Reports</h1>
      <div style={{ display: "flex", gap: 8, marginBottom: 16 }}>
        {STATUS_FILTERS.map((f, i) => (
          <button
            key={f}
            onClick={() => { setStatusFilter(f); setPageToken(""); }}
            style={{ fontWeight: statusFilter === f ? "bold" : "normal" }}
          >
            {FILTER_LABELS[i]}
          </button>
        ))}
      </div>

      {isLoading ? (
        <p>Loading…</p>
      ) : (
        <ReportTable
          reports={data?.reports ?? []}
          onAction={setSelected}
        />
      )}

      {data?.nextPageToken && (
        <button onClick={() => setPageToken(data.nextPageToken)}>
          Next page
        </button>
      )}

      {selected && (
        <ProcessReportModal
          report={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </div>
  );
}
```

### ProcessReportModal

Responsibilities:
- Show report details (reporter, target, category, description)
- Form fields: action selector, resolution selector (shown when action = resolve), admin note textarea
- On submit, call `processReport` then invalidate the reports query

```typescript
// src/components/reports/ProcessReportModal.tsx
import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { processReport } from "../../api/reports";
import type { Report } from "../../types/admin";

interface Props {
  report: Report;
  onClose: () => void;
}

export default function ProcessReportModal({ report, onClose }: Props) {
  const qc = useQueryClient();
  const [action, setAction]     = useState<"resolve" | "dismiss" | "review">("review");
  const [resolution, setRes]    = useState<string>("");
  const [adminNote, setNote]    = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      processReport(report.reportId, {
        action,
        resolution: action === "resolve" ? (resolution as any) : undefined,
        adminNote: adminNote || undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["reports"] });
      onClose();
    },
  });

  return (
    <div style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,.5)", display: "flex", alignItems: "center", justifyContent: "center" }}>
      <div style={{ background: "#fff", padding: 24, borderRadius: 8, minWidth: 400 }}>
        <h2>Process Report</h2>
        <p><strong>Target:</strong> {report.targetType} — {report.targetId}</p>
        <p><strong>Category:</strong> {report.reasonCategory}</p>
        <p><strong>Description:</strong> {report.description}</p>

        <label>Action
          <select value={action} onChange={(e) => setAction(e.target.value as any)}>
            <option value="review">Mark as Reviewing</option>
            <option value="resolve">Resolve</option>
            <option value="dismiss">Dismiss</option>
          </select>
        </label>

        {action === "resolve" && (
          <label>Resolution
            <select value={resolution} onChange={(e) => setRes(e.target.value)}>
              <option value="">— select —</option>
              <option value="banned">Ban User</option>
              <option value="deleted">Delete Content</option>
              <option value="warned">Warn User</option>
              <option value="no_action">No Action</option>
            </select>
          </label>
        )}

        <label>Admin Note
          <textarea value={adminNote} onChange={(e) => setNote(e.target.value)} rows={3} style={{ width: "100%" }} />
        </label>

        {mutation.isError && <p style={{ color: "red" }}>Failed to process report. Try again.</p>}

        <div style={{ display: "flex", gap: 8, marginTop: 16 }}>
          <button onClick={() => mutation.mutate()} disabled={mutation.isPending}>
            {mutation.isPending ? "Saving…" : "Submit"}
          </button>
          <button onClick={onClose}>Cancel</button>
        </div>
      </div>
    </div>
  );
}
```

### StatusBadge

Maps status strings to colored labels:

```typescript
// src/components/common/StatusBadge.tsx
const COLORS: Record<string, string> = {
  pending:    "#f59e0b",
  reviewing:  "#3b82f6",
  resolved:   "#10b981",
  dismissed:  "#6b7280",
};

export default function StatusBadge({ status }: { status: string }) {
  return (
    <span style={{
      backgroundColor: COLORS[status] ?? "#e5e7eb",
      color: "#fff",
      padding: "2px 8px",
      borderRadius: 12,
      fontSize: 12,
      fontWeight: 600,
    }}>
      {status}
    </span>
  );
}
```

---

## State Management

| Concern         | Recommended tool         |
|-----------------|--------------------------|
| Auth token      | Zustand store or Context |
| Server data     | TanStack Query           |
| Modal open/close| Local `useState`         |
| Form state      | Local `useState`         |

`src/store/authStore.ts` (Zustand):

```typescript
import { create } from "zustand";
import { persist } from "zustand/middleware";

interface AuthState {
  token: string | null;
  setToken: (t: string) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token:    null,
      setToken: (token) => set({ token }),
      logout:   () => set({ token: null }),
    }),
    { name: "admin-auth" }
  )
);
```

---

## Routing

```typescript
// src/App.tsx
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { useAuthStore } from "./store/authStore";
import LoginPage   from "./pages/LoginPage";
import ReportsPage from "./pages/ReportsPage";

function PrivateRoute({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token);
  return token ? <>{children}</> : <Navigate to="/login" replace />;
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route path="/reports" element={<PrivateRoute><ReportsPage /></PrivateRoute>} />
        <Route path="*" element={<Navigate to="/reports" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
```

---

## Error Handling

| HTTP status | Meaning                        | React action                          |
|-------------|--------------------------------|---------------------------------------|
| 401         | Missing or expired token       | Redirect to `/login`                  |
| 403         | Not an admin                   | Redirect to `/login` with error message |
| 404         | Report / resource not found    | Show inline error                     |
| 400         | Invalid request fields         | Show validation errors in form        |
| 500         | Server error                   | Show generic "Something went wrong"   |
| 501         | Endpoint not implemented yet   | Disable the relevant UI feature       |

The Axios interceptor (see [API Client](#api-client)) handles 401 and 403 globally. All other errors should be caught at the component level via TanStack Query's `isError` / `error` state.

---

## Environment Variables

| Variable             | Description                          | Example                     |
|----------------------|--------------------------------------|-----------------------------|
| `VITE_API_BASE_URL`  | Base URL of the admin HTTP gateway   | `http://localhost:8083`     |

For production, point this at the deployed service URL and ensure CORS is configured on the server side if the frontend and backend are on different origins.

---

## CORS Note

The grpc-gateway server does not currently enable CORS headers. If the React app is served from a different origin during development, either:

1. **Proxy via Vite** — add to `vite.config.ts`:

```typescript
export default {
  server: {
    proxy: {
      "/v1": "http://localhost:8083",
    },
  },
};
```

2. **Add CORS middleware** on the Go server in `cmd/server/main.go` using `rs/cors` or a custom handler.

The proxy approach is zero-config for development.

---

## Currently Implemented vs Planned

| Feature               | Status              |
|-----------------------|---------------------|
| List reports          | ✅ Implemented       |
| Process report        | ✅ Implemented       |
| Moderation audit log  | ✅ DB only (no API)  |
| List users            | ⚠️ Not implemented   |
| Ban / unban user      | ⚠️ Not implemented   |
| Update deck status    | ⚠️ Not implemented   |

Build the Reports page first. Stub out the Users and Decks pages with a "Coming soon" message so the navigation is already in place when those endpoints are added.
