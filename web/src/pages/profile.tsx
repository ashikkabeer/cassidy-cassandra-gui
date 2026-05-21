import * as React from "react";
import { Lock, Monitor, Smartphone, Database } from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Field } from "@/components/ui/field";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useAuth } from "@/lib/auth-store";
import { api, APIError } from "@/lib/api";

interface SessionDTO {
  id: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  ip_address?: string;
  user_agent?: string;
  is_current?: boolean;
}

function deviceIcon(ua?: string) {
  if (!ua) return <Monitor size={14} strokeWidth={1.6} />;
  if (/iPhone|iPad|Android/i.test(ua)) return <Smartphone size={14} strokeWidth={1.6} />;
  if (/cassidy-cli|curl|httpie/i.test(ua)) return <Database size={14} strokeWidth={1.6} />;
  return <Monitor size={14} strokeWidth={1.6} />;
}

function describeUA(ua?: string) {
  if (!ua) return "Unknown client";
  if (ua.includes("Chrome")) return "Chrome";
  if (ua.includes("Firefox")) return "Firefox";
  if (ua.includes("Safari")) return "Safari";
  return ua.slice(0, 40);
}

function fromNow(iso: string): string {
  const t = new Date(iso).getTime();
  const diff = Date.now() - t;
  if (diff < 60_000) return "now";
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)} min ago`;
  if (diff < 86_400_000) return `${Math.floor(diff / 3600_000)} hr ago`;
  return `${Math.floor(diff / 86_400_000)} days ago`;
}

export function ProfilePage() {
  const user = useAuth((s) => s.user);
  const logout = useAuth((s) => s.logout);

  const [sessions, setSessions] = React.useState<SessionDTO[]>([]);
  const [current, setCurrent] = React.useState("");
  const [newPw, setNewPw] = React.useState("");
  const [confirm, setConfirm] = React.useState("");
  const [pwBusy, setPwBusy] = React.useState(false);
  const [pwError, setPwError] = React.useState<string | null>(null);
  const [pwOk, setPwOk] = React.useState(false);

  const loadSessions = React.useCallback(async () => {
    if (!user) return;
    try {
      const list = await api<SessionDTO[]>(`/users/${user.id}/sessions`);
      setSessions(list ?? []);
    } catch {
      /* ignore */
    }
  }, [user]);

  React.useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

  if (!user) return null;

  const submitPw = async (e: React.FormEvent) => {
    e.preventDefault();
    setPwOk(false);
    if (newPw !== confirm) {
      setPwError("Passwords don't match.");
      return;
    }
    if (newPw.length < 12) {
      setPwError("Password must be at least 12 characters.");
      return;
    }
    setPwBusy(true);
    setPwError(null);
    try {
      await api("/auth/change-password", {
        method: "POST",
        body: { current, new: newPw },
      });
      setPwOk(true);
      setCurrent("");
      setNewPw("");
      setConfirm("");
    } catch (err) {
      setPwError(err instanceof APIError ? err.message : "Failed.");
    } finally {
      setPwBusy(false);
    }
  };

  const revoke = async (id: string) => {
    try {
      await api(`/sessions/${id}`, { method: "DELETE" });
      await loadSessions();
    } catch {
      /* ignore */
    }
  };

  const revokeOthers = async () => {
    const others = sessions.filter((s) => !s.is_current);
    for (const s of others) {
      try {
        await api(`/sessions/${s.id}`, { method: "DELETE" });
      } catch {
        /* ignore */
      }
    }
    await loadSessions();
  };

  const initials = user.username
    .split(/[\s_]/)
    .filter(Boolean)
    .slice(0, 2)
    .map((s) => s[0]?.toUpperCase() ?? "")
    .join("");

  const strength = Math.min(100, Math.floor((newPw.length / 16) * 100));
  const strengthLabel =
    newPw.length === 0 ? "" : newPw.length < 12 ? "Too short" : newPw.length < 16 ? "OK" : "Strong";

  return (
    <AppShell>
      <div className="h-full overflow-auto px-8 py-6">
        <div className="max-w-[580px]">
          <div className="mb-5">
            <div className="text-[16px] font-semibold tracking-[-0.2px]">Profile</div>
            <div className="mt-0.5 text-[11.5px] text-muted-foreground">
              Account settings for <span className="mono">{user.username}</span>
            </div>
          </div>

          <Card className="p-4">
            <div className="flex items-center gap-3">
              <div
                className="flex h-12 w-12 items-center justify-center rounded-full text-[18px] font-semibold"
                style={{ background: "hsl(210 30% 28%)", color: "hsl(210 70% 80%)" }}
              >
                {initials}
              </div>
              <div className="flex-1">
                <div className="text-[14px] font-semibold">{user.username}</div>
                <div className="text-[11.5px] text-muted-foreground">
                  {user.role} · joined {user.created_at.slice(0, 10)}
                </div>
              </div>
              <Button variant="outline" size="md" onClick={() => logout()}>
                Sign out
              </Button>
            </div>
            <Separator className="my-3.5" />
            <div className="grid grid-cols-2 gap-3">
              <Field label="Username">
                <Input value={user.username} readOnly className="mono" />
              </Field>
              <Field label="Email">
                <Input value={user.email ?? ""} readOnly className="mono" />
              </Field>
              <Field label="Default keyspace">
                <Select defaultValue="telemetry">
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="telemetry">telemetry</SelectItem>
                    <SelectItem value="analytics">analytics</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
              <Field label="Theme">
                <Select defaultValue="system-dark">
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="system-dark">System (dark)</SelectItem>
                    <SelectItem value="dark">Dark</SelectItem>
                  </SelectContent>
                </Select>
              </Field>
            </div>
          </Card>

          <Card className="mt-3.5 p-4">
            <div className="mb-3 flex items-center gap-2">
              <Lock size={14} strokeWidth={1.8} className="text-muted-foreground" />
              <div className="text-[13px] font-semibold">Change password</div>
            </div>
            <form onSubmit={submitPw} className="flex flex-col gap-2.5">
              {pwError && (
                <div className="rounded-[var(--radius)] border border-[hsl(var(--destructive)/0.35)] bg-[hsl(var(--destructive)/0.12)] px-2.5 py-2 text-[11.5px] text-[hsl(0_80%_80%)]">
                  {pwError}
                </div>
              )}
              {pwOk && (
                <div className="rounded-[var(--radius)] border border-[hsl(var(--success)/0.35)] bg-[hsl(var(--success)/0.12)] px-2.5 py-2 text-[11.5px] text-[hsl(var(--success))]">
                  Password updated.
                </div>
              )}
              <Field label="Current password">
                <Input
                  type="password"
                  placeholder="••••••••"
                  value={current}
                  onChange={(e) => setCurrent(e.target.value)}
                />
              </Field>
              <div className="grid grid-cols-2 gap-2.5">
                <Field label="New password" help="≥ 12 chars · mixed case · symbol">
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={newPw}
                    onChange={(e) => setNewPw(e.target.value)}
                  />
                </Field>
                <Field label="Confirm new password">
                  <Input
                    type="password"
                    placeholder="••••••••••••"
                    value={confirm}
                    onChange={(e) => setConfirm(e.target.value)}
                  />
                </Field>
              </div>
              {newPw.length > 0 && (
                <div className="flex items-center gap-2 text-[11.5px]">
                  <div className="h-1 flex-1 overflow-hidden rounded-full bg-secondary">
                    <div
                      className={
                        newPw.length < 12
                          ? "h-full bg-[hsl(0_70%_55%)]"
                          : newPw.length < 16
                            ? "h-full bg-[hsl(var(--warning))]"
                            : "h-full bg-[hsl(var(--success))]"
                      }
                      style={{ width: `${strength}%` }}
                    />
                  </div>
                  <span className="text-muted-foreground">{strengthLabel}</span>
                </div>
              )}
              <div className="mt-1 flex justify-end gap-1.5">
                <Button
                  type="button"
                  variant="ghost"
                  size="md"
                  onClick={() => {
                    setCurrent("");
                    setNewPw("");
                    setConfirm("");
                    setPwError(null);
                    setPwOk(false);
                  }}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  variant="default"
                  size="md"
                  disabled={pwBusy || !current || !newPw || !confirm}
                >
                  Update password
                </Button>
              </div>
            </form>
          </Card>

          <Card className="mt-3.5 p-4">
            <div className="mb-3 flex items-center gap-2">
              <Database size={14} strokeWidth={1.8} className="text-muted-foreground" />
              <div className="text-[13px] font-semibold">Active sessions</div>
              <div className="flex-1" />
              {sessions.filter((s) => !s.is_current).length > 0 && (
                <Button
                  variant="destructive-outline"
                  size="sm"
                  onClick={() => void revokeOthers()}
                >
                  Revoke all others
                </Button>
              )}
            </div>
            {sessions.length === 0 && (
              <div className="py-4 text-center text-[11.5px] text-muted-foreground">
                No active sessions.
              </div>
            )}
            {sessions.map((s, i) => (
              <div
                key={s.id}
                className="flex items-center gap-2.5 py-2"
                style={{
                  borderTop:
                    i === 0
                      ? "1px solid hsl(var(--border))"
                      : "1px solid hsl(var(--border) / 0.6)",
                }}
              >
                <span className="shrink-0 text-muted-foreground">
                  {deviceIcon(s.user_agent)}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1.5 text-[12px] font-medium">
                    {describeUA(s.user_agent)}
                    {s.is_current && <Badge variant="success">This session</Badge>}
                  </div>
                  <div className="mono text-[10.5px] text-muted-foreground">
                    {s.ip_address ?? "—"} · {fromNow(s.last_seen_at)}
                  </div>
                </div>
                {!s.is_current && (
                  <Button
                    size="sm"
                    variant="destructive-outline"
                    onClick={() => void revoke(s.id)}
                  >
                    Revoke
                  </Button>
                )}
              </div>
            ))}
          </Card>
        </div>
      </div>
    </AppShell>
  );
}

export default ProfilePage;
