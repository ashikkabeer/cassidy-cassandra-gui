import { create } from "zustand";
import { api, APIError } from "@/lib/api";

export interface User {
  id: string;
  username: string;
  email?: string;
  role: "admin" | "editor" | "viewer";
  is_active: boolean;
  must_reset_pw: boolean;
  created_at: string;
}

export type AuthStatus = "loading" | "anon" | "first-run" | "authed";

interface AuthState {
  status: AuthStatus;
  user: User | null;
  error: string | null;
  loadMe: () => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  setup: (token: string, username: string, email: string, password: string) => Promise<void>;
}

export const useAuth = create<AuthState>((set) => ({
  status: "loading",
  user: null,
  error: null,

  async loadMe() {
    set({ status: "loading", error: null });
    try {
      const { user } = await api<{ user: User }>("/auth/me");
      set({ status: "authed", user });
    } catch (e) {
      if (e instanceof APIError && e.status === 401) {
        set({ status: e.firstRun ? "first-run" : "anon", user: null });
        return;
      }
      set({ status: "anon", error: e instanceof Error ? e.message : String(e) });
    }
  },

  async login(username, password) {
    set({ error: null });
    const { user } = await api<{ user: User }>("/auth/login", {
      method: "POST",
      body: { username, password },
    });
    set({ status: "authed", user });
  },

  async logout() {
    try {
      await api("/auth/logout", { method: "POST" });
    } catch {
      /* ignore */
    }
    set({ status: "anon", user: null });
  },

  async setup(token, username, email, password) {
    set({ error: null });
    const { user } = await api<{ user: User }>("/auth/setup", {
      method: "POST",
      body: { token, username, email, password },
    });
    set({ status: "authed", user });
  },
}));
