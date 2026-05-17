import {
  Activity,
  BookOpen,
  Gauge,
  Globe2,
  Moon,
  Settings,
  Sun,
  User,
} from "lucide-react";
import { Link, useLocation } from "react-router-dom";
import type { ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useTheme } from "@/components/ThemeProvider";

type PageKey = "dashboard" | "upstreams" | "monitoring" | "settings" | "account" | "docs";

type NavItem = {
  key: PageKey;
  label: string;
  icon: typeof Gauge;
  path: string;
};

const navItems: NavItem[] = [
  { key: "dashboard", label: "仪表盘", icon: Gauge, path: "/" },
  { key: "upstreams", label: "上游管理", icon: Globe2, path: "/upstreams" },
  { key: "monitoring", label: "监控日志", icon: Activity, path: "/monitoring" },
  { key: "settings", label: "系统设置", icon: Settings, path: "/settings" },
  { key: "account", label: "账号管理", icon: User, path: "/account" },
  { key: "docs", label: "使用文档", icon: BookOpen, path: "/docs" },
];

type AppShellProps = {
  children: ReactNode;
};

export type { PageKey };

export function AppShell({ children }: AppShellProps) {
  const location = useLocation();
  const { theme, toggleTheme } = useTheme();

  const currentPath = location.pathname;
  const activeKey = navItems.find((item) => item.path === currentPath)?.key ?? (currentPath === "/account" ? "account" : "dashboard");

  return (
    <div className="min-h-screen bg-background text-foreground">
      <aside className="fixed inset-y-0 left-0 z-30 hidden w-72 border-r border-border bg-card lg:block">
        <div className="flex h-full flex-col p-4">
          <div className="flex items-center gap-3 border-b border-border pb-5">
            <div className="flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-lg">
              <Globe2 className="h-6 w-6" />
            </div>
            <div>
              <div className="text-lg font-semibold">DockerProxyHub</div>
              <span className="rounded-md bg-blue-50 px-2 py-0.5 text-xs font-medium text-blue-600 dark:bg-cyan-500/10 dark:text-cyan-400">v0.1.0</span>
            </div>
          </div>

          <nav className="mt-5 flex-1 space-y-1">
            <div className="mb-3 px-2 text-xs font-medium text-muted-foreground">控制台</div>
            {navItems.map((item) => (
              <Link
                key={item.key}
                to={item.path}
                className={cn(
                  "flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-colors",
                  activeKey === item.key
                    ? "bg-accent text-accent-foreground"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                )}
              >
                <item.icon className="h-4 w-4" />
                {item.label}
              </Link>
            ))}
          </nav>

          <div className="border-t border-border pt-4">
            <div className="flex items-center justify-between">
              <div className="inline-flex items-center gap-2 rounded-lg bg-emerald-50 px-3 py-2 text-sm font-medium text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-400">
                <span className="h-2 w-2 rounded-full bg-emerald-500" />
                在线
              </div>
              <Button variant="ghost" size="icon" onClick={toggleTheme} className="text-muted-foreground hover:text-foreground">
                {theme === "dark" ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
              </Button>
            </div>
          </div>
        </div>
      </aside>

      <main className="lg:pl-72">
        <div className="p-6 lg:p-8">{children}</div>
      </main>
    </div>
  );
}