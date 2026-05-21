import * as React from "react";
import {
  Search,
  Plus,
  MoreHorizontal,
  Shield,
  Copy,
  Check as CheckIcon,
} from "lucide-react";
import { AppShell } from "@/components/app-shell";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Field } from "@/components/ui/field";
import { Badge } from "@/components/ui/badge";
import { Skeleton, StatusDot } from "@/components/primitives";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Dialog, DialogContent } from "@/components/ui/dialog";
import { api, APIError } from "@/lib/api";
import { type User } from "@/lib/auth-store";

type UserRow = User;

function Avatar({ name }: { name: string }) {
  const initials = name
    .split(/[\s_]/)
    .filter(Boolean)
    .slice(0, 2)
    .map((s) => s[0]?.toUpperCase() ?? "")
    .join("");
  const hues = [210, 30, 280, 170, 340, 90];
  const h = hues[(name.charCodeAt(0) || 0) % hues.length];
  return (
    <span
      className="inline-flex h-[22px] w-[22px] items-center justify-center rounded-full text-[9.5px] font-semibold"
      style={{ background: `hsl(${h} 30% 28%)`, color: `hsl(${h} 70% 80%)` }}
    >
      {initials}
    </span>
  );
}

function RoleBadge({ role }: { role: UserRow["role"] }) {
  if (role === "admin") {
    return (
      <Badge variant="info" icon={<Shield size={9} strokeWidth={2} />}>
        admin
      </Badge>
    );
  }
  if (role === "editor") return <Badge variant="secondary">editor</Badge>;
  return <Badge variant="outline">viewer</Badge>;
}

interface InviteResult {
  user: UserRow;
  temp_password?: string;
}

function InviteDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  onCreated: () => void;
}) {
  const [username, setUsername] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [role, setRole] = React.useState<UserRow["role"]>("editor");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [result, setResult] = React.useState<InviteResult | null>(null);
  const [copied, setCopied] = React.useState(false);

  React.useEffect(() => {
    if (!open) {
      setUsername("");
      setEmail("");
      setRole("editor");
      setError(null);
      setResult(null);
      setCopied(false);
    }
  }, [open]);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await api<InviteResult>("/users", {
        method: "POST",
        body: { username, email, role },
      });
      setResult(res);
      onCreated();
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to invite.");
    } finally {
      setBusy(false);
    }
  };

  const copyTemp = async () => {
    if (!result?.temp_password) return;
    await navigator.clipboard.writeText(result.temp_password);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  const footer = result ? (
    <>
      <div className="flex-1" />
      <Button variant="default" size="md" onClick={() => onOpenChange(false)}>
        Done
      </Button>
    </>
  ) : (
    <>
      <div className="flex-1" />
      <Button variant="ghost" size="md" onClick={() => onOpenChange(false)}>
        Cancel
      </Button>
      <Button
        variant="default"
        size="md"
        disabled={busy || !username}
        onClick={submit}
      >
        Invite
      </Button>
    </>
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        width={460}
        title={result ? "User created" : "Invite user"}
        subtitle={
          result
            ? "Share this temporary password — it won't be shown again."
            : "We'll generate a one-time password they can reset on first login."
        }
        footer={footer}
      >
        {result ? (
          <div className="flex flex-col gap-3">
            <Field label="Username">
              <Input value={result.user.username} readOnly className="mono" />
            </Field>
            <Field label="Temporary password">
              <Input
                value={result.temp_password ?? ""}
                readOnly
                className="mono"
                suffix={
                  <button
                    type="button"
                    onClick={copyTemp}
                    className="flex items-center text-muted-foreground hover:text-foreground"
                    aria-label="Copy"
                  >
                    {copied ? <CheckIcon size={12} /> : <Copy size={12} />}
                  </button>
                }
              />
            </Field>
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            {error && (
              <div className="rounded-[var(--radius)] border border-[hsl(var(--destructive)/0.35)] bg-[hsl(var(--destructive)/0.12)] px-2.5 py-2 text-[11.5px] text-[hsl(0_80%_80%)]">
                {error}
              </div>
            )}
            <Field label="Username">
              <Input
                placeholder="jane.doe"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                autoFocus
              />
            </Field>
            <Field label="Email">
              <Input
                placeholder="jane@example.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </Field>
            <Field label="Role">
              <Select value={role} onValueChange={(v) => setRole(v as UserRow["role"])}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="admin">admin</SelectItem>
                  <SelectItem value="editor">editor</SelectItem>
                  <SelectItem value="viewer">viewer</SelectItem>
                </SelectContent>
              </Select>
            </Field>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}

export function AdminUsersPage() {
  const [users, setUsers] = React.useState<UserRow[]>([]);
  const [state, setState] = React.useState<"default" | "loading" | "empty" | "error">("loading");
  const [filter, setFilter] = React.useState("");
  const [inviteOpen, setInviteOpen] = React.useState(false);
  const [errMsg, setErrMsg] = React.useState<string | null>(null);

  const load = React.useCallback(async () => {
    setState("loading");
    try {
      const list = await api<UserRow[]>("/users");
      setUsers(list ?? []);
      setState(list && list.length > 0 ? "default" : "empty");
    } catch (e) {
      setErrMsg(e instanceof APIError ? e.message : "Failed to load users");
      setState("error");
    }
  }, []);

  React.useEffect(() => {
    void load();
  }, [load]);

  const filtered = filter
    ? users.filter(
        (u) =>
          u.username.toLowerCase().includes(filter.toLowerCase()) ||
          (u.email ?? "").toLowerCase().includes(filter.toLowerCase()),
      )
    : users;

  return (
    <AppShell active="users">
      <div className="flex h-full flex-col">
        <div className="flex items-center gap-2 border-b px-5 py-3.5">
          <div>
            <div className="text-[16px] font-semibold tracking-[-0.2px]">Users</div>
            <div className="mt-0.5 text-[11.5px] text-muted-foreground">
              App accounts that can sign in to Cassidy
            </div>
          </div>
          <div className="flex-1" />
          <Input
            wrapperClassName="w-[220px] bg-panel"
            placeholder="Find user…"
            icon={<Search size={12} strokeWidth={1.8} />}
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          />
          <Button variant="default" size="md" onClick={() => setInviteOpen(true)}>
            <Plus size={12} strokeWidth={2.2} />
            Invite user
          </Button>
        </div>

        <div className="flex items-center gap-1.5 border-b px-5 py-2 text-[11.5px]">
          <Badge variant="info">
            All <span className="mono ml-1">{users.length}</span>
          </Badge>
          <Badge variant="outline">
            Active{" "}
            <span className="mono ml-1">
              {users.filter((u) => u.is_active).length}
            </span>
          </Badge>
          <Badge variant="outline">
            Admins{" "}
            <span className="mono ml-1">
              {users.filter((u) => u.role === "admin").length}
            </span>
          </Badge>
          <Badge variant="outline">
            Inactive{" "}
            <span className="mono ml-1">
              {users.filter((u) => !u.is_active).length}
            </span>
          </Badge>
          <div className="flex-1" />
          <span className="text-muted-foreground">Sort by</span>
          <Select defaultValue="created-newest">
            <SelectTrigger className="h-6 text-[11px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="created-newest">Created · newest</SelectItem>
              <SelectItem value="created-oldest">Created · oldest</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex-1 overflow-auto">
          {state === "error" && (
            <div className="p-16 text-center text-[12px] text-[hsl(0_80%_75%)]">
              {errMsg ?? "Failed to load users."}
            </div>
          )}
          {state === "empty" && (
            <div className="p-16 text-center text-[12px] text-muted-foreground">
              No users yet. Click <strong>Invite user</strong> to add one.
            </div>
          )}
          {(state === "default" || state === "loading") && (
            <table className="w-full border-collapse text-[12px]">
              <thead>
                <tr className="border-b bg-panel text-[11px] text-muted-foreground">
                  <th className="px-5 py-1.5 text-left font-medium">User</th>
                  <th className="px-3 py-1.5 text-left font-medium">Email</th>
                  <th className="px-3 py-1.5 text-left font-medium" style={{ width: 90 }}>
                    Role
                  </th>
                  <th className="px-3 py-1.5 text-left font-medium" style={{ width: 80 }}>
                    Status
                  </th>
                  <th className="px-3 py-1.5 text-left font-medium" style={{ width: 110 }}>
                    Created
                  </th>
                  <th style={{ width: 44 }} />
                </tr>
              </thead>
              <tbody>
                {state === "loading"
                  ? Array.from({ length: 4 }).map((_, i) => (
                      <tr key={i} className="h-9 border-b border-border/50">
                        <td className="px-5 py-1.5">
                          <Skeleton w={120} h={10} />
                        </td>
                        <td className="px-3 py-1.5">
                          <Skeleton w={140} h={10} />
                        </td>
                        <td className="px-3 py-1.5">
                          <Skeleton w={50} h={14} className="rounded-full" />
                        </td>
                        <td className="px-3 py-1.5">
                          <Skeleton w={50} h={10} />
                        </td>
                        <td className="px-3 py-1.5">
                          <Skeleton w={70} h={10} />
                        </td>
                        <td />
                      </tr>
                    ))
                  : filtered.map((u) => (
                      <tr key={u.id} className="h-9 border-b border-border/50">
                        <td className="px-5 py-1.5">
                          <div className="flex items-center gap-2">
                            <Avatar name={u.username} />
                            <span className="mono text-[12px] font-medium">
                              {u.username}
                            </span>
                            {u.must_reset_pw && (
                              <Badge variant="warning">temp pw</Badge>
                            )}
                          </div>
                        </td>
                        <td className="mono px-3 py-1.5 text-muted-foreground">
                          {u.email ?? ""}
                        </td>
                        <td className="px-3 py-1.5">
                          <RoleBadge role={u.role} />
                        </td>
                        <td className="px-3 py-1.5">
                          <StatusDot
                            status={u.is_active ? "green" : "grey"}
                            label={u.is_active ? "Active" : "Disabled"}
                          />
                        </td>
                        <td className="mono px-3 py-1.5 text-muted-foreground">
                          {u.created_at.slice(0, 10)}
                        </td>
                        <td>
                          <Button variant="ghost" size="icon-sm">
                            <MoreHorizontal size={14} strokeWidth={2.4} />
                          </Button>
                        </td>
                      </tr>
                    ))}
              </tbody>
            </table>
          )}
        </div>
        <InviteDialog
          open={inviteOpen}
          onOpenChange={setInviteOpen}
          onCreated={load}
        />
      </div>
    </AppShell>
  );
}

export default AdminUsersPage;
