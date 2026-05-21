import * as React from "react";
import { useNavigate, useLocation } from "react-router-dom";
import { toast } from "sonner";
import {
  Play,
  Download,
  Search,
  Lock,
  ChevronLeft,
  ChevronRight,
  Plus,
  Check,
  Copy,
  AlertTriangle,
  X,
  Code2,
  Table2,
  Database,
} from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { SchemaBrowser, type LoadState } from "@/components/schema-browser";
import { CqlEditorLive } from "@/components/cql-editor-live";
import { DataGrid, type GridColumn } from "@/components/data-grid";
import { PkIcon } from "@/components/data-grid";
import { TableData } from "@/components/table-data";
import { Highlight } from "@/components/cql-highlight";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Panel } from "@/components/ui/card";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Spinner, Skeleton } from "@/components/primitives";
import { cn } from "@/lib/utils";
import { useActiveConnection } from "@/lib/active-connection";
import { useActiveConnectionDetail } from "@/lib/active-connection-data";
import {
  type Keyspace as LiveKeyspace,
  type TableDetail,
  type Column as LiveColumn,
  listKeyspaces,
  listTables,
  getTable,
  getTableDDL,
} from "@/lib/schema";
import {
  type QueryResult,
  type HistoryEntry,
  runQuery,
  exportQuery,
  listHistory,
  deleteHistory,
} from "@/lib/query";
import { APIError } from "@/lib/api";

// ─── Right pane — live table details ──────────────────────────────────────

interface ActiveTable {
  ks: string;
  name: string;
}

function byPosition(a: LiveColumn, b: LiveColumn) {
  return a.position - b.position;
}

function Prop({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex justify-between gap-2">
      <span className="shrink-0 text-muted-foreground">{k}</span>
      <span className="mono min-w-0 break-all text-right">{v}</span>
    </div>
  );
}

