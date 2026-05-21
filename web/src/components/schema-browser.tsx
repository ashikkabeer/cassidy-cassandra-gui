import * as React from "react";
import {
  ChevronDown,
  ChevronRight,
  Folder,
  Table2,
  KeyRound,
  Zap,
  Search,
  RefreshCw,
  Plus,
  Copy,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuSeparator,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import { Skeleton, Spinner } from "@/components/primitives";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import {
  type Keyspace as LiveKeyspace,
  type TableSummary,
  type Column as LiveColumn,
} from "@/lib/schema";

// LoadState tracks per-keyspace table-list loading so the tree can render a
// spinner / error inline while the rest stays interactive.
export type LoadState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "loaded"; tables: TableSummary[] }
  | { kind: "error"; message: string };

export interface SchemaBrowserProps {
  keyspaces: LiveKeyspace[] | null; // null = still loading the top-level
  keyspacesError?: string | null;
  loadStates: Record<string, LoadState>;
  expandedKs: string[];
  expandedTbl: Record<string, string[]>;
  /** Columns we already have for tables that have been opened — lets the tree
   *  show columns inline. Keyed by `${ks}::${tbl}`. */
  tableColumns?: Record<string, LiveColumn[]>;
  activeTable?: { ks: string; name: string } | null;
  onToggleKs: (ks: string) => void;
  onToggleTbl: (ks: string, table: string) => void;
  onSelectTable: (ks: string, table: string) => void;
  onViewRows: (ks: string, table: string) => void;
  onRefresh: () => void;
}

interface RowDef {
  kind: "ks" | "tbl" | "col" | "h" | "idx";
  label: string;
  depth: number;
  // Set on table ("tbl") rows so the right-click menu knows the table identity.
  ks?: string;
  tableName?: string;
  sub?: string | null;
  icon?: React.ReactNode;
  expanded?: boolean;
  expandable?: boolean;
  active?: boolean;
  color?: string;
  loading?: boolean;
  errorMsg?: string;
  onClick?: () => void;
}

function TreeRow({ row }: { row: RowDef }) {
  if (row.kind === "h") {
    return (
      <div
        style={{ paddingLeft: 4 + row.depth * 12 }}
        className="mt-1 py-0.5 text-[9.5px] uppercase tracking-[0.5px] text-muted-foreground"
      >
        {row.label}
      </div>
    );
  }
  return (
    <div
      onClick={row.onClick}
      style={{ paddingLeft: 4 + row.depth * 12 }}
      className={cn(
        "flex h-[22px] cursor-pointer items-center gap-1 pr-1.5 text-[12px]",
        row.active && "bg-accent",
      )}
    >
      {row.expandable ? (
        row.expanded ? (
          <ChevronDown
            size={10}
            strokeWidth={2}
            className="shrink-0 text-muted-foreground"
          />
        ) : (
          <ChevronRight
            size={10}
            strokeWidth={2}
            className="shrink-0 text-muted-foreground"
          />
        )
      ) : (
        <span className="w-2.5 shrink-0" />
      )}
      {row.icon && (
        <span className="flex shrink-0" style={{ color: row.color }}>
          {row.icon}
        </span>
      )}
      <span className="mono flex-1 truncate text-[11.5px]">{row.label}</span>
      {row.loading && <Spinner size={10} className="text-muted-foreground" />}
      {row.errorMsg && (
        <span title={row.errorMsg} className="text-[10px] text-[hsl(0_80%_72%)]">
          err
        </span>
      )}
      {!row.loading && row.sub && (
        <span className="mono text-[10px] text-muted-foreground">{row.sub}</span>
      )}
    </div>
  );
}

