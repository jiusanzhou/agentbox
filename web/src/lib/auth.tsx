"use client";

import {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  type ReactNode,
} from "react";
import { useRouter, usePathname } from "next/navigation";

interface User {
  id: string;
  name: string;
  email: string;
}

interface AuthContextType {
  user: User | null;
  token: string | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  register: (name: string, email: string, password: string) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | null>(null);

const PUBLIC_PATHS = ["/", "/login", "/register"];

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const router = useRouter();
  const pathname = usePathname();

  const saveAuth = useCallback((t: string, u: User) => {
    localStorage.setItem("abox_token", t);
    document.cookie = `abox_token=${t}; path=/; max-age=${60 * 60 * 24 * 7}; SameSite=Lax`;
    setToken(t);
    setUser(u);
  }, []);

  const clearAuth = useCallback(() => {
    localStorage.removeItem("abox_token");
    document.cookie = "abox_token=; path=/; max-age=0";
    setToken(null);
    setUser(null);
  }, []);

  // Check for existing token on mount
  useEffect(() => {
    const stored = localStorage.getItem("abox_token");
    if (!stored) {
      setLoading(false);
      return;
    }

    fetch("/api/auth/me", {
      headers: { Authorization: `Bearer ${stored}` },
    })
      .then((r) => {
        if (!r.ok) throw new Error("Invalid token");
        return r.json();
      })
      .then((data) => {
        setToken(stored);
        setUser(data.user || data);
      })
      .catch(() => {
        clearAuth();
      })
      .finally(() => setLoading(false));
  }, [clearAuth]);

  // Redirect unauthenticated users away from protected routes
  useEffect(() => {
    if (loading) return;
    if (!user && !PUBLIC_PATHS.includes(pathname)) {
      router.replace("/login");
    }
  }, [user, loading, pathname, router]);

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email, password }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.error || err.message || "Login failed");
      }
      const data = await res.json();
      saveAuth(data.token, data.user);
      router.replace("/dashboard");
    },
    [saveAuth, router]
  );

  const register = useCallback(
    async (name: string, email: string, password: string) => {
      const res = await fetch("/api/auth/register", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, email, password }),
      });
      if (!res.ok) {
        const err = await res.json().catch(() => ({}));
        throw new Error(err.error || err.message || "Registration failed");
      }
      const data = await res.json();
      saveAuth(data.token, data.user);
      router.replace("/dashboard");
    },
    [saveAuth, router]
  );

  const logout = useCallback(() => {
    clearAuth();
    router.replace("/login");
  }, [clearAuth, router]);

  return (
    <AuthContext.Provider value={{ user, token, loading, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
