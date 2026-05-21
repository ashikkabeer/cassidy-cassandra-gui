// CodeMirror 6 CQL language + schema-aware autocomplete.
//
// We define a Cassandra dialect by extending `@codemirror/lang-sql`'s
// `SQLDialect`. The differences from ANSI SQL are small (CQL keywords overlap
// heavily; main extras are CREATE KEYSPACE / TTL / USING / ALLOW FILTERING /
// BATCH). Anything we don't override behaves like SQL — fine for highlighting.
//
// The autocompletion source is the interesting bit: it merges (a) static
// keyword/type/function tokens with (b) live schema items pulled from the
// /completion endpoint, debounced so a fast typer doesn't hammer the server.

import { sql, SQLDialect } from "@codemirror/lang-sql";
import type { CompletionContext, CompletionResult, Completion } from "@codemirror/autocomplete";
import { getCompletion } from "@/lib/query";

const CQL_KEYWORDS = [
  "select from where and or not in limit order by asc desc",
  "insert into values update set delete",
  "create table keyspace type index materialized view",
  "drop alter truncate use with primary key if exists ttl using token",
  "allow filtering batch apply begin group clustering",
].join(" ");

const CQL_TYPES = [
  "text uuid timeuuid bigint int float double boolean timestamp date",
  "blob inet decimal varchar varint counter ascii duration",
  "set list map tuple frozen",
].join(" ");

export const cqlDialect = SQLDialect.define({
  keywords: CQL_KEYWORDS,
  types: CQL_TYPES,
  // CQL string literals use single quotes; double-quoted text is a quoted
  // identifier. lang-sql defaults to single quotes only — explicit for clarity.
  doubleQuotedStrings: false,
  hashComments: false,
  slashComments: true,
});

// cqlLanguage is the LanguageSupport object you pass to CodeMirror's
// `extensions={[cqlLanguage]}`. The `upperCaseKeywords` flag matches the
// design's editor screenshot which shows keywords in upper case.
export const cqlLanguage = sql({
  dialect: cqlDialect,
  upperCaseKeywords: true,
});

// kindToCMType maps our backend's completion kind to CodeMirror's icon type
// strings. CodeMirror styles each `type` differently in the popover.
function kindToCMType(kind: string): string {
  switch (kind) {
    case "keyspace":
      return "namespace";
    case "table":
      return "class";
    case "column":
      return "property";
    case "function":
      return "function";
    case "type":
      return "type";
    case "keyword":
    default:
      return "keyword";
  }
}

// makeCqlCompletion returns an autocomplete source closure bound to a specific
// connection. The keyspace callback is read lazily because the workspace's
// active keyspace can change while the editor is open.
export function makeCqlCompletion(
  getConnId: () => string | null,
  getKeyspace: () => string,
) {
  let lastPrefix = "";
  let lastFetch = 0;
  let cache: Completion[] = [];

  return async (ctx: CompletionContext): Promise<CompletionResult | null> => {
    const word = ctx.matchBefore(/[A-Za-z_][\w]*/);
    if (!word || (word.from === word.to && !ctx.explicit)) return null;

    const connId = getConnId();
    if (!connId) return null;

    const prefix = word.text;
    const now = Date.now();

    // Refetch only when the prefix changed and 150 ms elapsed.
    if (prefix !== lastPrefix || now - lastFetch > 150) {
      lastPrefix = prefix;
      lastFetch = now;
      try {
        const suggestions = await getCompletion(connId, prefix, getKeyspace());
        cache = suggestions.map<Completion>((s) => ({
          label: s.label,
          type: kindToCMType(s.kind),
          detail: s.detail,
        }));
      } catch {
        // Swallow — local keyword fallback still applies via lang-sql.
      }
    }

    return {
      from: word.from,
      options: cache,
      validFor: /^[\w]*$/,
    };
  };
}
