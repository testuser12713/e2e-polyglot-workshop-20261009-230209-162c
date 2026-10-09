import { useEffect } from "react";
import { Navigate, Outlet, useLocation } from "react-router-dom";
import { useAuth } from "../lib/auth";

type UnauthorizedHandler = () => void | Promise<void>;

const GUARD_MARK = "__kfzWerkstattFetchGuard";

let unauthorizedHandler: UnauthorizedHandler | null = null;
let handlingUnauthorized = false;

function installFetchGuard(): void {
  const current = globalThis.fetch;
  if (typeof current !== "function") {
    return;
  }
  if ((current as unknown as Record<string, unknown>)[GUARD_MARK]) {
    return;
  }
  const guarded = ((...args: Parameters<typeof fetch>) =>
    current(...args).then((response) => {
      if (response.status === 401 && unauthorizedHandler && !handlingUnauthorized) {
        handlingUnauthorized = true;
        void Promise.resolve(unauthorizedHandler()).finally(() => {
          handlingUnauthorized = false;
        });
      }
      return response;
    })) as typeof fetch;
  (guarded as unknown as Record<string, unknown>)[GUARD_MARK] = true;
  globalThis.fetch = guarded;
}

export default function RequireAuth() {
  const { isAuthenticated, logout } = useAuth();
  const location = useLocation();

  unauthorizedHandler = isAuthenticated ? () => logout() : null;
  if (isAuthenticated) {
    installFetchGuard();
  }

  useEffect(() => {
    unauthorizedHandler = isAuthenticated ? () => logout() : null;
    return () => {
      unauthorizedHandler = null;
    };
  }, [isAuthenticated, logout]);

  if (!isAuthenticated) {
    return <Navigate to="/werkstatt/anmelden" replace state={{ from: location.pathname }} />;
  }

  return <Outlet />;
}
