import * as React from "react";
import { Code2, Table2, KeyRound } from "lucide-react";
import { Highlight } from "@/components/cql-highlight";
import { cn } from "@/lib/utils";

export interface SuggestItem {
  name: string;
  kind: "col" | "kw" | "tbl" | "pk";
  type: string;
  active?: boolean;
}

interface CqlEditorProps {
  lines: string[];
  caretLine?: number;
  caretCol?: number;
  showSuggest?: boolean;
  suggestItems?: SuggestItem[];
  suggestAt?: { col: number };
  suggestTitle?: string;
  height?: string | number;
}

export function CqlEditor({
  lines,
  caretLine,
  caretCol,
  showSuggest,
  suggestItems = [],
  suggestAt,
  suggestTitle,
  height = "100%",
}: CqlEditorProps) {
  const lineH = 20;
  return (
    <div
      style={{ height }}
      className="mono relative flex overflow-hidden bg-background text-[12.5px]"
    >
      <div
        className="shrink-0 select-none border-r py-1.5 pl-0 pr-1.5 text-right text-[10.5px] text-muted-foreground"
        style={{ width: 36, lineHeight: `${lineH}px` }}
      >
        {lines.map((_, i) => (
          <div
            key={i}
            style={{ height: lineH }}
            className={cn(i + 1 === caretLine && "text-foreground")}
          >
            {i + 1}
          </div>
        ))}
      </div>
      <div
        className="relative flex-1 overflow-auto px-3.5 py-1.5"
        style={{ lineHeight: `${lineH}px` }}
      >
        {lines.map((ln, i) => {
          const isCaret = i + 1 === caretLine;
          return (
            <div
              key={i}
              style={{ height: lineH }}
              className={cn(
                "relative -mx-3.5 px-3.5",
                isCaret && "bg-accent/40",
              )}
            >
              {isCaret && caretCol != null ? (
                <>
                  <Highlight src={ln.slice(0, caretCol)} />
                  <span className="caret" />
                  <Highlight src={ln.slice(caretCol)} />
                </>
              ) : (
                <Highlight src={ln} />
              )}
            </div>
          );
        })}

        {showSuggest && suggestAt && caretLine != null && (
          <div
            className="absolute z-10 min-w-[240px] rounded-[var(--radius)] border border-[hsl(var(--border-strong))] bg-popover font-sans text-[12px] shadow-[0_8px_24px_rgba(0,0,0,.5)]"
            style={{
              top: caretLine * lineH + 6,
              left: suggestAt.col * 7.4 + 14,
            }}
          >
            {suggestTitle && (
              <div className="border-b px-2.5 py-1.5 text-[10.5px] uppercase tracking-[0.4px] text-muted-foreground">
                {suggestTitle}
              </div>
            )}
            <div className="max-h-[200px] overflow-auto p-1">
              {suggestItems.map((it, idx) => (
                <div
                  key={idx}
                  className={cn(
                    "flex items-center gap-2 rounded-sm px-2 py-1",
                    it.active && "bg-accent",
                  )}
                >
                  <span className="flex w-3.5 justify-center text-muted-foreground">
                    {it.kind === "col" && <Table2 size={10} strokeWidth={1.5} />}
                    {it.kind === "kw" && <Code2 size={10} strokeWidth={1.5} />}
                    {it.kind === "tbl" && <Table2 size={10} strokeWidth={1.5} />}
                    {it.kind === "pk" && <KeyRound size={10} strokeWidth={1.5} />}
                  </span>
                  <span className="mono flex-1 text-[11.5px]">{it.name}</span>
                  <span className="mono text-[10.5px] text-muted-foreground">
                    {it.type}
                  </span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
