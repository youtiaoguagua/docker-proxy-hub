import { useState } from "react";
import { LogOut, User } from "lucide-react";

import { changeAdmin, formatError } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuth } from "@/components/AuthProvider";

export function AccountPage() {
  const { logout } = useAuth();
  const [currentPassword, setCurrentPassword] = useState("");
  const [username, setUsername] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");
    setMessage("");
    setLoading(true);
    try {
      await changeAdmin(currentPassword, username || undefined, newPassword || undefined);
      if (newPassword) {
        setMessage("密码已修改，请重新登录");
        setTimeout(() => logout(), 1500);
      } else {
        setMessage("信息已更新");
      }
      setCurrentPassword("");
      setUsername("");
      setNewPassword("");
    } catch (err) {
      if (err && typeof err === "object" && "error" in err) {
        const apiError = err as { error: { code: string } };
        if (apiError.error.code === "unauthorized") {
          logout();
          return;
        }
      }
      setError(formatError(err));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">账号管理</h1>
        <p className="mt-2 text-sm text-muted-foreground">修改管理员信息与退出登录</p>
      </div>

      <Card className="max-w-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <User className="h-5 w-5 text-primary" />
            修改管理员信息
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="currentPassword">当前密码</Label>
              <Input id="currentPassword" type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor="username">新用户名（留空不修改）</Label>
              <Input id="username" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="新用户名" />
            </div>
            <div className="space-y-2">
              <Label htmlFor="newPassword">新密码（留空不修改）</Label>
              <Input id="newPassword" type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="新密码" />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            {message && <p className="text-sm text-emerald-600 dark:text-emerald-400">{message}</p>}
            <Button type="submit" disabled={loading}>{loading ? "提交中..." : "保存修改"}</Button>
          </form>
        </CardContent>
      </Card>

      <Card className="mt-6 max-w-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <LogOut className="h-5 w-5 text-destructive" />
            退出登录
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p className="mb-4 text-sm text-muted-foreground">退出当前管理员账号</p>
          <Button variant="destructive" onClick={logout}>退出登录</Button>
        </CardContent>
      </Card>
    </>
  );
}