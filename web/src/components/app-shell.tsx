import * as React from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  Database,
  Code2,
  History,
  Users,
  Search,
  ChevronDown,
  Lock,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Separator } from "@/components/ui/separator";
import { Kbd } from "@/components/primitives";
import { useActiveConnectionDetail } from "@/lib/active-connection-data";
import { useAuth } from "@/lib/auth-store";
import { listConnections } from "@/lib/connections";

// Derive avatar initials (up to 2 chars) from a username like "ashik" → "AS".
function initialsOf(name: string): string {
  const parts = name.trim().split(/[\s._-]+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[1][0]).toUpperCase();
}

function Logo({ size = 14 }: { size?: number }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <ellipse cx="12" cy="5" rx="9" ry="3" />
      <path d="M3 5v6c0 1.7 4 3 9 3s9-1.3 9-3V5M3 11v6c0 1.7 4 3 9 3s9-1.3 9-3v-6" />
    </svg>
  );
}

type NavId = "workspace" | "connections" | "history" | "users";

interface NavItemProps {
  icon: React.ReactNode;
  label: string;
  to: string;
  active?: boolean;
  badge?: React.ReactNode;
  collapsed?: boolean;
}

function NavItem({ icon, label, to, active, badge, collapsed }: NavItemProps) {
  return (
    <Link
      to={to}
      title={collapsed ? label : undefined}
      className={cn(
        "flex h-7 items-center gap-2.5 rounded-[var(--radius)] text-[12.5px] no-underline",
        collapsed ? "justify-center px-0" : "px-2.5",
        active
          ? "bg-accent font-medium text-foreground"
          : "text-muted-foreground hover:text-foreground hover:bg-accent/60",
      )}
    >
      <span className="flex">{icon}</span>
      {!collapsed && <span className="flex-1">{label}</span>}
      {!collapsed && badge != null && <Badge variant="outline">{badge}</Badge>}
    </Link>
  );
}

export interface SidebarProps {
  active?: NavId;
  collapsed?: boolean;
}

export function Sidebar({ active = "workspace", collapsed = false }: SidebarProps) {
  const user = useAuth((s) => s.user);
  const name = user?.username ?? "—";
  const role = user?.role ? user.role[0].toUpperCase() + user.role.slice(1) : "";

  // Live count of the user's saved connections for the sidebar badge. Refetched
  // when the Sidebar mounts (i.e. on each navigation between pages).
  const [connCount, setConnCount] = React.useState<number | null>(null);
  React.useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const list = await listConnections();
        if (!cancelled) setConnCount((list ?? []).length);
      } catch {
        /* leave badge hidden if it fails */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <aside
      className={cn(
        "flex h-full shrink-0 flex-col border-r bg-background",
      )}
      style={{ width: collapsed ? 48 : 200 }}
    >
      <div
        className={cn(
          "flex h-11 items-center gap-2 border-b",
          collapsed ? "justify-center px-0" : "px-3.5",
        )}
      >
        <div className="flex h-[22px] w-[22px] items-center justify-center rounded-[5px] bg-foreground text-background">
          <Logo size={13} />
        </div>
        {!collapsed && (
          <div className="flex flex-col leading-[1.1]">
            <span className="text-[12.5px] font-semibold tracking-[-0.2px]">
              Cassidy
            </span>
            <span className="text-[9.5px] uppercase tracking-[0.4px] text-muted-foreground">
              Cassandra GUI
            </span>
          </div>
        )}
      </div>

      <div className="flex flex-1 flex-col gap-px p-2">
        {!collapsed && (
          <div className="px-2.5 pb-0.5 pt-1.5 text-[10px] uppercase tracking-[0.4px] text-muted-foreground">
            Navigate
          </div>
        )}
        <NavItem
          icon={<Code2 size={14} strokeWidth={1.5} />}
          label="Workspace"
          to="/"
          active={active === "workspace"}
          collapsed={collapsed}
        />
        <NavItem
          icon={<Database size={14} strokeWidth={1.5} />}
          label="Connections"
          to="/connections"
          active={active === "connections"}
          badge={connCount ? String(connCount) : undefined}
          collapsed={collapsed}
        />
        <NavItem
          icon={<History size={14} strokeWidth={1.5} />}
          label="Query history"
          to="/history"
          active={active === "history"}
          collapsed={collapsed}
        />

        {!collapsed && (
          <div className="px-2.5 pb-0.5 pt-3 text-[10px] uppercase tracking-[0.4px] text-muted-foreground">
            Admin
          </div>
        )}
        <NavItem
          icon={<Users size={14} strokeWidth={1.5} />}
          label="Users"
          to="/admin/users"
          active={active === "users"}
          collapsed={collapsed}
        />
      </div>

      <div className="border-t p-2">
        <Link
          to="/profile"
          className="flex items-center gap-2 rounded-[var(--radius)] px-1.5 py-1.5 no-underline text-foreground hover:bg-accent/60"
        >
          <div className="flex h-[22px] w-[22px] items-center justify-center rounded-full bg-[hsl(240_4%_18%)] text-[10.5px] font-semibold text-foreground">
            {initialsOf(name)}
          </div>
          {!collapsed && (
            <div className="min-w-0 flex-1">
              <div className="truncate text-[12px] font-medium">{name}</div>
              <div className="text-[10.5px] text-muted-foreground">{role}</div>
            </div>
          )}
        </Link>
      </div>
    </aside>
  );
}

