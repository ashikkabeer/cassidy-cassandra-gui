import * as React from "react";
import {
  ChevronDown,
  ChevronRight,
  Eye,
  Folder,
  Lock,
  Zap,
  Check,
  X,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field } from "@/components/ui/field";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { Separator } from "@/components/ui/separator";
import { Panel } from "@/components/ui/card";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Kbd, Spinner } from "@/components/primitives";
import {
  type ConnectionDTO,
  type CreateConnectionRequest,
  type TestResult,
  createConnection,
  updateConnection,
  testUnsavedConnection,
} from "@/lib/connections";
import { APIError } from "@/lib/api";

function SectionHeading({ n, children }: { n: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="mb-2.5 mt-3.5 flex items-center gap-2">
      <Kbd>{n}</Kbd>
      <span className="text-[12.5px] font-semibold">{children}</span>
      <Separator className="flex-1" />
    </div>
  );
}

function HostsField({
  value,
  onChange,
}: {
  value: string[];
  onChange: (v: string[]) => void;
}) {
  const [pending, setPending] = React.useState("");
  const commit = () => {
    // Accept comma/space/newline-separated pastes as multiple hosts.
    const parts = pending
      .split(/[\s,]+/)
      .map((p) => p.trim())
      .filter(Boolean);
    if (parts.length === 0) return;
    const next = [...value];
    for (const p of parts) {
      if (!next.includes(p)) next.push(p);
    }
    onChange(next);
    setPending("");
  };
  return (
    <div className="flex min-h-[28px] flex-wrap items-start gap-1 rounded-[var(--radius)] border border-[hsl(var(--border-strong))] bg-background px-1.5 py-1">
      {value.map((h) => (
        <span
          key={h}
          className="mono m-px inline-flex items-center gap-1 rounded-sm bg-secondary px-1.5 py-px text-[11px]"
        >
          {h}
          <button
            type="button"
            onClick={() => onChange(value.filter((x) => x !== h))}
            className="opacity-60 hover:opacity-100"
            aria-label={`Remove ${h}`}
          >
            <X size={9} strokeWidth={2} />
          </button>
        </span>
      ))}
      <input
        value={pending}
        onChange={(e) => setPending(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === ",") {
            e.preventDefault();
            commit();
          } else if (e.key === "Backspace" && !pending && value.length > 0) {
            onChange(value.slice(0, -1));
          }
        }}
        onBlur={commit}
        placeholder="add host…"
        className="min-w-[60px] flex-1 bg-transparent text-[11.5px] outline-none"
      />
    </div>
  );
}

interface FormState {
  name: string;
  hosts: string[];
  port: number;
  datacenter: string;
  default_keyspace: string;
  auth_username: string;
  password: string;
  passwordTouched: boolean;
  tls_enabled: boolean;
  tls_skip_verify: boolean;
  tls_ca_cert: string;
  tls_client_cert: string;
  tls_client_key: string;
  clientKeyTouched: boolean;
  read_only: boolean;
  consistency: string;
  connect_timeout_ms: number;
  request_timeout_ms: number;
}

function blankState(): FormState {
  return {
    name: "",
    hosts: [],
    port: 9042,
    datacenter: "",
    default_keyspace: "",
    auth_username: "",
    password: "",
    passwordTouched: false,
    tls_enabled: false,
    tls_skip_verify: false,
    tls_ca_cert: "",
    tls_client_cert: "",
    tls_client_key: "",
    clientKeyTouched: false,
    read_only: false,
    consistency: "LOCAL_QUORUM",
    connect_timeout_ms: 10000,
    request_timeout_ms: 15000,
  };
}

function stateFromConnection(c: ConnectionDTO): FormState {
  return {
    name: c.name,
    hosts: c.hosts,
    port: c.port,
    datacenter: c.datacenter ?? "",
    default_keyspace: c.default_keyspace ?? "",
    auth_username: c.auth_username ?? "",
    password: "",
    passwordTouched: false,
    tls_enabled: c.tls_enabled,
    tls_skip_verify: c.tls_skip_verify,
    tls_ca_cert: "",
    tls_client_cert: "",
    tls_client_key: "",
    clientKeyTouched: false,
    read_only: c.read_only,
    consistency: c.consistency,
    connect_timeout_ms: c.connect_timeout_ms,
    request_timeout_ms: c.request_timeout_ms,
  };
}

