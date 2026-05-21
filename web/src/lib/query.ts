// Typed client for /connections/{id}/query + /query-history + /completion.
import { api } from "@/lib/api";

export interface RunRequest {
  cql: string;
  keyspace?: string;
  page_size?: number;
  page_state?: string;
  consistency?: string;
}

export interface ColumnInfo {
  name: string;
  type: string;
}

export interface QueryResult {
  columns: ColumnInfo[];
  rows: unknown[][];
  next_page_state?: string;
  applied: boolean;
  row_count: number;
  duration_ms: number;
  warnings?: string[];
  statement_kind: string;
}

export interface HistoryEntry {
  id: string;
  connection_id?: string;
  keyspace?: string;
  cql: string;
  statement_kind: string;
  success: boolean;
  error_code?: string;
  error_message?: string;
  row_count: number;
  duration_ms: number;
  executed_at: string;
}

export type CompletionKind =
  | "keyspace"
  | "table"
  | "column"
  | "keyword"
  | "type"
  | "function";

export interface CompletionSuggestion {
  label: string;
  detail?: string;
  kind: CompletionKind;
}

export const runQuery = (connId: string, req: RunRequest) =>
  api<QueryResult>(`/connections/${connId}/query`, { method: "POST", body: req });

// exportQuery returns the raw Response so the caller can stream/save it. Note
// we still pipe through the same CSRF logic via api(), but bypass JSON parsing
// since the server emits CSV/NDJSON.
export async function exportQuery(
  connId: string,
  req: RunRequest,
  format: "csv" | "json",
): Promise<Response> {
  // Read the CSRF cookie ourselves — api() consumes JSON which we don't want.
  const csrf =
    document.cookie
      .split(";")
      .map((c) => c.trim())
      .find((c) => c.startsWith("cassidy_csrf="))
      ?.slice("cassidy_csrf=".length) ?? "";
  return fetch(`/api/v1/connections/${connId}/query/export?format=${format}`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(csrf ? { "X-CSRF-Token": decodeURIComponent(csrf) } : {}),
    },
    body: JSON.stringify(req),
  });
}

export const listHistory = (params: {
  connection_id?: string;
  kind?: string;
  success?: boolean;
  limit?: number;
  before?: string; // RFC3339 cursor: returns entries executed strictly before this
}) => {
  const q = new URLSearchParams();
  if (params.connection_id) q.set("connection_id", params.connection_id);
  if (params.kind) q.set("kind", params.kind);
  if (params.success != null) q.set("success", String(params.success));
  if (params.limit) q.set("limit", String(params.limit));
  if (params.before) q.set("before", params.before);
  const qs = q.toString();
  return api<HistoryEntry[]>(`/query-history${qs ? "?" + qs : ""}`);
};

export const deleteHistory = (id: string) =>
  api<void>(`/query-history/${id}`, { method: "DELETE" });

export const getCompletion = (
  connId: string,
  prefix: string,
  keyspace?: string,
) => {
  const q = new URLSearchParams({ prefix });
  if (keyspace) q.set("keyspace", keyspace);
  return api<CompletionSuggestion[]>(
    `/connections/${connId}/completion?${q.toString()}`,
  );
};
