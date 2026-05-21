import * as React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { AlertTriangle, Eye, Lock, User, Info, KeyRound } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Field } from "@/components/ui/field";
import { Checkbox } from "@/components/ui/checkbox";
import { Separator } from "@/components/ui/separator";
import { Badge } from "@/components/ui/badge";
import { Spinner } from "@/components/primitives";
import { useAuth } from "@/lib/auth-store";
import { APIError } from "@/lib/api";

function Logo({ size = 18 }: { size?: number }) {
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

function LoginShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative flex h-full w-full items-center justify-center overflow-hidden bg-background">
      <div
        className="pointer-events-none absolute inset-0"
        style={{
          background:
            "radial-gradient(ellipse at top, hsl(240 6% 8%) 0%, hsl(var(--background)) 60%)",
        }}
      />
      <div
        className="pointer-events-none absolute inset-0 opacity-40"
        style={{
          backgroundImage:
            "linear-gradient(hsl(240 4% 8%) 1px, transparent 1px), linear-gradient(90deg, hsl(240 4% 8%) 1px, transparent 1px)",
          backgroundSize: "40px 40px",
          maskImage:
            "radial-gradient(ellipse at center, black 0%, transparent 70%)",
          WebkitMaskImage:
            "radial-gradient(ellipse at center, black 0%, transparent 70%)",
        }}
      />
      <div className="relative z-10">{children}</div>
    </div>
  );
}

function Brand() {
  return (
    <div className="mb-7 flex items-center justify-center gap-2.5">
      <div className="flex h-8 w-8 items-center justify-center rounded-[7px] bg-foreground text-background">
        <Logo size={18} />
      </div>
      <div className="leading-[1.1]">
        <div className="text-[15px] font-semibold tracking-[-0.2px]">Cassidy</div>
        <div className="text-[10px] uppercase tracking-[0.6px] text-muted-foreground">
          Cassandra GUI
        </div>
      </div>
    </div>
  );
}

export function LoginPage() {
  const login = useAuth((s) => s.login);
  const navigate = useNavigate();
  const location = useLocation();
  const intended = (location.state as { from?: string } | null)?.from ?? "/";

  const [username, setUsername] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username || !password) return;
    setLoading(true);
    setError(null);
    try {
      await login(username, password);
      navigate(intended, { replace: true });
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.code === "rate_limited" ? "Too many attempts — try again shortly." : "Invalid username or password.");
      } else {
        setError("Sign-in failed. Please try again.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <LoginShell>
      <Brand />
      <Card className="w-[340px] p-5">
        <div className="mb-4">
          <div className="text-[16px] font-semibold tracking-[-0.2px]">Sign in</div>
          <div className="mt-0.5 text-[12px] text-muted-foreground">
            Use your Cassidy admin credentials.
          </div>
        </div>

        {error && (
          <div className="mb-3.5 flex items-start gap-2 rounded-[var(--radius)] border border-[hsl(var(--destructive)/0.35)] bg-[hsl(var(--destructive)/0.12)] px-2.5 py-2 text-[11.5px] text-[hsl(0_80%_80%)]">
            <AlertTriangle size={13} strokeWidth={1.8} className="mt-px" />
            <div>
              <div className="font-medium">{error}</div>
            </div>
          </div>
        )}

        <form onSubmit={submit} className="flex flex-col gap-3">
          <Field label="Username">
            <Input
              icon={<User size={12} strokeWidth={1.6} />}
              placeholder="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              error={!!error}
              autoFocus
            />
          </Field>
          <Field label="Password">
            <Input
              icon={<Lock size={12} strokeWidth={1.6} />}
              suffix={<Eye size={12} strokeWidth={1.6} />}
              type="password"
              placeholder="••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={!!error}
            />
          </Field>
          <div className="mt-0.5 flex items-center justify-between">
            <label className="inline-flex items-center gap-2 text-[12px]">
              <Checkbox defaultChecked />
              Remember this browser
            </label>
            <a className="cursor-pointer text-[11px] text-muted-foreground underline underline-offset-[3px]">
              Forgot password?
            </a>
          </div>

          <Button
            type="submit"
            variant="default"
            size="lg"
            disabled={loading || !username || !password}
            className="mt-1 w-full"
          >
            {loading ? (
              <>
                <Spinner size={12} /> Signing in…
              </>
            ) : (
              "Sign in"
            )}
          </Button>
        </form>
      </Card>

      <div className="mt-4 text-center text-[11px] text-muted-foreground">
        Cassidy <span className="mono">v0.0.1</span> ·{" "}
        <a className="underline underline-offset-[3px]">Docs</a>
      </div>
    </LoginShell>
  );
}

export function FirstRunPage() {
  const setup = useAuth((s) => s.setup);
  const navigate = useNavigate();

  const [token, setToken] = React.useState("");
  const [username, setUsername] = React.useState("admin");
  const [email, setEmail] = React.useState("");
  const [pw1, setPw1] = React.useState("");
  const [pw2, setPw2] = React.useState("");
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (pw1 !== pw2) {
      setError("Passwords don't match.");
      return;
    }
    if (pw1.length < 12) {
      setError("Password must be at least 12 characters.");
      return;
    }
    if (!token || !username) return;
    setLoading(true);
    setError(null);
    try {
      await setup(token, username, email, pw1);
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof APIError) {
        setError(err.message);
      } else {
        setError("Setup failed. Check the server logs.");
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <LoginShell>
      <Brand />
      <Card className="w-[360px] p-5">
        <Badge variant="info" icon={<Info size={10} strokeWidth={2.2} />}>
          First-run setup
        </Badge>
        <div className="mb-4 mt-2.5">
          <div className="text-[16px] font-semibold tracking-[-0.2px]">
            Set up admin account
          </div>
          <div className="mt-0.5 text-[12px] text-muted-foreground">
            Paste the one-time setup token from your server logs to claim the admin
            account.
          </div>
        </div>

        {error && (
          <div className="mb-3 rounded-[var(--radius)] border border-[hsl(var(--destructive)/0.35)] bg-[hsl(var(--destructive)/0.12)] px-2.5 py-2 text-[11.5px] text-[hsl(0_80%_80%)]">
            {error}
          </div>
        )}

        <form onSubmit={submit} className="flex flex-col gap-3">
          <Field label="Setup token" hint="from CASSIDY_SETUP_TOKEN">
            <Input
              icon={<KeyRound size={12} strokeWidth={1.6} />}
              placeholder="cs_setup_•••••••••••••••••"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              autoFocus
            />
          </Field>
          <Separator className="my-0.5" />
          <div className="grid grid-cols-2 gap-2.5">
            <Field label="Username">
              <Input value={username} onChange={(e) => setUsername(e.target.value)} />
            </Field>
            <Field label="Email">
              <Input
                placeholder="you@company.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
              />
            </Field>
          </div>
          <Field label="Password" help="At least 12 characters.">
            <Input
              type="password"
              placeholder="••••••••••••"
              value={pw1}
              onChange={(e) => setPw1(e.target.value)}
            />
          </Field>
          <Field label="Confirm password">
            <Input
              type="password"
              placeholder="••••••••••••"
              value={pw2}
              onChange={(e) => setPw2(e.target.value)}
            />
          </Field>
          <Button
            type="submit"
            variant="default"
            size="lg"
            disabled={loading || !token || !username || !pw1 || !pw2}
            className="mt-1 w-full"
          >
            {loading ? (
              <>
                <Spinner size={12} /> Creating admin…
              </>
            ) : (
              "Create admin account"
            )}
          </Button>
        </form>
      </Card>
    </LoginShell>
  );
}

export default LoginPage;