function LiveTableDetails({
  detail,
  ddl,
  loading,
  error,
  onBrowseData,
  onViewData,
}: {
  detail: TableDetail | null;
  ddl: string | null;
  loading: boolean;
  error: string | null;
  onBrowseData?: () => void;
  onViewData?: () => void;
}) {
  const [copied, setCopied] = React.useState(false);
  if (loading) {
    return (
      <div className="p-3">
        {Array.from({ length: 6 }).map((_, i) => (
          <div key={i} className="mb-2 flex justify-between">
            <Skeleton w="55%" h={10} />
            <Skeleton w="25%" h={10} />
          </div>
        ))}
      </div>
    );
  }
  if (error) return <div className="p-3 text-[12px] text-[hsl(0_80%_75%)]">{error}</div>;
  if (!detail) {
    return (
      <div className="p-4 text-[12px] text-muted-foreground">
        Pick a table on the left to see its schema and DDL.
      </div>
    );
  }
  const pkCols = detail.columns.filter((c) => c.kind === "partition_key").sort(byPosition);
  const ckCols = detail.columns.filter((c) => c.kind === "clustering").sort(byPosition);
  const compactClass = detail.compaction?.class?.split(".").pop();

  return (
    <div className="flex h-full flex-col">
      <div className="border-b px-3 py-2.5">
        <div className="text-[10px] uppercase tracking-[0.4px] text-muted-foreground">Table</div>
        <div className="mt-0.5 flex items-center gap-1.5">
          <Table2 size={12} strokeWidth={1.6} className="text-[hsl(var(--info))]" />
          <span className="mono flex-1 truncate text-[12.5px] font-semibold">
            {detail.keyspace}.{detail.name}
          </span>
          {onViewData && (
            <Button size="sm" variant="outline" onClick={onViewData} title="SELECT * … LIMIT 100">
              View 100
            </Button>
          )}
          {onBrowseData && (
            <Button size="sm" variant="outline" onClick={onBrowseData}>
              Edit
            </Button>
          )}
        </div>
        {detail.comment && (
          <div className="mt-1 text-[11px] text-muted-foreground">{detail.comment}</div>
        )}
      </div>

      <div className="flex-1 overflow-y-auto overflow-x-hidden">
        <div className="px-3 pb-1 pt-2 text-[10px] uppercase tracking-[0.4px] text-muted-foreground">
          Columns
        </div>
        <div className="px-2">
          {detail.columns.map((c) => (
            <div key={c.name} className="flex min-w-0 items-center gap-1.5 rounded-sm px-1.5 py-[3px] text-[11px]">
              <PkIcon kind={c.kind === "partition_key" ? "pk" : c.kind === "clustering" ? "ck" : null} />
              {c.kind !== "partition_key" && c.kind !== "clustering" && <span className="w-2.5 shrink-0" />}
              <span className="mono min-w-0 flex-1 truncate" title={c.name}>{c.name}</span>
              {c.kind === "static" && <Badge variant="outline">static</Badge>}
              {c.clustering_order && <Badge variant="outline">{c.clustering_order.toUpperCase()}</Badge>}
              <span className="mono shrink-0 text-[10.5px] text-muted-foreground">{c.type}</span>
            </div>
          ))}
        </div>

        <Separator className="my-2.5" />
        <div className="px-3">
          <div className="mb-1.5 flex items-center justify-between">
            <span className="text-[10px] uppercase tracking-[0.4px] text-muted-foreground">Primary key</span>
            <Badge variant="outline">{pkCols.length > 1 ? "composite" : "simple"}</Badge>
          </div>
          <Panel className="mono break-all px-2 py-1.5 text-[11px] leading-relaxed">
            <span className="text-muted-foreground">PK </span>
            <span className="pk-pill">
              {pkCols.length === 0
                ? "(none)"
                : pkCols.length === 1
                  ? pkCols[0].name
                  : `(${pkCols.map((c) => c.name).join(", ")})`}
            </span>
            {ckCols.length > 0 && (
              <>
                <span className="text-muted-foreground">, CK </span>
                <span className="ck-pill">
                  ({ckCols.map((c) => `${c.name}${c.clustering_order ? ` ${c.clustering_order.toUpperCase()}` : ""}`).join(", ")})
                </span>
              </>
            )}
          </Panel>
        </div>

        <Separator className="my-2.5" />
        <div className="px-3">
          <div className="mb-1.5 text-[10px] uppercase tracking-[0.4px] text-muted-foreground">Properties</div>
          <div className="text-[11px] leading-[1.8]">
            {compactClass && <Prop k="compaction" v={compactClass} />}
            {detail.gc_grace_seconds > 0 && <Prop k="gc_grace" v={`${detail.gc_grace_seconds}s`} />}
            {detail.default_ttl > 0 && <Prop k="default ttl" v={`${detail.default_ttl}s`} />}
            {detail.speculative_retry && <Prop k="spec_retry" v={detail.speculative_retry} />}
            {detail.flags && detail.flags.length > 0 && <Prop k="flags" v={detail.flags.join(", ")} />}
          </div>
        </div>

        {detail.indexes && detail.indexes.length > 0 && (
          <>
            <Separator className="my-2.5" />
            <div className="px-3">
              <div className="mb-1.5 text-[10px] uppercase tracking-[0.4px] text-muted-foreground">Indexes</div>
              {detail.indexes.map((idx) => (
                <div key={idx.name} className="mono mb-1 flex items-center justify-between text-[11px]">
                  <span>{idx.name}</span>
                  <span className="text-muted-foreground">{idx.kind}</span>
                </div>
              ))}
            </div>
          </>
        )}

        <Separator className="my-2.5" />
        <div className="px-3 pb-3">
          <div className="mb-1.5 flex items-center justify-between">
            <span className="text-[10px] uppercase tracking-[0.4px] text-muted-foreground">DDL</span>
            <Button
              size="sm"
              variant="ghost"
              disabled={!ddl}
              onClick={async () => {
                if (!ddl) return;
                await navigator.clipboard.writeText(ddl);
                setCopied(true);
                setTimeout(() => setCopied(false), 1500);
              }}
            >
              {copied ? <Check size={10} strokeWidth={2} /> : <Copy size={10} strokeWidth={1.6} />}
              {copied ? "Copied" : "Copy"}
            </Button>
          </div>
          <Panel className="mono whitespace-pre-wrap break-all px-2 py-1.5 text-[10.5px] leading-[1.5]">
            {ddl ? <Highlight src={ddl} /> : <span className="text-muted-foreground">…</span>}
          </Panel>
        </div>
      </div>
    </div>
  );
}

// ─── Right pane — live query history ──────────────────────────────────────