function buildCreateRequest(s: FormState): CreateConnectionRequest {
  return {
    name: s.name,
    hosts: s.hosts,
    port: s.port,
    datacenter: s.datacenter,
    default_keyspace: s.default_keyspace,
    auth_username: s.auth_username,
    password: s.password,
    tls_enabled: s.tls_enabled,
    tls_skip_verify: s.tls_skip_verify,
    tls_ca_cert: s.tls_ca_cert,
    tls_client_cert: s.tls_client_cert,
    tls_client_key: s.tls_client_key,
    read_only: s.read_only,
    consistency: s.consistency,
    connect_timeout_ms: s.connect_timeout_ms,
    request_timeout_ms: s.request_timeout_ms,
  };
}

export interface ConnectionFormProps {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  /** When editing, the existing connection. Omit to create a new one. */
  existing?: ConnectionDTO | null;
  onSaved?: (c: ConnectionDTO) => void;
}

export function ConnectionForm({
  open,
  onOpenChange,
  existing,
  onSaved,
}: ConnectionFormProps) {
  const [state, setState] = React.useState<FormState>(blankState);
  const [tlsOpen, setTlsOpen] = React.useState(false);
  const [testResult, setTestResult] = React.useState<TestResult | null>(null);
  const [testing, setTesting] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  // Reset every time the dialog opens with a (possibly different) target.
  React.useEffect(() => {
    if (!open) return;
    if (existing) {
      setState(stateFromConnection(existing));
      setTlsOpen(existing.tls_enabled);
    } else {
      setState(blankState());
      setTlsOpen(false);
    }
    setTestResult(null);
    setError(null);
  }, [open, existing]);

  const update = (patch: Partial<FormState>) =>
    setState((prev) => ({ ...prev, ...patch }));

  const handleTest = async () => {
    setTesting(true);
    setTestResult(null);
    setError(null);
    try {
      // Build a CreateConnectionRequest from current form state, regardless of
      // whether we're editing or creating — the server uses this body to build
      // a throwaway session, not to mutate the DB.
      const res = await testUnsavedConnection(buildCreateRequest(state));
      setTestResult(res);
    } catch (err) {
      setTestResult({
        ok: false,
        error: err instanceof APIError ? err.message : "Test failed",
      });
    } finally {
      setTesting(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      let saved: ConnectionDTO;
      if (existing) {
        // For updates, omit password/client-key when they haven't been touched
        // so the server retains the existing ciphertext.
        const req: Partial<CreateConnectionRequest> = {
          name: state.name,
          hosts: state.hosts,
          port: state.port,
          datacenter: state.datacenter,
          default_keyspace: state.default_keyspace,
          auth_username: state.auth_username,
          tls_enabled: state.tls_enabled,
          tls_skip_verify: state.tls_skip_verify,
          tls_ca_cert: state.tls_ca_cert,
          tls_client_cert: state.tls_client_cert,
          read_only: state.read_only,
          consistency: state.consistency,
          connect_timeout_ms: state.connect_timeout_ms,
          request_timeout_ms: state.request_timeout_ms,
        };
        if (state.passwordTouched) req.password = state.password;
        if (state.clientKeyTouched) req.tls_client_key = state.tls_client_key;
        saved = await updateConnection(existing.id, req);
      } else {
        saved = await createConnection(buildCreateRequest(state));
      }
      onSaved?.(saved);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Save failed");
    } finally {
      setSaving(false);
    }
  };

  const canSave = state.name.trim().length > 0 && state.hosts.length > 0 && !saving;

  const footer = (
    <>
      <div className="flex flex-1 items-center gap-2">
        <Button
          variant="outline"
          size="md"
          disabled={testing || state.hosts.length === 0}
          onClick={handleTest}
        >
          {testing ? <Spinner size={12} /> : <Zap size={12} strokeWidth={1.6} />} Test connection
        </Button>
        {testResult?.ok && (
          <span className="inline-flex items-center gap-1.5 text-[11.5px] text-[hsl(var(--success))]">
            <Check size={12} strokeWidth={2.2} />
            Connected
            {testResult.nodes_up != null && ` · ${testResult.nodes_up} node${testResult.nodes_up === 1 ? "" : "s"}`}
            {testResult.latency_ms != null && ` · ${testResult.latency_ms}ms`}
            {testResult.version && ` · CQL ${testResult.version}`}
          </span>
        )}
        {testResult && !testResult.ok && (
          <span className="inline-flex items-center gap-1.5 truncate text-[11.5px] text-[hsl(0_80%_75%)]">
            <X size={12} strokeWidth={2.2} />
            {testResult.error ?? "Connection failed"}
          </span>
        )}
      </div>
      <Button variant="ghost" size="md" onClick={() => onOpenChange(false)}>
        Cancel
      </Button>
      <Button variant="default" size="md" disabled={!canSave} onClick={handleSave}>
        {saving ? <Spinner size={12} /> : null}
        {existing ? "Save changes" : "Save connection"}
      </Button>
    </>
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        width={640}
        title={existing ? "Edit connection" : "New connection"}
        subtitle={
          existing ? `Connection ${existing.name}` : "Connect to a Cassandra cluster"
        }
        footer={footer}
      >
        {error && (
          <div className="mb-3 rounded-[var(--radius)] border border-[hsl(var(--destructive)/0.35)] bg-[hsl(var(--destructive)/0.12)] px-2.5 py-2 text-[11.5px] text-[hsl(0_80%_80%)]">
            {error}
          </div>
        )}

        <SectionHeading n={1}>Basics</SectionHeading>
        <div className="grid grid-cols-[1fr_110px] gap-2.5">
          <Field label="Display name" hint="required">
            <Input
              value={state.name}
              onChange={(e) => update({ name: e.target.value })}
              placeholder="prod-eu-west"
            />
          </Field>
          <Field label="Port">
            <Input
              value={String(state.port)}
              onChange={(e) => update({ port: Number(e.target.value) || 0 })}
              className="mono"
              inputMode="numeric"
            />
          </Field>
        </div>
        <div className="mt-2.5">
          <Field label="Contact points" hint="enter or comma to add">
            <HostsField value={state.hosts} onChange={(v) => update({ hosts: v })} />
          </Field>
        </div>
        <div className="mt-2.5 grid grid-cols-2 gap-2.5">
          <Field label="Local datacenter" help="Required for DC-aware routing">
            <Input
              value={state.datacenter}
              onChange={(e) => update({ datacenter: e.target.value })}
              placeholder="datacenter1"
              className="mono"
            />
          </Field>
          <Field label="Default keyspace" hint="optional">
            <Input
              value={state.default_keyspace}
              onChange={(e) => update({ default_keyspace: e.target.value })}
              className="mono"
            />
          </Field>
        </div>

        <SectionHeading n={2}>Authentication</SectionHeading>
        <div className="grid grid-cols-2 gap-2.5">
          <Field label="Username">
            <Input
              value={state.auth_username}
              onChange={(e) => update({ auth_username: e.target.value })}
              className="mono"
            />
          </Field>
          <Field
            label="Password"
            help={
              existing && !state.passwordTouched
                ? existing.has_password
                  ? "Leave blank to keep existing password"
                  : undefined
                : undefined
            }
          >
            <Input
              type="password"
              value={state.password}
              onChange={(e) =>
                update({ password: e.target.value, passwordTouched: true })
              }
              placeholder={
                existing && existing.has_password ? "••••••••••" : "password"
              }
              suffix={<Eye size={12} strokeWidth={1.6} />}
            />
          </Field>
        </div>

        <button
          type="button"
          onClick={() => setTlsOpen(!tlsOpen)}
          className="mb-2.5 mt-3.5 flex w-full cursor-pointer items-center gap-2"
        >
          <Kbd>3</Kbd>
          <span className="text-[12.5px] font-semibold">TLS</span>
          <Badge variant={state.tls_enabled ? "info" : "outline"}>
            {state.tls_enabled ? "Enabled" : "Off"}
          </Badge>
          <Separator className="flex-1" />
          {tlsOpen ? (
            <ChevronDown size={12} strokeWidth={1.8} className="text-muted-foreground" />
          ) : (
            <ChevronRight size={12} strokeWidth={1.8} className="text-muted-foreground" />
          )}
        </button>

        {tlsOpen && (
          <Panel className="p-3">
            <div className="mb-2 flex items-center justify-between">
              <div>
                <div className="text-[12px] font-medium">Enable TLS</div>
                <div className="text-[11px] text-muted-foreground">
                  Encrypted client-to-node connection
                </div>
              </div>
              <Switch
                checked={state.tls_enabled}
                onCheckedChange={(v) => update({ tls_enabled: v })}
              />
            </div>
            <Separator className="my-1.5" />
            <div className="mb-2 mt-2 flex items-center justify-between">
              <div>
                <div className="text-[12px] font-medium">Skip certificate verification</div>
                <div className="text-[11px] text-muted-foreground">
                  Not recommended outside local dev
                </div>
              </div>
              <Switch
                checked={state.tls_skip_verify}
                disabled={!state.tls_enabled}
                onCheckedChange={(v) => update({ tls_skip_verify: v })}
              />
            </div>
            <Separator className="my-1.5" />
            <Field label="CA certificate (PEM)">
              <Input
                value={state.tls_ca_cert}
                onChange={(e) => update({ tls_ca_cert: e.target.value })}
                placeholder="-----BEGIN CERTIFICATE-----"
                className="mono"
                suffix={<Folder size={12} strokeWidth={1.6} />}
              />
            </Field>
            <div className="mt-2.5 grid grid-cols-2 gap-2.5">
              <Field label="Client certificate (PEM)">
                <Input
                  value={state.tls_client_cert}
                  onChange={(e) => update({ tls_client_cert: e.target.value })}
                  placeholder="optional"
                  className="mono"
                />
              </Field>
              <Field
                label="Client key (PEM)"
                help={
                  existing && !state.clientKeyTouched && existing.has_tls_client_cert
                    ? "Leave blank to keep existing key"
                    : undefined
                }
              >
                <Input
                  type="password"
                  value={state.tls_client_key}
                  onChange={(e) =>
                    update({ tls_client_key: e.target.value, clientKeyTouched: true })
                  }
                  placeholder="optional"
                  className="mono"
                />
              </Field>
            </div>
          </Panel>
        )}

        <SectionHeading n={4}>Advanced</SectionHeading>
        <div className="grid grid-cols-3 gap-2.5">
          <Field label="Consistency">
            <Select
              value={state.consistency}
              onValueChange={(v) => update({ consistency: v })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {[
                  "ONE",
                  "LOCAL_ONE",
                  "TWO",
                  "THREE",
                  "QUORUM",
                  "LOCAL_QUORUM",
                  "EACH_QUORUM",
                  "ALL",
                  "ANY",
                ].map((c) => (
                  <SelectItem key={c} value={c}>
                    {c}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </Field>
          <Field label="Connect timeout" hint="ms">
            <Input
              value={String(state.connect_timeout_ms)}
              onChange={(e) =>
                update({ connect_timeout_ms: Number(e.target.value) || 0 })
              }
              className="mono"
              inputMode="numeric"
            />
          </Field>
          <Field label="Request timeout" hint="ms">
            <Input
              value={String(state.request_timeout_ms)}
              onChange={(e) =>
                update({ request_timeout_ms: Number(e.target.value) || 0 })
              }
              className="mono"
              inputMode="numeric"
            />
          </Field>
        </div>

        <div className="mt-4 flex items-center gap-3 rounded-[var(--radius)] border border-[hsl(var(--warning)/0.3)] bg-[hsl(var(--warning)/0.08)] p-3">
          <div className="flex h-[30px] w-[30px] shrink-0 items-center justify-center rounded-md bg-[hsl(var(--warning)/0.15)] text-[hsl(var(--warning))]">
            <Lock size={14} strokeWidth={1.8} />
          </div>
          <div className="flex-1">
            <div className="text-[12.5px] font-semibold">
              Treat this connection as read-only
            </div>
            <div className="mt-0.5 text-[11px] text-muted-foreground">
              Disables INSERT / UPDATE / DELETE / DDL across all UI. Statement-level
              guards apply server-side too.
            </div>
          </div>
          <Switch
            checked={state.read_only}
            onCheckedChange={(v) => update({ read_only: v })}
          />
        </div>
      </DialogContent>
    </Dialog>
  );
}
