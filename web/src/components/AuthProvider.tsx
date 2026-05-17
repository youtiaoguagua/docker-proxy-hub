import { createContext, useContext, useEffect, useState } from "react";

import { getMe, getSetupStatus, logout as logoutApi } from "@/api/client";

type AuthState =
  | { status: "loading" }
  | { status: "setup" }
  | { status: "login" }
  | { status: "authenticated"; admin: unknown };

type AuthContextValue = {
  auth: AuthState;
  login: () => Promise<void>;
  logout: () => Promise<void>;
  setupComplete: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [auth, setAuth] = useState<AuthState>({ status: "loading" });

  useEffect(() => {
    getSetupStatus()
      .then((status) => {
        if (status.setupRequired) {
          setAuth({ status: "setup" });
          return;
        }
        return getMe();
      })
      .then((result) => {
        if (result) {
          setAuth({ status: "authenticated", admin: result.admin });
        }
      })
      .catch(() => {
        setAuth({ status: "login" });
      });
  }, []);

  const login = async () => {
    const result = await getMe();
    setAuth({ status: "authenticated", admin: result.admin });
  };

  const logout = async () => {
    try {
      await logoutApi();
    } catch {
      // cookie cleared anyway
    }
    setAuth({ status: "login" });
  };

  const setupComplete = async () => {
    const result = await getMe();
    setAuth({ status: "authenticated", admin: result.admin });
  };

  return (
    <AuthContext.Provider value={{ auth, login, logout, setupComplete }}>
      {children}
    </AuthContext.Provider>
  );
}