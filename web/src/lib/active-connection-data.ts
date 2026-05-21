// useActiveConnectionDetail() — fetches the full ConnectionDTO for whichever
// connection is currently active in the Zustand store. Falls back to null
// when no connection is selected or while the request is in flight. Auto-
// clears `activeId` when the connection 404s (e.g. it was deleted in another
// tab).
import * as React from "react";
import { useActiveConnection } from "@/lib/active-connection";
import { type ConnectionDTO, getConnection } from "@/lib/connections";
import { APIError } from "@/lib/api";

export type ActiveConnStatus = "idle" | "loading" | "ready" | "error";

export interface ActiveConnState {
  status: ActiveConnStatus;
  conn: ConnectionDTO | null;
  error: string | null;
  reload: () => void;
}

export function useActiveConnectionDetail(): ActiveConnState {
  const activeId = useActiveConnection((s) => s.activeId);
  const setActive = useActiveConnection((s) => s.setActive);
  const [conn, setConn] = React.useState<ConnectionDTO | null>(null);
  const [status, setStatus] = React.useState<ActiveConnStatus>(activeId ? "loading" : "idle");
  const [error, setError] = React.useState<string | null>(null);
  const [tick, setTick] = React.useState(0);

  React.useEffect(() => {
    if (!activeId) {
      setConn(null);
      setStatus("idle");
      setError(null);
      return;
    }
    let cancelled = false;
    setStatus("loading");
    setError(null);
    (async () => {
      try {
        const c = await getConnection(activeId);
        if (cancelled) return;
        setConn(c);
        setStatus("ready");
      } catch (e) {
        if (cancelled) return;
        if (e instanceof APIError && e.status === 404) {
          // The active connection vanished — clear the selection so we don't
          // keep retrying.
          setActive(null);
          setConn(null);
          setStatus("idle");
          setError(null);
          return;
        }
        setConn(null);
        setStatus("error");
        setError(e instanceof Error ? e.message : String(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeId, tick, setActive]);

  return {
    status,
    conn,
    error,
    reload: () => setTick((n) => n + 1),
  };
}
