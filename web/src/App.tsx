import { Routes, Route, Link } from "react-router-dom";
import { LoginPage, FirstRunPage } from "@/pages/login";
import { ConnectionsPage } from "@/pages/connections";
import { WorkspacePage } from "@/pages/workspace";
import { AdminUsersPage } from "@/pages/admin-users";
import { ProfilePage } from "@/pages/profile";
import { HistoryPage } from "@/pages/history";
import { AuthGate } from "@/components/auth-gate";
import { Card } from "@/components/ui/card";

function WorkspaceRoute() {
  return <WorkspacePage />;
}

function Home() {
  const screens: { to: string; title: string; sub: string }[] = [
    { to: "/login", title: "Login", sub: "real auth wired" },
    { to: "/connections", title: "Connections", sub: "default / loading / empty / error · modal" },
    { to: "/", title: "Workspace", sub: "query · data · all states" },
    { to: "/history", title: "Query history", sub: "all queries · re-open in workspace" },
    { to: "/admin/users", title: "Admin · Users", sub: "live user list + invite" },
    { to: "/profile", title: "Profile", sub: "change password · sessions" },
  ];
  return (
    <div className="flex h-full items-center justify-center bg-background p-8">
      <div className="w-full max-w-3xl">
        <div className="mb-5">
          <div className="text-[18px] font-semibold tracking-[-0.2px]">
            Cassidy — Design preview
          </div>
          <div className="mt-1 text-[12px] text-muted-foreground">
            Every screen from the design handoff, wired to the live backend.
          </div>
        </div>
        <div className="grid grid-cols-2 gap-3">
          {screens.map((s) => (
            <Link key={s.to + s.title} to={s.to} className="no-underline">
              <Card className="cursor-pointer p-3.5 hover:border-[hsl(var(--border-strong))]">
                <div className="text-[13px] font-semibold">{s.title}</div>
                <div className="mt-0.5 text-[11.5px] text-muted-foreground">{s.sub}</div>
              </Card>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}

export default function App() {
  return (
    <Routes>
      {/* Public routes */}
      <Route path="/login" element={<LoginPage />} />
      <Route path="/first-run" element={<FirstRunPage />} />

      {/* Authenticated routes */}
      <Route
        path="/"
        element={
          <AuthGate>
            <WorkspaceRoute />
          </AuthGate>
        }
      />
      <Route
        path="/workspace"
        element={
          <AuthGate>
            <WorkspaceRoute />
          </AuthGate>
        }
      />
      <Route
        path="/connections"
        element={
          <AuthGate>
            <ConnectionsPage />
          </AuthGate>
        }
      />
      <Route
        path="/history"
        element={
          <AuthGate>
            <HistoryPage />
          </AuthGate>
        }
      />
      <Route
        path="/admin/users"
        element={
          <AuthGate>
            <AdminUsersPage />
          </AuthGate>
        }
      />
      <Route
        path="/profile"
        element={
          <AuthGate>
            <ProfilePage />
          </AuthGate>
        }
      />
      <Route
        path="/home"
        element={
          <AuthGate>
            <Home />
          </AuthGate>
        }
      />
      <Route
        path="*"
        element={
          <AuthGate>
            <Home />
          </AuthGate>
        }
      />
    </Routes>
  );
}
