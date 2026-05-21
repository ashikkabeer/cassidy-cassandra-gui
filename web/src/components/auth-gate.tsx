import * as React from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useAuth } from "@/lib/auth-store";
import { Spinner } from "@/components/primitives";

/**
 * AuthGate calls `/auth/me` once on mount, then either renders its children
 * (authed) or redirects to /first-run (first-run flow) / /login (anonymous).
 * The intended URL is preserved across the redirect via location state, so the
 * Login page can navigate back to it after a successful sign-in.
 */
export function AuthGate({ children }: { children: React.ReactNode }) {
  const status = useAuth((s) => s.status);
  const loadMe = useAuth((s) => s.loadMe);
  const loaded = React.useRef(false);
  const location = useLocation();

  React.useEffect(() => {
    if (!loaded.current) {
      loaded.current = true;
      void loadMe();
    }
  }, [loadMe]);

  if (status === "loading") {
    return (
      <div className="flex h-full w-full items-center justify-center text-muted-foreground">
        <Spinner size={18} />
      </div>
    );
  }
  if (status === "first-run") {
    return <Navigate to="/first-run" replace />;
  }
  if (status === "anon") {
    return (
      <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
    );
  }
  return <>{children}</>;
}
