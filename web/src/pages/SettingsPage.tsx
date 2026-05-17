import { Moon, Sun } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useTheme } from "@/components/ThemeProvider";

export function SettingsPage() {
  const { theme, toggleTheme } = useTheme();

  return (
    <>
      <div className="mb-8">
        <h1 className="text-3xl font-bold tracking-tight">系统设置</h1>
        <p className="mt-2 text-sm text-[var(--color-muted-foreground)]">调整系统偏好设置</p>
      </div>

      <Card className="max-w-md">
        <CardHeader>
          <CardTitle>外观</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {theme === "dark" ? <Moon className="h-5 w-5" /> : <Sun className="h-5 w-5" />}
              <div>
                <p className="text-sm font-medium">
                  {theme === "dark" ? "深色模式" : "浅色模式"}
                </p>
                <p className="text-xs text-[var(--color-muted-foreground)]">
                  切换界面颜色主题
                </p>
              </div>
            </div>
            <Button variant="outline" size="sm" onClick={toggleTheme}>
              {theme === "dark" ? "切换到浅色" : "切换到深色"}
            </Button>
          </div>
        </CardContent>
      </Card>
    </>
  );
}