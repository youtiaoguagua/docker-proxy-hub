import { Routes, Route, Navigate } from "react-router-dom";

import { useAuth } from "@/components/AuthProvider";
import { AppShell } from "@/components/AppShell";
import { DashboardPage } from "@/pages/DashboardPage";
import { LoginPage } from "@/pages/LoginPage";
import { MonitoringPage } from "@/pages/MonitoringPage";
import { ProxyConfigPage } from "@/pages/ProxyConfigPage";
import { SettingsPage } from "@/pages/SettingsPage";
import { SetupPage } from "@/pages/SetupPage";
import { UpstreamsPage } from "@/pages/UpstreamsPage";
import { UsageDocsPage } from "@/pages/UsageDocsPage";
import { AccountPage } from "@/pages/AccountPage";

function AuthenticatedLayout() {
  return (
    <AppShell>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/upstreams" element={<UpstreamsPage />} />
        <Route path="/proxy" element={<ProxyConfigPage />} />
        <Route path="/monitoring" element={<MonitoringPage />} />
        <Route path="/settings" element={<SettingsPage />} />
        <Route path="/account" element={<AccountPage />} />
        <Route path="/docs" element={<UsageDocsPage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </AppShell>
  );
}

function App() {
  const { auth, login, setupComplete } = useAuth();

  if (auth.status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <p className="text-muted-foreground">加载中...</p>
      </div>
    );
  }

  if (auth.status === "setup") {
    return <SetupPage onComplete={setupComplete} />;
  }

  if (auth.status === "login") {
    return <LoginPage onLogin={login} />;
  }

  return <AuthenticatedLayout />;
}

export default App;