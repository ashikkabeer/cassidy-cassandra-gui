// Tiny Zustand store for the *active* connection — the one the topbar pill
// displays and the workspace is talking to. Persisted to localStorage so it
// survives reloads.
import { create } from "zustand";
import { persist } from "zustand/middleware";

interface ActiveConnectionState {
  activeId: string | null;
  setActive: (id: string | null) => void;
}

export const useActiveConnection = create<ActiveConnectionState>()(
  persist(
    (set) => ({
      activeId: null,
      setActive: (id) => set({ activeId: id }),
    }),
    { name: "cassidy.activeConnection" },
  ),
);
