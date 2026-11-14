"use client";

import { useEffect, useState } from "react";

import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useUITranslation } from "@/i18n/use-ui-translation";
import { api } from "@/lib/api";
import { auth } from "@/lib/auth";

export default function SetupPage() {
  const { ui, liveUI } = useUITranslation();
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(true);
  // 查不到初始化状态时不能默认当成"未初始化"——那样会把初始化表单摆给一个
  // 其实已经设过密码的实例，用户照着填就会覆盖掉原密码。此时关闭入口。
  const [unavailable, setUnavailable] = useState(false);

  useEffect(() => {
    api
      .authStatus()
      .then(({ initialized }) => {
        if (initialized) router.replace("/login");
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : liveUI("screen.56a1f0a8fa5060"));
        setUnavailable(true);
      })
      .finally(() => setChecking(false));
  }, [router, liveUI]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (password !== confirm) {
      setError(liveUI("screen.58807745708ed7"));
      return;
    }
    if (password.length < 8) {
      setError(liveUI("screen.8bef9bbfc97a19"));
      return;
    }
    setLoading(true);
    setError("");
    try {
      const { token } = await api.initPassword(password);
      auth.setToken(token);
      router.replace("/function/tasks");
    } catch (err) {
      setError(err instanceof Error ? err.message : liveUI("screen.613c781999ac85"));
    } finally {
      setLoading(false);
    }
  }

  if (checking) return null;

  return (
    <div className="flex min-h-dvh items-center justify-center bg-muted/40 p-6">
      {/* 与登录页同形态：居中单卡片，不带任何可哈希的图片资源 */}
      <div className="w-full max-w-sm">
        <div className="space-y-10 rounded-2xl border bg-card p-8 shadow-lg lg:p-10">
          <div className="space-y-4 text-center">
            <h2 className="font-medium text-2xl tracking-tight">
              {unavailable ? ui("screen.0a6f0e35da190d") : ui("screen.8c5b300e7fc51d")}
            </h2>
            <p className="mx-auto max-w-xl text-muted-foreground">
              {unavailable
                ? ui("screen.6120fb1f9b2336")
                : ui("screen.011a2568e50e1b")}
            </p>
          </div>
          {unavailable ? (
            <div className="flex flex-col gap-4">
              {error && <p className="text-center text-destructive text-sm">{error}</p>}
              <Button type="button" className="w-full" onClick={() => window.location.reload()}>
                {ui("screen.b8784c8dd5636f")}
              </Button>
            </div>
          ) : (
            <form onSubmit={handleSubmit} className="flex flex-col gap-4">
              <div className="space-y-1.5">
                <Label htmlFor="password">{ui("screen.515e9c7cf7b29c")}</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={ui("screen.d482f263cd9ba2")}
                  autoFocus
                  autoComplete="new-password"
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="confirm">{ui("screen.81b17bc0c362df")}</Label>
                <Input
                  id="confirm"
                  type="password"
                  value={confirm}
                  onChange={(e) => setConfirm(e.target.value)}
                  placeholder={ui("screen.d0c9a0e81079a7")}
                  autoComplete="new-password"
                />
              </div>
              {error && <p className="text-destructive text-sm">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading || !password || !confirm}>
                {loading ? ui("screen.917b1c1f18d027") : ui("screen.516b851f038942")}
              </Button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