export function SchemaBrowser({
  keyspaces,
  keyspacesError,
  loadStates,
  expandedKs,
  expandedTbl,
  tableColumns = {},
  activeTable,
  onToggleKs,
  onToggleTbl,
  onSelectTable,
  onViewRows,
  onRefresh,
}: SchemaBrowserProps) {
  const [filter, setFilter] = React.useState("");

  const filteredKs = React.useMemo(() => {
    if (!keyspaces) return [];
    if (!filter) return keyspaces;
    const f = filter.toLowerCase();
    return keyspaces.filter((k) => k.name.toLowerCase().includes(f));
  }, [keyspaces, filter]);

  const rows: RowDef[] = [];
  for (const ks of filteredKs) {
    const state = loadStates[ks.name] ?? ({ kind: "idle" } as LoadState);
    const expanded = expandedKs.includes(ks.name);
    let sub: string | null = null;
    if (state.kind === "loaded") sub = `${state.tables.length} tables`;
    rows.push({
      kind: "ks",
      label: ks.name,
      depth: 0,
      icon: <Folder size={11} strokeWidth={1.6} />,
      sub,
      expanded,
      expandable: true,
      color: ks.system ? "hsl(var(--muted-foreground))" : "hsl(var(--warning))",
      loading: state.kind === "loading",
      errorMsg: state.kind === "error" ? state.message : undefined,
      onClick: () => onToggleKs(ks.name),
    });
    if (!expanded) continue;
    if (state.kind !== "loaded") continue;
    const tableFilter = filter.toLowerCase();
    const matched = filter
      ? state.tables.filter(
          (t) =>
            t.name.toLowerCase().includes(tableFilter) ||
            ks.name.toLowerCase().includes(tableFilter),
        )
      : state.tables;
    for (const tbl of matched) {
      const tblKey = `${ks.name}::${tbl.name}`;
      const isExp = expandedTbl[ks.name]?.includes(tbl.name) ?? false;
      const isActive = activeTable?.ks === ks.name && activeTable?.name === tbl.name;
      rows.push({
        kind: "tbl",
        label: tbl.name,
        depth: 1,
        ks: ks.name,
        tableName: tbl.name,
        icon: <Table2 size={10} strokeWidth={1.5} />,
        expanded: isExp,
        expandable: true,
        active: isActive,
        color: "hsl(var(--info))",
        onClick: () => {
          onToggleTbl(ks.name, tbl.name);
          onSelectTable(ks.name, tbl.name);
        },
      });
      if (!isExp) continue;
      const cols = tableColumns[tblKey];
      if (!cols) {
        rows.push({
          kind: "col",
          label: "loading…",
          depth: 2,
          color: "hsl(var(--muted-foreground))",
          loading: true,
        });
        continue;
      }
      for (const c of cols) {
        rows.push({
          kind: "col",
          label: c.name,
          depth: 2,
          icon:
            c.kind === "partition_key" ? (
              <KeyRound size={10} strokeWidth={1.8} />
            ) : c.kind === "clustering" ? (
              <Zap size={10} strokeWidth={1.6} />
            ) : (
              <span className="inline-block h-1 w-1 rounded-full bg-current" />
            ),
          sub: c.type,
          color:
            c.kind === "partition_key"
              ? "hsl(38 92% 65%)"
              : c.kind === "clustering"
                ? "hsl(190 80% 65%)"
                : "hsl(var(--muted-foreground))",
        });
      }
    }
  }

  const totalKs = keyspaces?.length ?? 0;
  const totalTables = Object.values(loadStates).reduce(
    (n, s) => n + (s.kind === "loaded" ? s.tables.length : 0),
    0,
  );

  return (
    // Table rows have a custom right-click menu (below); suppress the browser's
    // native menu everywhere else in the tree (keyspaces, columns, empty space).
    <div
      className="flex h-full flex-col bg-background"
      onContextMenu={(e) => e.preventDefault()}
    >
      <div className="flex items-center gap-1.5 border-b px-2.5 py-2">
        <Input
          wrapperClassName="h-6 bg-panel flex-1 text-[11px]"
          placeholder="Filter schema…"
          icon={<Search size={11} strokeWidth={1.8} />}
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <Button variant="ghost" size="icon-sm" aria-label="Refresh" onClick={onRefresh}>
          <RefreshCw size={12} strokeWidth={1.6} />
        </Button>
      </div>
      <div className="flex-1 overflow-auto py-1">
        {keyspaces === null && !keyspacesError && (
          <div className="px-3 py-2">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className="flex h-[22px] items-center gap-1.5">
                <Skeleton w={10} h={10} />
                <Skeleton w={`${40 + ((i * 13) % 40)}%`} h={10} />
              </div>
            ))}
          </div>
        )}
        {keyspacesError && (
          <div className="px-3 py-2 text-[11.5px] text-[hsl(0_80%_75%)]">
            {keyspacesError}
          </div>
        )}
        {rows.map((r, i) =>
          r.kind === "tbl" && r.ks && r.tableName ? (
            <ContextMenu key={i}>
              <ContextMenuTrigger asChild>
                <div>
                  <TreeRow row={r} />
                </div>
              </ContextMenuTrigger>
              <ContextMenuContent>
                <ContextMenuItem onSelect={() => onViewRows(r.ks!, r.tableName!)}>
                  <Table2 size={12} strokeWidth={1.6} />
                  View first 100 rows
                </ContextMenuItem>
                <ContextMenuSeparator />
                <ContextMenuItem
                  onSelect={() => {
                    void navigator.clipboard.writeText(`${r.ks}.${r.tableName}`);
                    toast.success("Copied name");
                  }}
                >
                  <Copy size={12} strokeWidth={1.6} />
                  Copy name
                </ContextMenuItem>
              </ContextMenuContent>
            </ContextMenu>
          ) : (
            <TreeRow key={i} row={r} />
          ),
        )}
        {keyspaces && rows.length === 0 && filter && (
          <div className="px-3 py-2 text-[11.5px] text-muted-foreground">
            No keyspaces match &quot;{filter}&quot;.
          </div>
        )}
      </div>
      <div className="flex items-center justify-between border-t px-2.5 py-1.5 text-[10.5px] text-muted-foreground">
        <span>
          <span className="mono">{totalKs}</span> keyspaces ·{" "}
          <span className="mono">{totalTables}</span> tables
        </span>
        <Button variant="ghost" size="icon-sm" aria-label="Add" disabled>
          <Plus size={11} strokeWidth={2} />
        </Button>
      </div>
    </div>
  );
}
