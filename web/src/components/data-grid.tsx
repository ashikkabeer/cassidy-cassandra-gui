import * as React from "react";
import { KeyRound, Zap, ChevronUp, ChevronDown, Lock, Copy, Check } from "lucide-react";
import { cn } from "@/lib/utils";
import { toast } from "sonner";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

export type PkKind = "pk" | "ck" | null;

// If the string parses as a JSON object/array, return it pretty-printed; else null.
// Used by the cell inspector to format JSON stored in text/varchar columns.
function tryFormatJSON(s: string): string | null {
  const t = s.trim();
  if (!(t.startsWith("{") || t.startsWith("["))) return null;
  try {
    return JSON.stringify(JSON.parse(t), null, 2);
  } catch {
    return null;
  }
}

export interface GridColumn {
  name: string;
  type: string;
  pk?: PkKind;
  width?: number;
  /** When false, the cell is locked even in editable grids (counter/collection,
   *  or any non-PK column the caller wants read-only). PK/CK are always locked. */
  editable?: boolean;
}

export function PkIcon({ kind }: { kind?: PkKind }) {
  if (kind === "pk")
    return (
      <span className="pk-pill" title="Partition key">
        <KeyRound size={10} strokeWidth={2} />
      </span>
    );
  if (kind === "ck")
    return (
      <span className="ck-pill" title="Clustering key">
        <Zap size={10} strokeWidth={2} />
      </span>
    );
  return null;
}

export interface DataGridProps {
  columns: GridColumn[];
  rows: Record<string, unknown>[];
  sortCol?: string;
  sortDir?: "asc" | "desc";
  editable?: boolean;
  selectedCell?: { r: number; c: string } | null;
  /** dirty values keyed `${rowIndex}:${colName}`. */
  dirtyCells?: Record<string, string>;
  deleted?: number[];
  compact?: boolean;
  /** Selecting an editable cell. */
  onCellClick?: (r: number, c: string) => void;
  /** Committing an edit (Enter / blur). */
  onCellChange?: (r: number, c: string, value: string) => void;
  /** Cancelling an edit (Esc). */
  onCellBlur?: () => void;
}

// cellEditable reports whether a column can be edited inline. PK/CK are never
// editable; otherwise honor the column's explicit `editable` flag (defaults true
// when the grid is editable and the flag is unset, for the query-result grid).
function cellEditable(c: GridColumn): boolean {
  if (c.pk === "pk" || c.pk === "ck") return false;
  return c.editable !== false;
}

