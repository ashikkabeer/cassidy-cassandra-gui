import * as React from "react";
import { toast } from "sonner";
import {
  Plus,
  Check,
  Search,
  Trash2,
  ChevronLeft,
  ChevronRight,
  Lock,
} from "lucide-react";
import { DataGrid, type GridColumn } from "@/components/data-grid";
import { ConfirmCqlDialog } from "@/components/confirm-cql-dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/primitives";
import {
  type RowsPage,
  type ColumnMeta,
  type ChangeSet,
  type Op,
  getRows,
  previewChanges,
  commitChanges,
} from "@/lib/dataedit";
import { APIError } from "@/lib/api";

interface TableDataProps {
  connId: string;
  ks: string;
  tbl: string;
  readOnly: boolean;
}

type NewRow = Record<string, string>;

function pkKind(c: ColumnMeta): "pk" | "ck" | null {
  if (c.kind === "partition_key") return "pk";
  if (c.kind === "clustering") return "ck";
  return null;
}

function toGridColumns(cols: ColumnMeta[]): GridColumn[] {
  return cols.map((c) => ({
    name: c.name,
    type: c.type,
    pk: pkKind(c),
    editable: c.editable,
    width: 160,
  }));
}

// Existing rows arrive as positional arrays; convert to objects keyed by column.
function rowToObject(cols: ColumnMeta[], row: unknown[]): Record<string, unknown> {
  const o: Record<string, unknown> = {};
  cols.forEach((c, i) => {
    o[c.name] = row[i];
  });
  return o;
}

