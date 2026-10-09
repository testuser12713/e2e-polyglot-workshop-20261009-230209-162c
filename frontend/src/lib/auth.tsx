import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { apiFetch, setAuthToken } from "./api";

export interface Employee {
  id: number;
  email: string;
  name: string;
}

interface LoginResponse {
  token: string;
  employee: Employee;
}

interface AuthState {
  token: string | null;
  employee: Employee | null;
}

interface AuthContextValue extends AuthState {
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const STORAGE_KEY = "werkstatt.auth";

const AuthContext = createContext<AuthContextValue | null>(null);

function readStored(): AuthState {
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return { token: null, employee: null };
    }
    const parsed = JSON.parse(raw) as Partial<AuthState>;
    if (typeof parsed.token === "string" && parsed.token) {
      return { token: parsed.token, employee: parsed.employee ?? null };
    }
  } catch {
    // A corrupted or unavailable sessionStorage must never break boot.
  }
  return { token: null, employee: null };
}

function persist(state: AuthState): void {
  try {
    if (state.token) {
      sessionStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } else {
      sessionStorage.removeItem(STORAGE_KEY);
    }
  } catch {
    // Storage may be unavailable (private mode); the in-memory state still works.
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<AuthState>(() => readStored());

  useEffect(() => {
    setAuthToken(state.token);
  }, [state.token]);

  const login = useCallback(async (email: string, password: string) => {
    const response = await apiFetch<LoginResponse>("/api/auth/login", {
      method: "POST",
      body: { email, password },
    });
    const next: AuthState = { token: response.token, employee: response.employee };
    setAuthToken(next.token);
    persist(next);
    setState(next);
  }, []);

  const logout = useCallback(async () => {
    try {
      await apiFetch("/api/auth/logout", { method: "POST" });
    } catch {
      // Logging out locally must always succeed, even if the API is unreachable.
    }
    const next: AuthState = { token: null, employee: null };
    setAuthToken(null);
    persist(next);
    setState(next);
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      ...state,
      isAuthenticated: Boolean(state.token),
      login,
      logout,
    }),
    [state, login, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
