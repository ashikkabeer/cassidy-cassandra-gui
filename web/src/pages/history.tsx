import * as React from "react";
import { useNavigate } from "react-router-dom";
import { Search, Check, X, Copy, ExternalLink, Trash2 } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/primitives";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { listHistory, deleteHistory, type HistoryEntry } from "@/lib/query";
import { listConnections } from "@/lib/connections";

const PAGE = 100;
type StatusFilter = "all" | "ok" | "err";

function successParam(s: StatusFilter): boolean | undefined {
  if (s === "ok") return true;
  if (s === "err") return false;
  return undefined;
}

function HistoryListRow({
  h,
  connName,
  onOpen,
  onCopy,
  onDelete,
}: {
  h: HistoryEntry;
  connName: string;
  onOpen: () => void;
  onCopy: () => void;
  onDelete: () => void;
}) {
  const variant = h.success ? "success" : "destructive";
  const icon = h.success ? <Check size={9} strokeWidth={2.5} /> : <X size={9} strokeWidth={2.5} />;
  return (
    <div className="group border-b border-border/60 px-4 py-2.5 hover:bg-panel/40">
      <div className="mb-1 flex items-center gap-2">
        <Badge variant={variant} icon={icon}>
          {h.statement_kind.toUpperCase()}
        </Badge>
        <span className="mono truncate text-[11px] text-muted-foreground" title={connName}>
          {connName}
        </span>
        {h.keyspace && (
          <span className="mono text-[11px] text-muted-foreground">· {h.keyspace}</span>
        )}
        <span className="flex-1" />
        <span className="mono text-[10.5px] text-muted-foreground">
          {new Date(h.executed_at).toLocaleString()}
        </span>
        <span className="mono text-[10.5px] text-muted-foreground">{h.duration_ms}ms</span>
        {h.row_count > 0 && (
          <span className="mono text-[10.5px] text-muted-foreground">{h.row_count}r</span>
        )}
        <div className="flex items-center gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
          <Button variant="ghost" size="icon-sm" aria-label="Open in workspace" onClick={onOpen}>
            <ExternalLink size={12} strokeWidth={1.6} />
          </Button>
          <Button variant="ghost" size="icon-sm" aria-label="Copy CQL" onClick={onCopy}>
            <Copy size={12} strokeWidth={1.6} />
          </Button>
          <Button variant="ghost" size="icon-sm" aria-label="Delete" onClick={onDelete}>
            <Trash2 size={12} strokeWidth={1.6} />
          </Button>
        </div>
      </div>
      <div
        className={cn(
          "mono cursor-pointer text-[11.5px] leading-[1.4]",
          !h.success ? "text-[hsl(0_80%_78%)]" : "text-foreground",
        )}
        style={{ overflow: "hidden", display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical" }}
        onClick={onOpen}
        title="Open in workspace"
      >
        {h.cql}
      </div>
      {h.error_message && (
        <div className="mt-1 text-[10.5px] text-[hsl(0_80%_75%)]">{h.error_message}</div>
      )}
    </div>
  );
}

export function HistoryPage() {
  const navigate = useNavigate();
  const [entries, setEntries] = React.useState<HistoryEntry[] | null>(null);
  const [connMap, setConnMap] = React.useState<Record<string, string>>({});
  const [search, setSearch] = React.useState("");
  const [status, setStatus] = React.useState<StatusFilter>("all");
  const [hasMore, setHasMore] = React.useState(false);
  const [loadingMore, setLoadingMore] = React.useState(false);

  // Map connection_id -> display name so rows show a name instead of a UUID.
  React.useEffect(() => {
    void (async () => {
      try {
        const conns = await listConnections();
        setConnMap(Object.fromEntries((conns ?? []).map((c) => [c.id, c.name])));
      } catch {
        /* names are best-effort */
      }
    })();
  }, []);

  // Fetch a fresh page whenever the status filter changes.
  React.useEffect(() => {
    setEntries(null);
    void (async () => {
      try {
        const list = (await listHistory({ limit: PAGE, success: successParam(status) })) ?? [];
        setEntries(list);
        setHasMore(list.length === PAGE);
      } catch {
        setEntries([]);
        setHasMore(false);
      }
    })();
  }, [status]);

  const loadMore = async () => {
    if (!entries || entries.length === 0) return;
    setLoadingMore(true);
    try {
      const before = entries[entries.length - 1].executed_at;
      const more =
        (await listHistory({ limit: PAGE, success: successParam(status), before })) ?? [];
      setEntries((cur) => [...(cur ?? []), ...more]);
      setHasMore(more.length === PAGE);
    } catch {
      toast.error("Failed to load more");
    } finally {
      setLoadingMore(false);
    }
  };

  const openInWorkspace = (cql: string) => navigate("/workspace", { state: { openCql: cql } });

  const copyCql = async (cql: string) => {
    await navigator.clipboard.writeText(cql);
    toast.success("CQL copied");
  };

  const removeEntry = async (id: string) => {
    setEntries((cur) => (cur ?? []).filter((e) => e.id !== id));
    try {
      await deleteHistory(id);
    } catch {
      toast.error("Failed to delete");
    }
  };

  const connNameOf = (id?: string) =>
    id ? (connMap[id] ?? `${id.slice(0, 8)}…`) : "—";

  const filtered = React.useMemo(() => {
    if (!entries) return null;
    const q = search.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter(
      (e) =>
        e.cql.toLowerCase().includes(q) ||
        (e.keyspace ?? "").toLowerCase().includes(q) ||
        connNameOf(e.connection_id).toLowerCase().includes(q),
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entries, search, connMap]);

  return (
    <AppShell>
      <div className="flex h-full flex-col">
        {/* Header + toolbar */}
        <div className="flex items-center gap-2 border-b px-5 py-3.5">
          <div className="mr-2">
            <div className="text-[16px] font-semibold tracking-[-0.2px]">Query history</div>
            <div className="mt-0.5 text-[11.5px] text-muted-foreground">
              Every query you've run, across all connections
            </div>
          </div>
          <span className="flex-1" />
          <Input
            wrapperClassName="w-[260px] h-[30px] bg-panel"
            placeholder="Filter by CQL, keyspace, connection…"
            icon={<Search size={12} strokeWidth={1.8} />}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
          <Select value={status} onValueChange={(v) => setStatus(v as StatusFilter)}>
            <SelectTrigger className="h-[30px] w-[130px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All statuses</SelectItem>
              <SelectItem value="ok">OK only</SelectItem>
              <SelectItem value="err">Errors only</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* List */}
        <div className="min-h-0 flex-1 overflow-auto">
          {filtered === null ? (
            <div className="p-4">
              {Array.from({ length: 8 }).map((_, i) => (
                <div key={i} className="mb-3">
                  <Skeleton w="20%" h={12} />
                  <div className="mt-1.5">
                    <Skeleton w="70%" h={10} />
                  </div>
                </div>
              ))}
            </div>
          ) : filtered.length === 0 ? (
            <div className="p-6 text-[12px] text-muted-foreground">
              {entries && entries.length > 0
                ? "No queries match your filter."
                : "No queries yet — run a query in the workspace and it'll show up here."}
            </div>
          ) : (
            <>
              {filtered.map((h) => (
                <HistoryListRow
                  key={h.id}
                  h={h}
                  connName={connNameOf(h.connection_id)}
                  onOpen={() => openInWorkspace(h.cql)}
                  onCopy={() => void copyCql(h.cql)}
                  onDelete={() => void removeEntry(h.id)}
                />
              ))}
              {hasMore && !search && (
                <div className="flex justify-center p-4">
                  <Button variant="outline" size="sm" onClick={() => void loadMore()} disabled={loadingMore}>
                    {loadingMore ? "Loading…" : "Load more"}
                  </Button>
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </AppShell>
  );
}
