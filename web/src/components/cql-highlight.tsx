import * as React from "react";

const CQL_KW = new Set([
  "SELECT","FROM","WHERE","AND","OR","NOT","IN","LIMIT","ORDER","BY","ASC","DESC",
  "INSERT","INTO","VALUES","UPDATE","SET","DELETE","CREATE","TABLE","KEYSPACE","TYPE",
  "INDEX","MATERIALIZED","VIEW","DROP","ALTER","TRUNCATE","USE","WITH","PRIMARY","KEY",
  "IF","EXISTS","TTL","USING","TOKEN","ALLOW","FILTERING","BATCH","APPLY","BEGIN","GROUP",
  "CLUSTERING",
]);

const CQL_TY = new Set([
  "text","uuid","timeuuid","bigint","int","float","double","boolean","timestamp","date",
  "blob","inet","decimal","varchar","varint","counter","ascii","duration","set","list",
  "map","tuple","frozen",
]);

type Tok = { t: string; v?: string };

function tokenize(src: string): Tok[] {
  const out: Tok[] = [];
  let i = 0;
  while (i < src.length) {
    const c = src[i];
    if (c === "\n") {
      out.push({ t: "nl" });
      i++;
      continue;
    }
    if (/\s/.test(c)) {
      let j = i;
      while (j < src.length && /[ \t]/.test(src[j])) j++;
      out.push({ t: "sp", v: src.slice(i, j) });
      i = j;
      continue;
    }
    if (c === "-" && src[i + 1] === "-") {
      let j = i;
      while (j < src.length && src[j] !== "\n") j++;
      out.push({ t: "c", v: src.slice(i, j) });
      i = j;
      continue;
    }
    if (c === "'" || c === '"') {
      let j = i + 1;
      while (j < src.length && src[j] !== c) j++;
      out.push({ t: "str", v: src.slice(i, j + 1) });
      i = j + 1;
      continue;
    }
    if (/[0-9]/.test(c)) {
      let j = i;
      while (j < src.length && /[0-9.]/.test(src[j])) j++;
      out.push({ t: "num", v: src.slice(i, j) });
      i = j;
      continue;
    }
    if (/[A-Za-z_]/.test(c)) {
      let j = i;
      while (j < src.length && /[A-Za-z0-9_]/.test(src[j])) j++;
      const w = src.slice(i, j);
      const upper = w.toUpperCase();
      if (CQL_KW.has(upper)) out.push({ t: "kw", v: w });
      else if (CQL_TY.has(w.toLowerCase())) out.push({ t: "type", v: w });
      else out.push({ t: "id", v: w });
      i = j;
      continue;
    }
    if ("(){},;.*=".indexOf(c) >= 0) {
      out.push({ t: "pn", v: c });
      i++;
      continue;
    }
    out.push({ t: "id", v: c });
    i++;
  }
  return out;
}

const tokenClass: Record<string, string> = {
  kw: "tk-kw",
  type: "tk-type",
  str: "tk-str",
  num: "tk-num",
  id: "tk-id",
  pn: "tk-pn",
  c: "tk-c",
};

export function Highlight({ src }: { src: string }) {
  const toks = tokenize(src);
  return (
    <>
      {toks.map((tk, i) =>
        tk.t === "nl" ? (
          <br key={i} />
        ) : tk.t === "sp" ? (
          <span key={i}>{tk.v}</span>
        ) : (
          <span key={i} className={tokenClass[tk.t]}>
            {tk.v}
          </span>
        ),
      )}
    </>
  );
}

export { CQL_KW, CQL_TY };
