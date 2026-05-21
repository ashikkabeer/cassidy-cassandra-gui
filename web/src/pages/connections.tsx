import * as React from "react";
import {
  Database,
  Search,
  Filter,
  Plus,
  MoreHorizontal,
  Lock,
  AlertTriangle,
  RefreshCw,
} from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/primitives";
import { ConnectionForm } from "@/components/connection-form";
import {
  Dialog,
  DialogContent,
} from "@/components/ui/dialog";
import {
  type ConnectionDTO,
  deleteConnection,
  listConnections,
  testSavedConnection,
  type TestResult,
} from "@/lib/connections";
import { useActiveConnection } from "@/lib/active-connection";
import { cn } from "@/lib/utils";
import { APIError } from "@/lib/api";
import { useNavigate } from "react-router-dom";

type RowStatus = "idle" | "testing" | "ok" | "err";

interface ConnCardProps {
  c: ConnectionDTO;
  status: RowStatus;
  testResult?: TestResult | null;
  isActive: boolean;
  onTest: () => void;
  onEdit: () => void;
  onConnect: () => void;
  onDelete: () => void;
}

function fromNow(iso?: string): string {
  if (!iso) return "Never";
  const t = new Date(iso).getTime();
  const diff = Date.now() - t;
  if (diff < 60_000) return "now";
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)} min ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3600_000)} hr ago`;
  return `${Math.floor(diff / 86_400_000)} days ago`;
}

function statusDot(status: RowStatus, isActive: boolean): "green" | "amber" | "red" | "grey" {
  if (status === "err") return "red";
  if (status === "ok") return "green";
  if (status === "testing") return "amber";
  return isActive ? "green" : "grey";
}

function ConnCard({
  c,
  status,
  testResult,
  isActive,
  onTest,
  onEdit,
  onConnect,
  onDelete,
}: ConnCardProps) {
  const [menuOpen, setMenuOpen] = React.useState(false);
  const dot = statusDot(status, isActive);
  const errLine = status === "err" ? testResult?.error ?? "Connection failed" : null;
  return (
    <Card className={cn("relative p-3.5", status === "err" && "border-[hsl(var(--destructive)/0.35)]")}>
      <div className="flex items-start gap-2.5">
        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md bg-secondary text-muted-foreground">
          <Database size={14} strokeWidth={1.6} />
        </div>
        <div className="min-w-0 flex-1">
          <div className="mb-px flex items-center gap-1.5">
            <span className={cn("dot", `dot-${dot}`)} />
            <span className="text-[13px] font-semibold">{c.name}</span>
            {c.read_only && (
              <Badge variant="warning" icon={<Lock size={9} strokeWidth={2} />}>
                Read-only
              </Badge>
            )}
            {isActive && <Badge variant="info">Active</Badge>}
          </div>
          <div className="mono truncate text-[10.5px] text-muted-foreground">
            {c.hosts.join(", ")}
          </div>
        </div>
        <div className="relative">
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Connection actions"
            onClick={() => setMenuOpen((v) => !v)}
          >
            <MoreHorizontal size={14} strokeWidth={2.4} />
          </Button>
          {menuOpen && (
            <>
              <div className="fixed inset-0 z-30" onClick={() => setMenuOpen(false)} />
              <div className="absolute right-0 top-7 z-40 w-36 rounded-[var(--radius)] border bg-popover py-1 shadow-[0_8px_24px_rgba(0,0,0,.4)]">
                <button
                  type="button"
                  className="block w-full px-3 py-1.5 text-left text-[12px] hover:bg-accent"
                  onClick={() => {
                    setMenuOpen(false);
                    onEdit();
                  }}
                >
                  Edit
                </button>
                <button
                  type="button"
                  className="block w-full px-3 py-1.5 text-left text-[12px] text-[hsl(0_80%_75%)] hover:bg-accent"
                  onClick={() => {
                    setMenuOpen(false);
                    onDelete();
                  }}
                >
                  Delete
                </button>
              </div>
            </>
          )}
        </div>
      </div>

      <div className="mt-3 grid grid-cols-2 gap-1.5 text-[11px]">
        <div className="text-muted-foreground">Datacenter</div>
        <div className="mono text-right">{c.datacenter || "—"}</div>
        <div className="text-muted-foreground">User</div>
        <div className="mono text-right">{c.auth_username || "—"}</div>
        <div className="text-muted-foreground">Default keyspace</div>
        <div className="mono truncate text-right">{c.default_keyspace || "—"}</div>
        <div className="text-muted-foreground">TLS</div>
        <div className="mono text-right">{c.tls_enabled ? "on" : "off"}</div>
      </div>

      <Separator className="my-3" />

      <div className="flex items-center justify-between">
        <span className="text-[11px] text-muted-foreground">Used {fromNow(c.last_used_at)}</span>
        <div className="flex gap-1">
          <Button size="sm" variant="ghost" disabled={status === "testing"} onClick={onTest}>
            {status === "testing" ? "Testing…" : "Test"}
          </Button>
          <Button size="sm" variant="outline" onClick={onEdit}>
            Edit
          </Button>
          <Button size="sm" variant={isActive ? "ghost" : "default"} onClick={onConnect}>
            {isActive ? "Active" : "Connect"}
          </Button>
        </div>
      </div>

      {errLine && (
        <div className="mt-2.5 flex items-center gap-1.5 truncate rounded-sm border border-[hsl(var(--destructive)/0.3)] bg-[hsl(var(--destructive)/0.12)] px-2 py-1.5 text-[11px] text-[hsl(0_80%_78%)]">
          <AlertTriangle size={11} strokeWidth={2} />
          {errLine}
        </div>
      )}
      {status === "ok" && testResult && (
        <div className="mt-2.5 flex items-center gap-1.5 truncate rounded-sm border border-[hsl(var(--success)/0.3)] bg-[hsl(var(--success)/0.12)] px-2 py-1.5 text-[11px] text-[hsl(var(--success))]">
          Connected{testResult.nodes_up != null && ` · ${testResult.nodes_up} node${testResult.nodes_up === 1 ? "" : "s"}`}
          {testResult.version && ` · CQL ${testResult.version}`}
          {testResult.latency_ms != null && ` · ${testResult.latency_ms}ms`}
        </div>
      )}
    </Card>
  );
}

function ConnSkeleton() {
  return (
    <Card className="p-3.5">
      <div className="flex items-center gap-2.5">
        <Skeleton w={28} h={28} className="rounded-md" />
        <div className="flex-1">
          <Skeleton w={120} h={12} />
          <div className="h-1.5" />
          <Skeleton w={180} h={10} />
        </div>
      </div>
      <div className="mt-3.5 grid grid-cols-2 gap-2">
        <Skeleton w="40%" h={10} />
        <Skeleton w="50%" h={10} className="justify-self-end" />
        <Skeleton w="30%" h={10} />
        <Skeleton w="60%" h={10} className="justify-self-end" />
        <Skeleton w="50%" h={10} />
        <Skeleton w="40%" h={10} className="justify-self-end" />
      </div>
      <Separator className="my-3" />
      <div className="flex justify-between">
        <Skeleton w={80} h={10} />
        <Skeleton w={140} h={22} className="rounded-sm" />
      </div>
    </Card>
  );
}

interface ConnState {
  status: RowStatus;
  result?: TestResult | null;
}

export function ConnectionsPage() {
  const [connections, setConnections] = React.useState<ConnectionDTO[] | null>(null);
  const [loadErr, setLoadErr] = React.useState<string | null>(null);
  const [filter, setFilter] = React.useState("");
  const [formOpen, setFormOpen] = React.useState(false);
  const [editing, setEditing] = React.useState<ConnectionDTO | null>(null);
  const [rowState, setRowState] = React.useState<Record<string, ConnState>>({});
  const [deleteTarget, setDeleteTarget] = React.useState<ConnectionDTO | null>(null);
  const [deleting, setDeleting] = React.useState(false);
  const { activeId, setActive } = useActiveConnection();
  const navigate = useNavigate();

  const load = React.useCallback(async () => {
    setLoadErr(null);
    try {
      setConnections(await listConnections());
    } catch (err) {
      setConnections([]);
      setLoadErr(err instanceof APIError ? err.message : "Failed to load");
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const filtered = (connections ?? []).filter((c) => {
    if (!filter) return true;
    const f = filter.toLowerCase();
    return (
      c.name.toLowerCase().includes(f) ||
      c.hosts.some((h) => h.toLowerCase().includes(f)) ||
      (c.datacenter ?? "").toLowerCase().includes(f)
    );
  });

  const handleNew = () => {
    setEditing(null);
    setFormOpen(true);
  };
  const handleEdit = (c: ConnectionDTO) => {
    setEditing(c);
    setFormOpen(true);
  };
  const handleTest = async (c: ConnectionDTO) => {
    setRowState((s) => ({ ...s, [c.id]: { status: "testing" } }));
    try {
      const res = await testSavedConnection(c.id);
      setRowState((s) => ({
        ...s,
        [c.id]: { status: res.ok ? "ok" : "err", result: res },
      }));
    } catch (err) {
      setRowState((s) => ({
        ...s,
        [c.id]: {
          status: "err",
          result: { ok: false, error: err instanceof APIError ? err.message : "Test failed" },
        },
      }));
    }
  };
  const handleConnect = (c: ConnectionDTO) => {
    setActive(c.id);
    navigate("/workspace");
  };
  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteConnection(deleteTarget.id);
      if (activeId === deleteTarget.id) setActive(null);
      await load();
      setDeleteTarget(null);
    } catch (err) {
      setLoadErr(err instanceof APIError ? err.message : "Delete failed");
    } finally {
      setDeleting(false);
    }
  };

  const isLoading = connections === null && !loadErr;
  const isEmpty = !isLoading && !loadErr && (connections?.length ?? 0) === 0;

  return (
    <AppShell>
      <div className="flex h-full flex-col overflow-hidden">
        <div className="flex items-center gap-2 border-b px-5 py-3.5">
          <div>
            <div className="text-[16px] font-semibold tracking-[-0.2px]">Connections</div>
            <div className="mt-0.5 text-[11.5px] text-muted-foreground">
              {connections != null
                ? `${connections.length} saved · click Connect to switch`
                : "Saved Cassandra clusters"}
            </div>
          </div>
          <div className="flex-1" />
          <Input
            wrapperClassName="w-[220px] bg-panel"
            placeholder="Filter connections"
            icon={<Search size={12} strokeWidth={1.8} />}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          <Button variant="outline" size="md">
            <Filter size={12} strokeWidth={1.6} /> All
          </Button>
          <Button variant="default" size="md" onClick={handleNew}>
            <Plus size={12} strokeWidth={2.2} /> New connection
          </Button>
        </div>

        <div className="flex-1 overflow-auto p-5">
          {loadErr && (
            <div className="mx-auto mt-10 flex max-w-[480px] flex-col items-center gap-3.5 text-center">
              <div className="flex h-12 w-12 items-center justify-center rounded-full border border-[hsl(var(--destructive)/0.35)] bg-[hsl(var(--destructive)/0.12)] text-[hsl(0_80%_75%)]">
                <AlertTriangle size={22} strokeWidth={1.8} />
              </div>
              <div>
                <div className="text-[14px] font-semibold">Couldn&apos;t load connections</div>
                <div className="mt-1 text-[12px] text-muted-foreground">{loadErr}</div>
              </div>
              <Button variant="default" size="md" onClick={() => void load()}>
                <RefreshCw size={12} strokeWidth={1.8} /> Retry
              </Button>
            </div>
          )}

          {isEmpty && (
            <div className="flex h-full flex-col items-center justify-center gap-3.5 p-10 text-center">
              <div className="flex h-14 w-14 items-center justify-center rounded-xl border bg-panel text-muted-foreground">
                <Database size={28} strokeWidth={1.4} />
              </div>
              <div>
                <div className="text-[14px] font-semibold">No connections yet</div>
                <div className="mx-auto mt-1 max-w-[320px] text-[12px] text-muted-foreground">
                  Add a Cassandra cluster to get started. You&apos;ll need contact
                  points, an auth user, and (optionally) TLS certificates.
                </div>
              </div>
              <Button variant="default" size="md" onClick={handleNew}>
                <Plus size={12} strokeWidth={2.2} /> New connection
              </Button>
            </div>
          )}

          {isLoading && (
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <ConnSkeleton key={i} />
              ))}
            </div>
          )}

          {!isLoading && !loadErr && (connections?.length ?? 0) > 0 && (
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
              {filtered.map((c) => {
                const st = rowState[c.id] ?? { status: "idle" };
                return (
                  <ConnCard
                    key={c.id}
                    c={c}
                    status={st.status}
                    testResult={st.result}
                    isActive={activeId === c.id}
                    onTest={() => void handleTest(c)}
                    onEdit={() => handleEdit(c)}
                    onConnect={() => handleConnect(c)}
                    onDelete={() => setDeleteTarget(c)}
                  />
                );
              })}
              {filter && filtered.length === 0 && (
                <div className="col-span-full py-10 text-center text-[12px] text-muted-foreground">
                  No connections match &quot;{filter}&quot;.
                </div>
              )}
            </div>
          )}
        </div>

        <ConnectionForm
          open={formOpen}
          onOpenChange={setFormOpen}
          existing={editing}
          onSaved={() => void load()}
        />

        <Dialog open={!!deleteTarget} onOpenChange={(v) => !v && setDeleteTarget(null)}>
          <DialogContent
            width={420}
            danger
            title="Delete connection?"
            subtitle={deleteTarget ? `"${deleteTarget.name}" will be removed permanently.` : undefined}
            footer={
              <>
                <div className="flex-1" />
                <Button variant="ghost" size="md" onClick={() => setDeleteTarget(null)}>
                  Cancel
                </Button>
                <Button
                  variant="destructive"
                  size="md"
                  disabled={deleting}
                  onClick={() => void handleDelete()}
                >
                  Delete
                </Button>
              </>
            }
          >
            <div className="text-[12px] text-muted-foreground">
              Saved credentials are deleted along with this connection. The Cassandra
              cluster itself is not modified.
            </div>
          </DialogContent>
        </Dialog>
      </div>
    </AppShell>
  );
}

function activeConnName(list: ConnectionDTO[] | null, id: string | null): string {
  if (!list || !id) return "No connection selected";
  const c = list.find((x) => x.id === id);
  return c ? `${c.name}${c.default_keyspace ? ` · ks: ${c.default_keyspace}` : ""}` : "No connection selected";
}

function isReadOnly(list: ConnectionDTO[] | null, id: string | null): boolean {
  if (!list || !id) return false;
  return list.find((x) => x.id === id)?.read_only ?? false;
}

export default ConnectionsPage;