export interface ConnPillProps {
  // Optional overrides — by default the pill renders whatever's in the
  // useActiveConnection store. Pages that want a forced label (e.g. demo
  // mode) can override.
  name?: string;
  status?: "green" | "red" | "amber" | "grey";
  readOnly?: boolean;
}

export function ConnPill({ name, status, readOnly }: ConnPillProps) {
  const { conn, status: loadStatus } = useActiveConnectionDetail();
  const navigate = useNavigate();

  const displayName = name ?? conn?.name ?? (loadStatus === "loading" ? "Loading…" : "No connection selected");
  const displayKs = conn?.default_keyspace ? ` · ks: ${conn.default_keyspace}` : "";
  const dotStatus =
    status ??
    (loadStatus === "ready" ? "green"
      : loadStatus === "loading" ? "amber"
      : loadStatus === "error" ? "red"
      : "grey");
  const isReadOnly = readOnly ?? conn?.read_only ?? false;

  return (
    <button
      type="button"
      onClick={() => navigate("/connections")}
      title="Switch connection"
      className="inline-flex h-[26px] max-w-[280px] cursor-pointer items-center gap-2 rounded-[var(--radius)] border bg-panel pl-2.5 pr-2 hover:border-[hsl(var(--border-strong))]"
    >
      <span className={cn("dot", `dot-${dotStatus}`)} />
      <span className="mono truncate text-[11.5px]">
        {displayName}
        {displayKs}
      </span>
      {isReadOnly && (
        <Badge variant="warning" icon={<Lock size={9} strokeWidth={2} />}>
          Read-only
        </Badge>
      )}
      <ChevronDown size={12} strokeWidth={1.6} className="text-muted-foreground" />
    </button>
  );
}

export interface TopBarProps {
  right?: React.ReactNode;
}

export function TopBar({ right }: TopBarProps) {
  const navigate = useNavigate();
  const username = useAuth((s) => s.user?.username ?? "—");
  return (
    <header className="flex h-11 shrink-0 items-center gap-2 border-b bg-background px-3">
      <ConnPill />
      <div className="flex-1" />
      <div className="flex items-center gap-1.5">
        <Input
          wrapperClassName="w-[220px] h-[26px] bg-panel"
          placeholder="Search schema, queries…"
          icon={<Search size={12} strokeWidth={1.8} />}
          suffix={<Kbd>⌘K</Kbd>}
        />
        <Button
          variant="ghost"
          size="icon"
          aria-label="Switch connection"
          onClick={() => navigate("/connections")}
        >
          <Database size={14} strokeWidth={1.6} />
        </Button>
        {right}
        <Separator orientation="vertical" className="mx-1 h-[18px]" />
        <button
          type="button"
          className="flex cursor-pointer items-center gap-1.5 px-1"
          onClick={() => navigate("/profile")}
        >
          <div className="flex h-[22px] w-[22px] items-center justify-center rounded-full bg-[hsl(240_4%_18%)] text-[10.5px] font-semibold text-foreground">
            {initialsOf(username)}
          </div>
          <ChevronDown
            size={12}
            strokeWidth={1.6}
            className="text-muted-foreground"
          />
        </button>
      </div>
    </header>
  );
}

export interface AppShellProps {
  active?: NavId;
  collapsed?: boolean;
  topRight?: React.ReactNode;
  children?: React.ReactNode;
}

export function AppShell({
  active,
  collapsed,
  topRight,
  children,
}: AppShellProps) {
  // Read active route automatically if not provided
  const location = useLocation();
  const auto: NavId = location.pathname.startsWith("/connections")
    ? "connections"
    : location.pathname.startsWith("/history")
      ? "history"
      : location.pathname.startsWith("/admin/users")
        ? "users"
        : "workspace";

  return (
    <div className="flex h-full w-full bg-background">
      <Sidebar active={active ?? auto} collapsed={collapsed} />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar right={topRight} />
        <div className="min-h-0 flex-1 overflow-hidden">{children}</div>
      </div>
    </div>
  );
}