export function TableData({ connId, ks, tbl, readOnly }: TableDataProps) {
  const [page, setPage] = React.useState<RowsPage | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [pageStack, setPageStack] = React.useState<string[]>([]);

  // Edit buffers.
  const [dirty, setDirty] = React.useState<Record<string, string>>({});
  const [deleted, setDeleted] = React.useState<number[]>([]);
  const [newRows, setNewRows] = React.useState<NewRow[]>([]);
  const [selected, setSelected] = React.useState<{ r: number; c: string } | null>(null);

  // Commit dialog.
  const [confirmOpen, setConfirmOpen] = React.useState(false);
  const [previewCQL, setPreviewCQL] = React.useState("");
  const [previewDeletes, setPreviewDeletes] = React.useState(0);
  const [committing, setCommitting] = React.useState(false);
  const [filter, setFilter] = React.useState("");

  const load = React.useCallback(
    async (pageState?: string) => {
      setLoading(true);
      setError(null);
      try {
        const p = await getRows(connId, ks, tbl, { pageSize: 100, pageState });
        setPage(p);
      } catch (e) {
        setError(e instanceof APIError ? e.message : "Failed to load rows");
      } finally {
        setLoading(false);
      }
    },
    [connId, ks, tbl],
  );

  const reset = React.useCallback(() => {
    setDirty({});
    setDeleted([]);
    setNewRows([]);
    setSelected(null);
  }, []);

  React.useEffect(() => {
    reset();
    setPageStack([]);
    void load();
  }, [load, reset]);

  if (loading && !page) {
    return (
      <div className="p-4">
        {Array.from({ length: 10 }).map((_, i) => (
          <div key={i} className="mb-2 flex gap-3">
            <Skeleton w={36} h={12} />
            <Skeleton w="20%" h={12} />
            <Skeleton w="15%" h={12} />
            <Skeleton w="20%" h={12} />
          </div>
        ))}
      </div>
    );
  }
  if (error) {
    return (
      <div className="flex h-full items-center justify-center p-10 text-center">
        <div>
          <div className="text-[13px] font-semibold text-[hsl(0_80%_75%)]">Couldn&apos;t load rows</div>
          <div className="mt-1 text-[12px] text-muted-foreground">{error}</div>
          <Button variant="outline" size="md" className="mt-3" onClick={() => void load()}>
            Retry
          </Button>
        </div>
      </div>
    );
  }
  if (!page) return null;

  const cols = page.columns;
  const existingObjs = page.rows.map((r) => rowToObject(cols, r));
  // New rows render after the existing ones; their grid index is offset.
  const newObjs: Record<string, unknown>[] = newRows.map((nr) => ({ ...nr }));
  const gridRows = [...existingObjs, ...newObjs];
  const gridColumns = toGridColumns(cols);

  const pkColNames = cols.filter((c) => c.kind === "partition_key" || c.kind === "clustering").map((c) => c.name);

  const pendingCount = Object.keys(dirty).length + deleted.length + newRows.length;

  const filtered = filter
    ? gridRows.filter((r) =>
        Object.values(r).some((v) => String(v ?? "").toLowerCase().includes(filter.toLowerCase())),
      )
    : gridRows;

  const onCellClick = (r: number, c: string) => setSelected({ r, c });

  const onCellChange = (r: number, c: string, value: string) => {
    if (r < existingObjs.length) {
      // Editing an existing row → dirty buffer (unless unchanged).
      const original = existingObjs[r][c];
      const key = `${r}:${c}`;
      setDirty((d) => {
        const next = { ...d };
        if (String(original ?? "") === value) delete next[key];
        else next[key] = value;
        return next;
      });
    } else {
      // Editing a new row.
      const ni = r - existingObjs.length;
      setNewRows((rows) => rows.map((row, i) => (i === ni ? { ...row, [c]: value } : row)));
    }
    setSelected(null);
  };

  const addRow = () => {
    const blank: NewRow = {};
    cols.forEach((c) => {
      blank[c.name] = "";
    });
    setNewRows((rows) => [...rows, blank]);
  };

  const toggleDelete = (r: number) => {
    if (r >= existingObjs.length) {
      // Removing a not-yet-committed new row.
      const ni = r - existingObjs.length;
      setNewRows((rows) => rows.filter((_, i) => i !== ni));
      return;
    }
    setDeleted((d) => (d.includes(r) ? d.filter((x) => x !== r) : [...d, r]));
  };

  function buildChangeSet(): ChangeSet {
    const ops: Op[] = [];
    // Updates on existing rows (skip rows marked for deletion).
    const updatesByRow: Record<number, Record<string, unknown>> = {};
    for (const key of Object.keys(dirty)) {
      const [rStr, col] = key.split(":");
      const r = Number(rStr);
      if (deleted.includes(r)) continue;
      (updatesByRow[r] ||= {})[col] = dirty[key];
    }
    for (const rStr of Object.keys(updatesByRow)) {
      const r = Number(rStr);
      const pk: Record<string, unknown> = {};
      pkColNames.forEach((p) => {
        pk[p] = existingObjs[r][p];
      });
      ops.push({ kind: "update", pk, set: updatesByRow[r] });
    }
    // Deletes.
    for (const r of deleted) {
      const pk: Record<string, unknown> = {};
      pkColNames.forEach((p) => {
        pk[p] = existingObjs[r][p];
      });
      ops.push({ kind: "delete", pk });
    }
    // Inserts (new rows).
    for (const nr of newRows) {
      const pk: Record<string, unknown> = {};
      const set: Record<string, unknown> = {};
      cols.forEach((c) => {
        const val = nr[c.name];
        if (val === "" || val == null) return;
        if (c.kind === "partition_key" || c.kind === "clustering") pk[c.name] = val;
        else set[c.name] = val;
      });
      ops.push({ kind: "insert", pk, set });
    }
    return { ops };
  }

  const openCommit = async () => {
    if (pendingCount === 0) return;
    try {
      const cs = buildChangeSet();
      const prev = await previewChanges(connId, ks, tbl, cs);
      setPreviewCQL(prev.cql);
      setPreviewDeletes(prev.delete_count);
      setConfirmOpen(true);
    } catch (e) {
      toast.error(e instanceof APIError ? e.message : "Preview failed");
    }
  };

  const doCommit = async () => {
    setCommitting(true);
    try {
      const cs = buildChangeSet();
      const res = await commitChanges(connId, ks, tbl, cs);
      toast.success(`${res.statement_count} change${res.statement_count === 1 ? "" : "s"} applied`);
      setConfirmOpen(false);
      reset();
      await load(); // re-fetch so the grid matches Cassandra
    } catch (e) {
      toast.error(e instanceof APIError ? e.message : "Commit failed");
    } finally {
      setCommitting(false);
    }
  };

  const nextPage = () => {
    if (!page.next_page_state) return;
    setPageStack((s) => [...s, page.next_page_state!]);
    reset();
    void load(page.next_page_state);
  };
  const prevPage = () => {
    if (pageStack.length === 0) return;
    const stack = [...pageStack];
    stack.pop();
    const target = stack.length > 0 ? stack[stack.length - 1] : undefined;
    setPageStack(stack);
    reset();
    void load(target);
  };

  return (
    <div className="flex h-full flex-col">
      <div className="flex items-center gap-1.5 border-b bg-background px-2.5 py-1.5">
        <Badge variant="info">TABLE</Badge>
        <span className="mono text-[12px] font-medium">
          {ks}.{tbl}
        </span>
        {readOnly && (
          <Badge variant="warning" icon={<Lock size={9} strokeWidth={2} />}>
            Read-only
          </Badge>
        )}
        <Separator orientation="vertical" className="mx-0.5 h-3.5" />
        <Button size="md" variant="outline" disabled={readOnly} onClick={addRow}>
          <Plus size={11} strokeWidth={2} />
          Add row
        </Button>
        <Button
          size="md"
          variant="outline"
          disabled={readOnly || pendingCount === 0}
          onClick={() => void openCommit()}
        >
          <Check size={11} strokeWidth={2} />
          Commit{pendingCount > 0 ? ` (${pendingCount})` : ""}
        </Button>
        <Button size="md" variant="ghost" disabled={pendingCount === 0} onClick={reset}>
          Discard
        </Button>
        {selected && selected.r < existingObjs.length && !readOnly && (
          <Button size="md" variant="ghost" onClick={() => toggleDelete(selected.r)}>
            <Trash2 size={11} strokeWidth={1.6} />
            {deleted.includes(selected.r) ? "Undelete row" : "Delete row"}
          </Button>
        )}
        <div className="flex-1" />
        <Input
          wrapperClassName="w-[180px] h-[24px] bg-panel text-[11px]"
          placeholder="Filter rows"
          icon={<Search size={11} strokeWidth={1.8} />}
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <Button size="md" variant="ghost" disabled={pageStack.length === 0} onClick={prevPage}>
          <ChevronLeft size={11} strokeWidth={2} />
        </Button>
        <Button size="md" variant="ghost" disabled={!page.next_page_state} onClick={nextPage}>
          <ChevronRight size={11} strokeWidth={2} />
        </Button>
      </div>

      <div className="min-h-0 flex-1">
        <DataGrid
          columns={gridColumns}
          rows={filter ? filtered : gridRows}
          editable={!readOnly}
          selectedCell={selected}
          dirtyCells={dirty}
          deleted={deleted}
          onCellClick={onCellClick}
          onCellChange={onCellChange}
          onCellBlur={() => setSelected(null)}
        />
      </div>

      <ConfirmCqlDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        cql={previewCQL}
        changeCount={pendingCount}
        deleteCount={previewDeletes}
        committing={committing}
        onConfirm={() => void doCommit()}
      />
    </div>
  );
}