export function DataGrid({
  columns,
  rows,
  sortCol,
  sortDir,
  editable,
  selectedCell,
  dirtyCells = {},
  deleted = [],
  compact = true,
  onCellClick,
  onCellChange,
  onCellBlur,
}: DataGridProps) {
  const rowH = compact ? 26 : 32;

  // Cell value inspector — double-click any cell to see/copy its full value
  // (handy for long JSON stored in text/varchar columns).
  const [peek, setPeek] = React.useState<{ col: string; type: string; value: string } | null>(null);
  const [formatted, setFormatted] = React.useState(true);
  const [copied, setCopied] = React.useState(false);
  const openPeek = (col: string, type: string, value: string) => {
    setPeek({ col, type, value });
    setFormatted(true);
    setCopied(false);
  };

  return (
    <div className="flex h-full flex-col bg-background">
      {/* Single scroll container so the header scrolls horizontally in lockstep
          with the rows; the header stays pinned vertically via `sticky top-0`.
          `w-max min-w-full` makes header + rows share one width so columns align. */}
      <div className="relative flex-1 overflow-auto overscroll-x-contain">
        <div className="sticky top-0 z-[2] flex w-max min-w-full border-b bg-panel text-[11px] text-muted-foreground">
          <div className="w-9 shrink-0 border-r py-1 text-center">#</div>
          {columns.map((c) => (
            <div
              key={c.name}
              style={{ width: c.width ?? 140 }}
              className="flex shrink-0 items-center gap-1.5 overflow-hidden border-r px-2 py-1"
            >
              <PkIcon kind={c.pk ?? null} />
              <span
                className="mono min-w-0 truncate text-[11px] font-medium text-foreground"
                title={c.name}
              >
                {c.name}
              </span>
              <span className="mono shrink-0 text-[10px] text-muted-foreground">{c.type}</span>
              <span className="flex-1" />
              {sortCol === c.name &&
                (sortDir === "desc" ? (
                  <ChevronDown size={10} strokeWidth={2} />
                ) : (
                  <ChevronUp size={10} strokeWidth={2} />
                ))}
            </div>
          ))}
          <div className="flex-1 border-l" />
        </div>

        {rows.map((r, i) => {
          const isDel = deleted.includes(i);
          return (
            <div
              key={i}
              style={{ height: rowH }}
              className={cn(
                "flex w-max min-w-full border-b border-border/50",
                isDel
                  ? "bg-[hsl(var(--destructive)/0.08)] line-through opacity-55"
                  : i % 2
                    ? "bg-transparent"
                    : "bg-panel/40",
              )}
            >
              <div className="mono flex w-9 shrink-0 items-center justify-center border-r text-[10.5px] text-muted-foreground">
                {i + 1}
              </div>
              {columns.map((c) => {
                const raw = (r as Record<string, unknown>)[c.name];
                const dirty = dirtyCells[`${i}:${c.name}`];
                const v = (dirty !== undefined ? dirty : raw) as
                  | string
                  | number
                  | null
                  | undefined;
                const isSel = selectedCell && selectedCell.r === i && selectedCell.c === c.name;
                const colEditable = editable && cellEditable(c);
                const locked = editable && !cellEditable(c);
                return (
                  <div
                    key={c.name}
                    onClick={() => {
                      if (colEditable && !isDel) onCellClick?.(i, c.name);
                    }}
                    onDoubleClick={() => openPeek(c.name, c.type, v == null ? "" : String(v))}
                    title="Double-click to view full value"
                    style={{ width: c.width ?? 140 }}
                    className={cn(
                      "relative flex shrink-0 items-center border-r px-2",
                      isSel
                        ? "bg-accent"
                        : dirty !== undefined
                          ? "bg-[hsl(var(--warning)/0.12)]"
                          : undefined,
                      colEditable && !isDel ? "cursor-cell" : "cursor-default",
                    )}
                  >
                    {isSel && (
                      <span
                        className="pointer-events-none absolute inset-0 rounded-[1px]"
                        style={{ boxShadow: "inset 0 0 0 1.5px hsl(var(--ring))" }}
                      />
                    )}
                    {dirty !== undefined && (
                      <span className="absolute right-1 top-1 h-[5px] w-[5px] rounded-full bg-[hsl(var(--warning))]" />
                    )}
                    {locked && (
                      <span className="absolute right-1 top-1 opacity-50" title="Not editable inline">
                        <Lock size={9} strokeWidth={1.6} />
                      </span>
                    )}
                    {isSel && colEditable ? (
                      <input
                        autoFocus
                        defaultValue={v == null ? "" : String(v)}
                        className="mono w-full bg-transparent text-[11.5px] outline-none"
                        onKeyDown={(e) => {
                          if (e.key === "Enter") {
                            onCellChange?.(i, c.name, (e.target as HTMLInputElement).value);
                          } else if (e.key === "Escape") {
                            onCellBlur?.();
                          }
                        }}
                        onBlur={(e) => onCellChange?.(i, c.name, e.target.value)}
                      />
                    ) : v == null || v === "" ? (
                      <span className="mono text-[10.5px] italic text-muted-foreground">null</span>
                    ) : (
                      <span
                        className={cn(
                          "mono truncate text-[11.5px]",
                          typeof v === "number" || /^-?\d+(\.\d+)?$/.test(String(v))
                            ? "text-[hsl(28_90%_70%)]"
                            : v === "true" || v === "false"
                              ? "text-[hsl(260_60%_75%)]"
                              : "text-foreground",
                        )}
                      >
                        {String(v)}
                      </span>
                    )}
                  </div>
                );
              })}
              <div className="flex-1 border-l" />
            </div>
          );
        })}
      </div>

      <Dialog open={!!peek} onOpenChange={(o) => !o && setPeek(null)}>
        {peek &&
          (() => {
            const pretty = tryFormatJSON(peek.value);
            const isJson = pretty !== null;
            const shown = isJson && formatted ? pretty! : peek.value;
            return (
              <DialogContent
                width={680}
                title={peek.col}
                subtitle={`${peek.type}${isJson ? " · JSON" : ""} · ${peek.value.length} chars`}
                footer={
                  <>
                    {isJson && (
                      <Button variant="outline" size="sm" onClick={() => setFormatted((f) => !f)}>
                        {formatted ? "Raw" : "Formatted"}
                      </Button>
                    )}
                    <span className="flex-1" />
                    <Button
                      size="sm"
                      onClick={async () => {
                        await navigator.clipboard.writeText(shown);
                        setCopied(true);
                        toast.success("Copied value");
                        setTimeout(() => setCopied(false), 1500);
                      }}
                    >
                      {copied ? <Check size={12} strokeWidth={2} /> : <Copy size={12} strokeWidth={1.6} />}
                      {copied ? "Copied" : "Copy"}
                    </Button>
                  </>
                }
              >
                {peek.value === "" ? (
                  <div className="text-[12px] italic text-muted-foreground">(empty / null)</div>
                ) : (
                  <textarea
                    readOnly
                    value={shown}
                    spellCheck={false}
                    onFocus={(e) => e.currentTarget.select()}
                    className="mono h-[55vh] w-full resize-none rounded-[var(--radius)] border bg-panel p-2.5 text-[12px] leading-[1.5] outline-none"
                  />
                )}
              </DialogContent>
            );
          })()}
      </Dialog>
    </div>
  );
}
