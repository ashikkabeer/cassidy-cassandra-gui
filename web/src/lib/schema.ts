// Typed client for the /connections/{id}/... schema introspection surface.
import { api } from "@/lib/api";

export interface Peer {
  address: string;
  dc?: string;
  version?: string;
}

export interface ClusterInfo {
  cluster_name: string;
  version: string;
  partitioner: string;
  local_dc: string;
  host_count: number;
  peers?: Peer[];
}

export interface Replication {
  class: string;
  factor?: number;
  dc_factors?: Record<string, number>;
  raw?: Record<string, string>;
}

export interface Keyspace {
  name: string;
  durable_writes: boolean;
  replication: Replication;
  system: boolean;
}

export interface TableSummary {
  name: string;
  comment?: string;
  default_ttl: number;
  gc_grace: number;
  compaction_class?: string;
}

export type ColumnKind = "partition_key" | "clustering" | "regular" | "static";

export interface Column {
  name: string;
  type: string;
  kind: ColumnKind;
  position: number;
  clustering_order?: string;
}

export interface SchemaIndex {
  name: string;
  kind: string;
  options?: Record<string, string>;
}

export interface TableDetail {
  keyspace: string;
  name: string;
  columns: Column[];
  indexes?: SchemaIndex[];
  comment?: string;
  default_ttl: number;
  gc_grace_seconds: number;
  caching?: Record<string, string>;
  compaction?: Record<string, string>;
  compression?: Record<string, string>;
  bloom_filter_fp_chance?: number;
  speculative_retry?: string;
  flags?: string[];
}

export interface UDT {
  name: string;
  fields: { name: string; type: string }[];
}

export const getClusterInfo = (id: string) =>
  api<ClusterInfo>(`/connections/${id}/cluster-info`);

export const listKeyspaces = (id: string) =>
  api<Keyspace[]>(`/connections/${id}/keyspaces`);

export const listTables = (id: string, ks: string) =>
  api<TableSummary[]>(`/connections/${id}/keyspaces/${encodeURIComponent(ks)}/tables`);

export const getTable = (id: string, ks: string, t: string) =>
  api<TableDetail>(
    `/connections/${id}/keyspaces/${encodeURIComponent(ks)}/tables/${encodeURIComponent(t)}`,
  );

export const getTableDDL = (id: string, ks: string, t: string) =>
  api<{ ddl: string }>(
    `/connections/${id}/keyspaces/${encodeURIComponent(ks)}/tables/${encodeURIComponent(t)}/ddl`,
  );

export const listTypes = (id: string, ks: string) =>
  api<UDT[]>(`/connections/${id}/keyspaces/${encodeURIComponent(ks)}/types`);
