// Typed client for the /connections REST surface.
import { api } from "@/lib/api";

export interface ConnectionDTO {
  id: string;
  name: string;
  hosts: string[];
  port: number;
  datacenter?: string;
  default_keyspace?: string;
  auth_username?: string;
  has_password: boolean;
  tls_enabled: boolean;
  tls_skip_verify: boolean;
  has_tls_ca: boolean;
  has_tls_client_cert: boolean;
  read_only: boolean;
  consistency: string;
  connect_timeout_ms: number;
  request_timeout_ms: number;
  created_at: string;
  updated_at: string;
  last_used_at?: string;
}

export interface CreateConnectionRequest {
  name: string;
  hosts: string[];
  port: number;
  datacenter?: string;
  default_keyspace?: string;
  auth_username?: string;
  password?: string;
  tls_enabled?: boolean;
  tls_skip_verify?: boolean;
  tls_ca_cert?: string;
  tls_client_cert?: string;
  tls_client_key?: string;
  read_only?: boolean;
  consistency?: string;
  connect_timeout_ms?: number;
  request_timeout_ms?: number;
}

// UpdateConnectionRequest mirrors the server: every field is optional and
// missing fields are left untouched. For `password`/`tls_client_key`:
// undefined → retain ciphertext; "" → clear; non-empty → re-encrypt.
export type UpdateConnectionRequest = Partial<CreateConnectionRequest>;

export interface TestResult {
  ok: boolean;
  nodes_up?: number;
  cluster_name?: string;
  version?: string;
  partitioner?: string;
  latency_ms?: number;
  error?: string;
}

export interface ConnectionStatus {
  pooled: boolean;
  last_used_at?: string;
}

export const listConnections = () => api<ConnectionDTO[]>("/connections");

export const getConnection = (id: string) => api<ConnectionDTO>(`/connections/${id}`);

export const createConnection = (req: CreateConnectionRequest) =>
  api<ConnectionDTO>("/connections", { method: "POST", body: req });

export const updateConnection = (id: string, req: UpdateConnectionRequest) =>
  api<ConnectionDTO>(`/connections/${id}`, { method: "PUT", body: req });

export const deleteConnection = (id: string) =>
  api<void>(`/connections/${id}`, { method: "DELETE" });

export const testUnsavedConnection = (req: CreateConnectionRequest) =>
  api<TestResult>("/connections/test", { method: "POST", body: req });

export const testSavedConnection = (id: string) =>
  api<TestResult>(`/connections/${id}/test`, { method: "POST" });

export const disconnectConnection = (id: string) =>
  api<void>(`/connections/${id}/disconnect`, { method: "POST" });

export const getConnectionStatus = (id: string) =>
  api<ConnectionStatus>(`/connections/${id}/status`);
