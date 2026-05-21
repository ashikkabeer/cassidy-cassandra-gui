// Typed client for the table-data browse + edit endpoints.
import { api } from "@/lib/api";

export interface ColumnMeta {
  name: string;
  type: string;
  kind: "partition_key" | "clustering" | "regular" | "static";
  editable: boolean;
}

export interface RowsPage {
  columns: ColumnMeta[];
  rows: unknown[][];
  next_page_state?: string;
}

export interface Op {
  kind: "insert" | "update" | "delete";
  pk: Record<string, unknown>;
  set?: Record<string, unknown>;
}

export interface ChangeSet {
  ops: Op[];
}

export interface PreviewResponse {
  cql: string;
  delete_count: number;
  statement_count: number;
}

export interface CommitResponse {
  applied: boolean;
  statement_count: number;
}

export const getRows = (
  connId: string,
  ks: string,
  tbl: string,
  opts: { pageSize?: number; pageState?: string } = {},
) => {
  const q = new URLSearchParams();
  if (opts.pageSize) q.set("page_size", String(opts.pageSize));
  if (opts.pageState) q.set("page_state", opts.pageState);
  const qs = q.toString();
  return api<RowsPage>(
    `/connections/${connId}/keyspaces/${encodeURIComponent(ks)}/tables/${encodeURIComponent(tbl)}/rows${qs ? "?" + qs : ""}`,
  );
};

export const previewChanges = (connId: string, ks: string, tbl: string, cs: ChangeSet) =>
  api<PreviewResponse>(
    `/connections/${connId}/keyspaces/${encodeURIComponent(ks)}/tables/${encodeURIComponent(tbl)}/rows/preview`,
    { method: "POST", body: cs },
  );

export const commitChanges = (connId: string, ks: string, tbl: string, cs: ChangeSet) =>
  api<CommitResponse>(
    `/connections/${connId}/keyspaces/${encodeURIComponent(ks)}/tables/${encodeURIComponent(tbl)}/rows/commit`,
    { method: "POST", body: cs },
  );
