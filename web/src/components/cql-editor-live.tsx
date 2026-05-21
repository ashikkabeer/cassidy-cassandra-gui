import * as React from "react";
import CodeMirror, { type ReactCodeMirrorRef } from "@uiw/react-codemirror";
import { EditorView, keymap } from "@codemirror/view";
import { Prec } from "@codemirror/state";
import { autocompletion } from "@codemirror/autocomplete";
import { cqlLanguage, makeCqlCompletion } from "@/lib/cql-language";

// Dark theme matching tokens.css so the editor blends into the workspace.
const cassidyTheme = EditorView.theme(
  {
    "&": {
      backgroundColor: "hsl(var(--background))",
      color: "hsl(var(--foreground))",
      fontSize: "12.5px",
      height: "100%",
    },
    ".cm-content": {
      fontFamily: "'JetBrains Mono', ui-monospace, monospace",
      caretColor: "hsl(var(--foreground))",
    },
    ".cm-gutters": {
      backgroundColor: "hsl(var(--background))",
      color: "hsl(var(--muted-foreground))",
      border: "none",
      borderRight: "1px solid hsl(var(--border))",
    },
    ".cm-activeLine": { backgroundColor: "hsl(var(--accent) / 0.4)" },
    ".cm-activeLineGutter": { backgroundColor: "hsl(var(--accent) / 0.4)" },
    ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": {
      backgroundColor: "hsl(var(--accent)) !important",
    },
    ".cm-cursor": { borderLeftColor: "hsl(var(--foreground))" },
    "&.cm-focused": { outline: "none" },
    ".cm-tooltip": {
      backgroundColor: "hsl(var(--popover))",
      border: "1px solid hsl(var(--border-strong))",
      borderRadius: "var(--radius)",
      color: "hsl(var(--popover-foreground))",
    },
    ".cm-tooltip-autocomplete ul li[aria-selected]": {
      backgroundColor: "hsl(var(--accent))",
      color: "hsl(var(--accent-foreground))",
    },
  },
  { dark: true },
);

export interface CqlEditorLiveProps {
  value: string;
  onChange: (v: string) => void;
  onRun: () => void;
  /** Reports the currently-selected text ("" when nothing is selected). */
  onSelectionChange?: (selected: string) => void;
  readOnly?: boolean;
  connId: string | null;
  keyspace?: string;
}

export function CqlEditorLive({
  value,
  onChange,
  onRun,
  onSelectionChange,
  readOnly,
  connId,
  keyspace,
}: CqlEditorLiveProps) {
  const ref = React.useRef<ReactCodeMirrorRef>(null);

  // Keep the latest run handler / context in refs so the extensions array
  // stays stable (rebuilding it on every keystroke would thrash CodeMirror).
  const onRunRef = React.useRef(onRun);
  onRunRef.current = onRun;
  const onSelectionChangeRef = React.useRef(onSelectionChange);
  onSelectionChangeRef.current = onSelectionChange;
  const connIdRef = React.useRef(connId);
  connIdRef.current = connId;
  const keyspaceRef = React.useRef(keyspace ?? "");
  keyspaceRef.current = keyspace ?? "";

  const extensions = React.useMemo(() => {
    const runKeymap = Prec.highest(
      keymap.of([
        {
          key: "Mod-Enter",
          run: () => {
            onRunRef.current();
            return true;
          },
        },
      ]),
    );
    const completion = autocompletion({
      override: [
        makeCqlCompletion(
          () => connIdRef.current,
          () => keyspaceRef.current,
        ),
      ],
    });
    // Report selection changes so the toolbar Run / ⌘↵ can run only the
    // highlighted query when there is a non-empty selection.
    const selectionWatcher = EditorView.updateListener.of((u) => {
      if (!u.selectionSet && !u.docChanged) return;
      const sel = u.state.selection.main;
      onSelectionChangeRef.current?.(u.state.sliceDoc(sel.from, sel.to));
    });
    return [cqlLanguage, completion, runKeymap, selectionWatcher, EditorView.lineWrapping];
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <CodeMirror
      ref={ref}
      value={value}
      onChange={onChange}
      theme={cassidyTheme}
      extensions={extensions}
      readOnly={readOnly}
      basicSetup={{
        lineNumbers: true,
        highlightActiveLine: true,
        highlightActiveLineGutter: true,
        foldGutter: false,
        autocompletion: true,
        bracketMatching: true,
        closeBrackets: true,
      }}
      height="100%"
      style={{ height: "100%", overflow: "auto" }}
    />
  );
}