function HistoryRow({
  h,
  onOpen,
  onDelete,
}: {
  h: HistoryEntry;
  onOpen: () => void;
  onDelete: () => void;
}) {
  const variant = h.success ? "success" : "destructive";
  const icon = h.success ? <Check size={9} strokeWidth={2.5} /> : <X size={9} strokeWidth={2.5} />;
  return (
    <div className="group cursor-pointer border-b border-border/60 px-3 py-1.5" onClick={onOpen}>
      <div className="mb-0.5 flex items-center gap-1.5">
        <Badge variant={variant} icon={icon}>
          {h.statement_kind.toUpperCase()}
        </Badge>
        <span className="mono text-[10.5px] text-muted-foreground">
          {new Date(h.executed_at).toLocaleTimeString()}
        </span>
        <span className="flex-1" />
        <span className="mono text-[10.5px] text-muted-foreground">{h.duration_ms}ms</span>
        {h.row_count > 0 && (
          <span className="mono text-[10.5px] text-muted-foreground">{h.row_count}r</span>
        )}
        <button
          className="opacity-0 group-hover:opacity-100"
          onClick={(e) => {
            e.stopPropagation();
            onDelete();
          }}
          aria-label="Delete history entry"
        >
          <X size={11} className="text-muted-foreground hover:text-foreground" />
        </button>
      </div>
      <div
        className={cn("mono text-[11px] leading-[1.35]", !h.success ? "text-[hsl(0_80%_78%)]" : "text-foreground")}
        style={{ overflow: "hidden", display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical" }}
      >
        {h.cql}
      </div>
      {h.error_message && (
        <div className="mt-1 text-[10.5px] text-[hsl(0_80%_75%)]">{h.error_message}</div>
      )}
    </div>
  );
}

function RightPane({
  detail,
  ddl,
  loading,
  error,
  history,
  onOpenHistory,
  onDeleteHistory,
  onBrowseData,
  onViewData,
}: {
  detail: TableDetail | null;
  ddl: string | null;
  loading: boolean;
  error: string | null;
  history: HistoryEntry[];
  onOpenHistory: (cql: string) => void;
  onDeleteHistory: (id: string) => void;
  onBrowseData?: () => void;
  onViewData?: () => void;
}) {
  const [tab, setTab] = React.useState<string>("details");
  return (
    <div className="flex h-full flex-col bg-background">
      <Tabs value={tab} onValueChange={setTab}>
        <div className="border-b px-2">
          <TabsList>
            <TabsTrigger value="details">Details</TabsTrigger>
            <TabsTrigger value="history">
              History
              <Badge variant="outline">{history.length}</Badge>
            </TabsTrigger>
          </TabsList>
        </div>
      </Tabs>
      <div className="flex-1 overflow-hidden">
        {tab === "details" && (
          <LiveTableDetails detail={detail} ddl={ddl} loading={loading} error={error} onBrowseData={onBrowseData} onViewData={onViewData} />
        )}
        {tab === "history" && (
          <div className="h-full overflow-auto">
            {history.length === 0 ? (
              <div className="p-4 text-[12px] text-muted-foreground">No queries run yet.</div>
            ) : (
              history.map((h) => (
                <HistoryRow
                  key={h.id}
                  h={h}
                  onOpen={() => onOpenHistory(h.cql)}
                  onDelete={() => onDeleteHistory(h.id)}
                />
              ))
            )}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Center pane — query tab ──────────────────────────────────────────────

interface QueryTab {
  id: string;
  title: string;
  kind: "query" | "data";
  // query-kind fields
  cql: string;
  // The CQL actually executed for the current result — equals `cql`, or the
  // selected substring when the user ran only a highlighted query. Used so
  // paging continues the executed statement rather than the full editor text.
  runCql?: string;
  result: QueryResult | null;
  pageStack: string[]; // page_states for previously-seen pages (for back-paging)
  running: boolean;
  error: { code: string; message: string } | null;
  // data-kind fields
  ks?: string;
  tbl?: string;
}

function blankTab(seq: number, cql = ""): QueryTab {
  return {
    id: `tab-${seq}-${Math.random().toString(36).slice(2, 7)}`,
    title: `query #${seq}`,
    kind: "query",
    cql,
    result: null,
    pageStack: [],
    running: false,
    error: null,
  };
}

function makeDataTab(seq: number, ks: string, tbl: string): QueryTab {
  return {
    id: `data-${seq}-${Math.random().toString(36).slice(2, 7)}`,
    title: `${ks}.${tbl}`,
    kind: "data",
    cql: "",
    result: null,
    pageStack: [],
    running: false,
    error: null,
    ks,
    tbl,
  };
}

// Query tabs (and their results) are persisted per-connection so a page refresh
// restores the workspace exactly where the user left it.
const TABS_STORAGE_PREFIX = "cassidy.tabs:";

function loadPersistedTabs(
  connId: string | null,
): { tabs: QueryTab[]; activeTabId: string; seq: number } | null {
  if (!connId) return null;
  try {
    const raw = localStorage.getItem(TABS_STORAGE_PREFIX + connId);
    if (!raw) return null;
    const p = JSON.parse(raw) as { tabs?: QueryTab[]; activeTabId?: string; seq?: number };
    if (!Array.isArray(p.tabs) || p.tabs.length === 0) return null;
    // A query mid-flight when the page closed shouldn't restore as "running".
    const tabs = p.tabs.map((t) => ({ ...t, running: false }));
    const activeTabId =
      p.activeTabId && tabs.some((t) => t.id === p.activeTabId) ? p.activeTabId : tabs[0].id;
    return { tabs, activeTabId, seq: p.seq ?? tabs.length };
  } catch {
    return null;
  }
}

function savePersistedTabs(connId: string, tabs: QueryTab[], activeTabId: string, seq: number) {
  const write = (payloadTabs: QueryTab[]) =>
    localStorage.setItem(
      TABS_STORAGE_PREFIX + connId,
      JSON.stringify({ tabs: payloadTabs, activeTabId, seq }),
    );
  try {
    write(tabs);
  } catch {
    // Result sets too large for the storage quota — keep the tabs + queries but
    // drop the cached results rather than persisting nothing.
    try {
      write(tabs.map((t) => ({ ...t, result: null, pageStack: [] })));
    } catch {
      /* give up silently */
    }
  }
}

// Schema-browser view state (expanded keyspaces/tables, the selected table and
// its loaded detail) is persisted per-connection so a refresh restores the tree
// expansion and the right-pane details exactly as they were.
const SCHEMA_STORAGE_PREFIX = "cassidy.schema:";

interface SchemaViewState {
  expandedKs: string[];
  expandedTbl: Record<string, string[]>;
  tableColumns: Record<string, LiveColumn[]>;
  loadStates: Record<string, LoadState>;
  activeTable: ActiveTable | null;
  tableDetail: TableDetail | null;
  tableDDL: string | null;
}

function loadPersistedSchema(connId: string | null): SchemaViewState | null {
  if (!connId) return null;
  try {
    const raw = localStorage.getItem(SCHEMA_STORAGE_PREFIX + connId);
    if (!raw) return null;
    return JSON.parse(raw) as SchemaViewState;
  } catch {
    return null;
  }
}

function savePersistedSchema(connId: string, s: SchemaViewState) {
  // Only persist successfully-loaded keyspaces — transient loading/error states
  // would otherwise restore as stuck spinners.
  const loadStates: Record<string, LoadState> = {};
  for (const [ks, st] of Object.entries(s.loadStates)) {
    if (st.kind === "loaded") loadStates[ks] = st;
  }
  try {
    localStorage.setItem(
      SCHEMA_STORAGE_PREFIX + connId,
      JSON.stringify({ ...s, loadStates }),
    );
  } catch {
    /* quota exceeded — skip persisting schema view */
  }
}

function resultToGridColumns(r: QueryResult): GridColumn[] {
  return r.columns.map((c) => ({ name: c.name, type: c.type, pk: null, width: 160 }));
}

function resultToGridRows(r: QueryResult): Record<string, unknown>[] {
  return r.rows.map((row) => {
    const obj: Record<string, unknown> = {};
    r.columns.forEach((c, i) => {
      obj[c.name] = row[i];
    });
    return obj;
  });
}

function QueryEditorBar({
  readOnly,
  running,
  consistency,
  onConsistency,
  pageSize,
  onPageSize,
  onRun,
}: {
  readOnly: boolean;
  running: boolean;
  consistency: string;
  onConsistency: (v: string) => void;
  pageSize: string;
  onPageSize: (v: string) => void;
  onRun: () => void;
}) {
  return (
    <div className="flex items-center gap-1.5 border-b bg-background px-2.5 py-1.5">
      <Button variant="default" size="md" disabled={running} onClick={onRun}>
        {running ? <Spinner size={11} /> : <Play size={11} fill="currentColor" />}
        Run
        <span className="kbd ml-1 h-3.5">⌘↵</span>
      </Button>
      <Separator orientation="vertical" className="mx-0.5 h-3.5" />
      <span className="text-[11px] text-muted-foreground">Consistency</span>
      <Select value={consistency} onValueChange={onConsistency}>
        <SelectTrigger className="h-6 text-[11px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {["ONE", "LOCAL_ONE", "QUORUM", "LOCAL_QUORUM", "EACH_QUORUM", "ALL"].map((c) => (
            <SelectItem key={c} value={c}>
              {c}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <span className="text-[11px] text-muted-foreground">Page</span>
      <Select value={pageSize} onValueChange={onPageSize}>
        <SelectTrigger className="h-6 min-w-[70px] text-[11px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {["50", "100", "500", "1000"].map((n) => (
            <SelectItem key={n} value={n}>
              {n}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <div className="flex-1" />
      {readOnly && (
        <Badge variant="warning" icon={<Lock size={9} strokeWidth={2} />}>
          SELECT only
        </Badge>
      )}
      <span className="mono text-[10.5px] text-muted-foreground">UTF-8 · CQL</span>
    </div>
  );
}

function ResultsToolbar({
  result,
  hasPrev,
  hasNext,
  onPrev,
  onNext,
  onExport,
  exporting,
}: {
  result: QueryResult | null;
  hasPrev: boolean;
  hasNext: boolean;
  onPrev: () => void;
  onNext: () => void;
  onExport: () => void;
  exporting: boolean;
}) {
  return (
    <div className="flex items-center gap-2 border-b border-t bg-panel px-2.5 py-1.5 text-[11.5px]">
      {result ? (
        <>
          <Badge variant="success" icon={<Check size={9} strokeWidth={2.5} />}>
            OK
          </Badge>
          <span className="text-muted-foreground">
            {result.row_count.toLocaleString()} {result.row_count === 1 ? "row" : "rows"} · {result.duration_ms}ms
          </span>
        </>
      ) : (
        <span className="text-muted-foreground">No results yet — run a query.</span>
      )}
      <Separator orientation="vertical" className="h-3.5" />
      <Button variant="ghost" size="icon-sm" disabled={!hasPrev} onClick={onPrev} aria-label="Previous page">
        <ChevronLeft size={11} strokeWidth={2} />
      </Button>
      <Button variant="ghost" size="icon-sm" disabled={!hasNext} onClick={onNext} aria-label="Next page">
        <ChevronRight size={11} strokeWidth={2} />
      </Button>
      <div className="flex-1" />
      <Button variant="outline" size="sm" disabled={!result || result.row_count === 0 || exporting} onClick={onExport}>
        {exporting ? <Spinner size={11} /> : <Download size={11} strokeWidth={1.6} />}
        Export
      </Button>
    </div>
  );
}

// ─── No-connection empty state ────────────────────────────────────────────

function NoConnectionEmpty() {
  const navigate = useNavigate();
  return (
    <div className="flex h-full items-center justify-center p-10">
      <div className="max-w-[380px] text-center">
        <div className="mx-auto mb-3.5 flex h-14 w-14 items-center justify-center rounded-xl border bg-panel text-muted-foreground">
          <Database size={26} strokeWidth={1.4} />
        </div>
        <div className="text-[14px] font-semibold">Pick a connection to get started</div>
        <div className="mt-1.5 text-[12px] text-muted-foreground">
          No active connection. Choose one from the Connections page — schema and
          queries are scoped to it. Reference tables as <span className="mono">keyspace.table</span>.
        </div>
        <div className="mt-4 flex justify-center gap-1.5">
          <Button variant="default" size="md" onClick={() => navigate("/connections")}>
            <Database size={12} strokeWidth={1.6} />
            Go to connections
          </Button>
        </div>
      </div>
    </div>
  );
}

// ─── Workspace page ───────────────────────────────────────────────────────

export function WorkspacePage() {
  const activeId = useActiveConnection((s) => s.activeId);
  const { conn } = useActiveConnectionDetail();
  const readOnly = conn?.read_only ?? false;
  const navigate = useNavigate();
  const location = useLocation();

  // Schema browser state.
  const [keyspaces, setKeyspaces] = React.useState<LiveKeyspace[] | null>(null);
  const [keyspacesError, setKeyspacesError] = React.useState<string | null>(null);
  const [loadStates, setLoadStates] = React.useState<Record<string, LoadState>>({});
  const [expandedKs, setExpandedKs] = React.useState<string[]>([]);
  const [expandedTbl, setExpandedTbl] = React.useState<Record<string, string[]>>({});
  const [tableColumns, setTableColumns] = React.useState<Record<string, LiveColumn[]>>({});

  // Right pane (table details).
  const [activeTable, setActiveTable] = React.useState<ActiveTable | null>(null);
  const [tableDetail, setTableDetail] = React.useState<TableDetail | null>(null);
  const [tableDDL, setTableDDL] = React.useState<string | null>(null);
  const [tableLoading, setTableLoading] = React.useState(false);
  const [tableError, setTableError] = React.useState<string | null>(null);

  // Query tabs — restored from localStorage for the active connection so a
  // refresh preserves open tabs and their results.
  const seqRef = React.useRef(1);
  const loadedFor = React.useRef<string | null>(null);
  const initRef = React.useRef<{ tabs: QueryTab[]; activeTabId: string } | null>(null);
  if (initRef.current === null) {
    const p = loadPersistedTabs(activeId);
    loadedFor.current = activeId;
    if (p) {
      seqRef.current = p.seq;
      initRef.current = { tabs: p.tabs, activeTabId: p.activeTabId };
    } else {
      const t = blankTab(1);
      initRef.current = { tabs: [t], activeTabId: t.id };
    }
  }
  const [tabs, setTabs] = React.useState<QueryTab[]>(initRef.current.tabs);
  const [activeTabId, setActiveTabId] = React.useState<string>(initRef.current.activeTabId);
  const activeTab = tabs.find((t) => t.id === activeTabId) ?? tabs[0];

  // Current editor selection (kept fresh by CqlEditorLive) so Run / ⌘↵ can
  // execute only the highlighted query when there is a selection.
  const editorSelectionRef = React.useRef("");

  // When the active connection changes, swap in that connection's persisted
  // tabs (or a fresh blank tab). Runs after mount (init already handled first).
  React.useEffect(() => {
    if (!activeId || loadedFor.current === activeId) return;
    const p = loadPersistedTabs(activeId);
    loadedFor.current = activeId;
    if (p) {
      seqRef.current = p.seq;
      setTabs(p.tabs);
      setActiveTabId(p.activeTabId);
    } else {
      seqRef.current += 1;
      const t = blankTab(seqRef.current);
      setTabs([t]);
      setActiveTabId(t.id);
    }
  }, [activeId]);

  // Persist tabs + results whenever they change, keyed by the connection the
  // tabs belong to (loadedFor) — not the live activeId, to avoid a switch race.
  React.useEffect(() => {
    if (loadedFor.current) {
      savePersistedTabs(loadedFor.current, tabs, activeTabId, seqRef.current);
    }
  }, [tabs, activeTabId]);

  // Toolbar state (workspace-level).
  const [consistency, setConsistency] = React.useState("LOCAL_QUORUM");
  const [pageSize, setPageSize] = React.useState("100");
  const [exporting, setExporting] = React.useState(false);

  // History.
  const [history, setHistory] = React.useState<HistoryEntry[]>([]);

  const patchTab = React.useCallback((id: string, patch: Partial<QueryTab>) => {
    setTabs((ts) => ts.map((t) => (t.id === id ? { ...t, ...patch } : t)));
  }, []);

  const refreshHistory = React.useCallback(async () => {
    if (!activeId) return;
    try {
      setHistory(await listHistory({ connection_id: activeId, limit: 50 }));
    } catch {
      /* ignore */
    }
  }, [activeId]);

  // Tracks which connection the schema view state currently belongs to, so the
  // persist effect writes under the right key during a connection switch.
  const schemaLoadedFor = React.useRef<string | null>(null);

  // Load keyspaces + history when the active connection changes. Restores the
  // persisted tree expansion / selected table for the connection rather than
  // resetting to empty, so a refresh keeps the schema browser where it was.
  const loadKeyspaces = React.useCallback(async () => {
    if (!activeId) return;
    schemaLoadedFor.current = activeId;
    const saved = loadPersistedSchema(activeId);
    setKeyspaces(null);
    setKeyspacesError(null);
    setLoadStates(saved?.loadStates ?? {});
    setExpandedKs(saved?.expandedKs ?? []);
    setExpandedTbl(saved?.expandedTbl ?? {});
    setTableColumns(saved?.tableColumns ?? {});
    setActiveTable(saved?.activeTable ?? null);
    setTableDetail(saved?.tableDetail ?? null);
    setTableDDL(saved?.tableDDL ?? null);
    try {
      setKeyspaces((await listKeyspaces(activeId)) ?? []);
    } catch (e) {
      setKeyspacesError(e instanceof APIError ? e.message : "Failed to load keyspaces");
    }
  }, [activeId]);

  React.useEffect(() => {
    void loadKeyspaces();
    void refreshHistory();
  }, [loadKeyspaces, refreshHistory]);

  // Persist schema-browser view state (keyed by the connection it belongs to,
  // not the live activeId, to avoid a switch race) so refresh restores it.
  React.useEffect(() => {
    if (schemaLoadedFor.current) {
      savePersistedSchema(schemaLoadedFor.current, {
        expandedKs,
        expandedTbl,
        tableColumns,
        loadStates,
        activeTable,
        tableDetail,
        tableDDL,
      });
    }
  }, [expandedKs, expandedTbl, tableColumns, loadStates, activeTable, tableDetail, tableDDL]);

  const ensureTables = React.useCallback(
    async (ks: string) => {
      if (!activeId) return;
      const cur = loadStates[ks];
      if (cur?.kind === "loaded" || cur?.kind === "loading") return;
      setLoadStates((s) => ({ ...s, [ks]: { kind: "loading" } }));
      try {
        const tables = await listTables(activeId, ks);
        setLoadStates((s) => ({ ...s, [ks]: { kind: "loaded", tables: tables ?? [] } }));
      } catch (e) {
        setLoadStates((s) => ({
          ...s,
          [ks]: { kind: "error", message: e instanceof APIError ? e.message : "Failed to load tables" },
        }));
      }
    },
    [activeId, loadStates],
  );

  const onToggleKs = (ks: string) => {
    if (expandedKs.includes(ks)) setExpandedKs((s) => s.filter((x) => x !== ks));
    else {
      setExpandedKs((s) => [...s, ks]);
      void ensureTables(ks);
    }
  };

  const onToggleTbl = (ks: string, table: string) => {
    setExpandedTbl((s) => {
      const cur = s[ks] ?? [];
      const next = cur.includes(table) ? cur.filter((x) => x !== table) : [...cur, table];
      return { ...s, [ks]: next };
    });
  };

  const onSelectTable = React.useCallback(
    async (ks: string, table: string) => {
      if (!activeId) return;
      setActiveTable({ ks, name: table });
      setTableLoading(true);
      setTableError(null);
      setTableDetail(null);
      setTableDDL(null);
      try {
        const [detail, ddlResp] = await Promise.all([
          getTable(activeId, ks, table),
          getTableDDL(activeId, ks, table),
        ]);
        setTableDetail(detail);
        setTableDDL(ddlResp.ddl);
        setTableColumns((s) => ({ ...s, [`${ks}::${table}`]: detail.columns }));
      } catch (e) {
        setTableError(e instanceof APIError ? e.message : "Failed to load table");
      } finally {
        setTableLoading(false);
      }
    },
    [activeId],
  );

  // ── Tab actions ──
  const newTab = (cql = "") => {
    seqRef.current += 1;
    const t = blankTab(seqRef.current, cql);
    setTabs((ts) => [...ts, t]);
    setActiveTabId(t.id);
  };

  // Open-in-workspace handoff from the /history page: if navigation carried a
  // CQL payload, open it in a fresh query tab once, then clear the router state
  // so a later refresh doesn't reopen it.
  const handledOpenKey = React.useRef<string | null>(null);
  React.useEffect(() => {
    const cql = (location.state as { openCql?: string } | null)?.openCql;
    if (!cql || handledOpenKey.current === location.key) return;
    handledOpenKey.current = location.key;
    newTab(cql);
    navigate(location.pathname, { replace: true, state: {} });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [location.key]);

  const closeTab = (id: string) => {
    setTabs((ts) => {
      const next = ts.filter((t) => t.id !== id);
      if (next.length === 0) {
        seqRef.current += 1;
        const blank = blankTab(seqRef.current);
        setActiveTabId(blank.id);
        return [blank];
      }
      if (id === activeTabId) setActiveTabId(next[next.length - 1].id);
      return next;
    });
  };

  // Open (or focus) a data tab for a table.
  const openDataTab = (ks: string, tbl: string) => {
    const existing = tabs.find((t) => t.kind === "data" && t.ks === ks && t.tbl === tbl);
    if (existing) {
      setActiveTabId(existing.id);
      return;
    }
    seqRef.current += 1;
    const t = makeDataTab(seqRef.current, ks, tbl);
    setTabs((ts) => [...ts, t]);
    setActiveTabId(t.id);
  };

  // ── Run / paging ──
  const execute = React.useCallback(
    async (
      tab: QueryTab,
      pageState: string | undefined,
      pushPrev: string | undefined,
      overrideCql?: string,
    ) => {
      const cql = (overrideCql ?? tab.cql).trim();
      if (!activeId || !cql) return;
      patchTab(tab.id, { running: true, error: null });
      try {
        const res = await runQuery(activeId, {
          cql,
          consistency,
          page_size: Number(pageSize),
          page_state: pageState,
        });
        setTabs((ts) =>
          ts.map((t) => {
            if (t.id !== tab.id) return t;
            const stack = pushPrev !== undefined ? [...t.pageStack, pushPrev] : pushPrev === undefined && pageState === undefined ? [] : t.pageStack;
            return { ...t, result: res, running: false, error: null, pageStack: stack, runCql: cql };
          }),
        );
        if (res.warnings && res.warnings.length > 0) {
          res.warnings.forEach((w) => toast.warning(w));
        }
        // DDL may have changed the schema — refresh the tree.
        if (["create", "alter", "drop", "truncate"].includes(res.statement_kind)) {
          void loadKeyspaces();
          toast.success(`${res.statement_kind.toUpperCase()} applied`);
        }
        void refreshHistory();
      } catch (e) {
        const code = e instanceof APIError ? e.code : "error";
        const message = e instanceof APIError ? e.message : "Query failed";
        patchTab(tab.id, { running: false, error: { code, message } });
        toast.error(message);
        void refreshHistory();
      }
    },
    [activeId, consistency, pageSize, patchTab, loadKeyspaces, refreshHistory],
  );

  const runActive = () => {
    if (!activeTab || activeTab.kind !== "query") return;
    // If the user has highlighted text in the editor, run only that selection
    // (pgAdmin/DataGrip-style); otherwise run the whole tab.
    const selected = editorSelectionRef.current.trim();
    const override = selected || undefined;
    // Fresh run resets the page stack.
    setTabs((ts) => ts.map((t) => (t.id === activeTab.id ? { ...t, pageStack: [] } : t)));
    void execute({ ...activeTab, pageStack: [] }, undefined, undefined, override);
  };

  // pgAdmin-style "view first N rows": open a query tab pre-filled with
  // SELECT * FROM ks.tbl LIMIT N and run it immediately.
  const viewRows = (ks: string, tbl: string, limit = 100) => {
    const cql = `SELECT * FROM ${ks}.${tbl} LIMIT ${limit};`;
    seqRef.current += 1;
    const t = blankTab(seqRef.current, cql);
    setTabs((ts) => [...ts, t]);
    setActiveTabId(t.id);
    void execute(t, undefined, undefined);
  };

  const nextPage = () => {
    if (!activeTab?.result?.next_page_state) return;
    // Current page's "entry" state is the top of the stack basis; push the
    // state that produced the CURRENT page so Back can return to it.
    const currentEntry = activeTab.pageStack.length > 0 ? activeTab.pageStack[activeTab.pageStack.length - 1] : "";
    void execute(activeTab, activeTab.result.next_page_state, currentEntry, activeTab.runCql);
  };

  const prevPage = () => {
    if (!activeTab || activeTab.pageStack.length === 0) return;
    const stack = [...activeTab.pageStack];
    const prevState = stack.pop()!; // the page_state that produced the previous page
    patchTab(activeTab.id, { pageStack: stack });
    void execute({ ...activeTab, pageStack: stack }, prevState || undefined, undefined, activeTab.runCql);
  };

  const handleExport = async () => {
    if (!activeId || !activeTab?.cql.trim()) return;
    setExporting(true);
    try {
      const resp = await exportQuery(activeId, { cql: activeTab.cql, consistency }, "csv");
      if (!resp.ok) {
        toast.error("Export failed");
        return;
      }
      const blob = await resp.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "cassidy-export.csv";
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast.error("Export failed");
    } finally {
      setExporting(false);
    }
  };

  const onDeleteHistory = async (id: string) => {
    try {
      await deleteHistory(id);
      setHistory((h) => h.filter((e) => e.id !== id));
    } catch {
      /* ignore */
    }
  };

  if (!activeId) {
    return (
      <AppShell>
        <NoConnectionEmpty />
      </AppShell>
    );
  }

  return (
    <AppShell>
      <div className="flex h-full">
        <div className="w-[232px] shrink-0 border-r">
          <SchemaBrowser
            keyspaces={keyspaces}
            keyspacesError={keyspacesError}
            loadStates={loadStates}
            expandedKs={expandedKs}
            expandedTbl={expandedTbl}
            tableColumns={tableColumns}
            activeTable={activeTable}
            onToggleKs={onToggleKs}
            onToggleTbl={onToggleTbl}
            onSelectTable={onSelectTable}
            onRefresh={loadKeyspaces}
          />
        </div>
        <div className="flex min-w-0 flex-1 flex-col">
          {/* Tab strip */}
          <div className="flex h-8 items-stretch border-b bg-background">
            {tabs.map((t) => (
              <div
                key={t.id}
                onClick={() => setActiveTabId(t.id)}
                className={cn(
                  "relative inline-flex max-w-[200px] cursor-pointer select-none items-center gap-1.5 border-r px-2.5 text-[12px] text-muted-foreground hover:bg-panel-2 hover:text-foreground",
                  t.id === activeTabId && "bg-panel text-foreground",
                  t.id === activeTabId &&
                    "after:absolute after:left-0 after:right-0 after:top-0 after:h-0.5 after:bg-foreground",
                )}
              >
                {t.kind === "data" ? (
                  <Table2 size={11} strokeWidth={1.6} className="text-[hsl(var(--warning))]" />
                ) : (
                  <Code2 size={11} strokeWidth={1.6} className="text-[hsl(var(--info))]" />
                )}
                <span className="mono truncate text-[11.5px]">{t.title}</span>
                <button
                  className="rounded-sm p-0.5 opacity-50 hover:bg-secondary hover:opacity-100"
                  onClick={(e) => {
                    e.stopPropagation();
                    closeTab(t.id);
                  }}
                  aria-label="Close tab"
                >
                  <X size={9} strokeWidth={2} />
                </button>
              </div>
            ))}
            <button
              type="button"
              onClick={() => newTab()}
              className="inline-flex w-7 items-center justify-center border-r text-muted-foreground hover:bg-panel-2 hover:text-foreground"
              aria-label="New query tab"
            >
              <Plus size={11} strokeWidth={2} />
            </button>
          </div>

          {/* Active tab body */}
          {activeTab?.kind === "data" ? (
            <TableData
              connId={activeId}
              ks={activeTab.ks!}
              tbl={activeTab.tbl!}
              readOnly={readOnly}
            />
          ) : (
            <div className="flex min-h-0 flex-1 flex-col">
              <QueryEditorBar
                readOnly={readOnly}
                running={activeTab?.running ?? false}
                consistency={consistency}
                onConsistency={setConsistency}
                pageSize={pageSize}
                onPageSize={setPageSize}
                onRun={runActive}
              />
              <div className="border-b" style={{ height: "38%" }}>
                <CqlEditorLive
                  value={activeTab?.cql ?? ""}
                  onChange={(v) => activeTab && patchTab(activeTab.id, { cql: v })}
                  onRun={runActive}
                  onSelectionChange={(s) => (editorSelectionRef.current = s)}
                  readOnly={false}
                  connId={activeId}
                  keyspace={activeTable?.ks ?? conn?.default_keyspace ?? ""}
                />
              </div>
              <ResultsToolbar
                result={activeTab?.result ?? null}
                hasPrev={(activeTab?.pageStack.length ?? 0) > 0}
                hasNext={!!activeTab?.result?.next_page_state}
                onPrev={prevPage}
                onNext={nextPage}
                onExport={handleExport}
                exporting={exporting}
              />
              <div className="min-h-0 flex-1">
                {activeTab?.error ? (
                  <div className="flex h-full items-center justify-center p-10">
                    <div className="max-w-[420px] text-center">
                      <div className="mx-auto mb-3.5 flex h-12 w-12 items-center justify-center rounded-xl border border-[hsl(var(--destructive)/0.35)] bg-[hsl(var(--destructive)/0.12)] text-[hsl(0_80%_75%)]">
                        <AlertTriangle size={22} strokeWidth={1.8} />
                      </div>
                      <div className="text-[14px] font-semibold">Query failed</div>
                      <div className="mt-1.5 text-[12px] text-muted-foreground">
                        <code className="mono">{activeTab.error.code}</code>: {activeTab.error.message}
                      </div>
                    </div>
                  </div>
                ) : activeTab?.result ? (
                  activeTab.result.columns.length > 0 ? (
                    <DataGrid
                      columns={resultToGridColumns(activeTab.result)}
                      rows={resultToGridRows(activeTab.result)}
                    />
                  ) : (
                    <div className="flex h-full items-center justify-center text-[12px] text-muted-foreground">
                      Statement applied — no rows returned.
                    </div>
                  )
                ) : (
                  <div className="flex h-full items-center justify-center p-10 text-center">
                    <div className="max-w-[360px]">
                      <div className="text-[13px] font-semibold">Write a query</div>
                      <div className="mt-1.5 text-[12px] text-muted-foreground">
                        Reference tables as <span className="mono">keyspace.table</span>. Press{" "}
                        <span className="kbd">⌘</span> <span className="kbd">↵</span> to run.
                      </div>
                    </div>
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
        <div className="w-[290px] shrink-0 border-l">
          <RightPane
            detail={tableDetail}
            ddl={tableDDL}
            loading={tableLoading}
            error={tableError}
            history={history}
            onOpenHistory={(cql) => newTab(cql)}
            onDeleteHistory={onDeleteHistory}
            onBrowseData={() => activeTable && openDataTab(activeTable.ks, activeTable.name)}
            onViewData={() => activeTable && viewRows(activeTable.ks, activeTable.name)}
          />
        </div>
      </div>
    </AppShell>
  );
}

export default WorkspacePage;
