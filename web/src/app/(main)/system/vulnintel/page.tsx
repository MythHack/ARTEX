"use client";

import * as React from "react";

import {
  ActivityIcon,
  DatabaseIcon,
  FolderGitIcon,
  Loader2Icon,
  PlugZapIcon,
  RefreshCwIcon,
  SaveIcon,
  ShieldAlertIcon,
  TriangleAlertIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { useUITranslation } from "@/i18n/use-ui-translation";
import { api } from "@/lib/api";
import type { VulnIntelStatus } from "@/lib/types";

const SOURCE_LABEL: Record<string, string> = {
  nvd: "screen.34e525f8abe74b",
  kev: "screen.02d11ba93f9ba8",
  poc: "screen.a53514afe2511b",
};

type ProbeRow = {
  source: string;
  target: string;
  ok: boolean;
  status: number;
  latency_ms: number;
  error?: string;
  hint?: string;
};

function fmtTime(v?: string): string {
  if (!v) return "—";
  const d = new Date(v);
  if (Number.isNaN(d.getTime())) return v;
  return d.toLocaleString();
}

// 系统 → 漏洞情报:同步配置 + 水位 + 故障诊断 + 连通性自检 + 手动同步。
// 原先这页只读,失败原因藏在 badge 的 title 里,也没有任何配置入口——
// 现在配置、诊断、修复动作都在同一页。
export default function VulnintelPage() {
  const { ui, liveUI } = useUITranslation();
  const [status, setStatus] = React.useState<VulnIntelStatus | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState("");
  const [saving, setSaving] = React.useState(false);
  const [syncing, setSyncing] = React.useState(false);
  const [probing, setProbing] = React.useState(false);
  const [probeRows, setProbeRows] = React.useState<ProbeRow[]>([]);

  // 配置草稿：凭据字段留空 = 保持不变（避免清空输入框就丢 key）。
  const [nvdKey, setNvdKey] = React.useState("");
  const [ghToken, setGhToken] = React.useState("");
  const [proxy, setProxy] = React.useState("");
  const [pocEnabled, setPocEnabled] = React.useState(false);
  const [backfillDays, setBackfillDays] = React.useState(0);
  const [maxRuntimeMin, setMaxRuntimeMin] = React.useState(30);

  const load = React.useCallback(() => {
    setLoading(true);
    api
      .vulnintelStatus()
      .then((s) => {
        setStatus(s);
        setError("");
        const c = s.config;
        if (c) {
          setNvdKey("");
          setGhToken("");
          setProxy(c.proxy ?? "");
          setPocEnabled(!!c.poc_enabled);
          setBackfillDays(c.backfill_days ?? 0);
          setMaxRuntimeMin(c.max_runtime_minutes ?? 30);
        }
      })
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, []);

  React.useEffect(load, [load]);

  const saveConfig = async () => {
    setSaving(true);
    try {
      await api.vulnintelSaveConfig({
        enabled: status?.enabled,
        nvd_api_key: nvdKey.trim() || undefined,
        github_token: ghToken.trim() || undefined,
        proxy: proxy.trim(),
        poc_enabled: pocEnabled,
        backfill_days: backfillDays,
        max_runtime_minutes: maxRuntimeMin,
      });
      toast.success(liveUI("screen.9ce730f24c1a26"));
      load();
    } catch (e) {
      toast.error(liveUI("screen.9f94c6b3fdb1d4", { v0: String(e) }));
    } finally {
      setSaving(false);
    }
  };

  const syncNow = async () => {
    setSyncing(true);
    try {
      await api.vulnintelSyncNow();
      toast.success(liveUI("screen.4ff204e5859222"));
    } catch (e) {
      toast.error(liveUI("screen.b3c81a29eec60c", { v0: String(e) }));
    } finally {
      setSyncing(false);
    }
  };

  const probe = async () => {
    setProbing(true);
    try {
      const res = await api.vulnintelProbe();
      setProbeRows(res.results ?? []);
      const bad = (res.results ?? []).filter((r) => !r.ok).length;
      if (bad === 0) toast.success(liveUI("screen.d1e8ccb660f059"));
      else toast.warning(liveUI("screen.9b8ebca77e0b8e", { v0: bad }));
    } catch (e) {
      toast.error(liveUI("screen.1f46bcdea827d1", { v0: String(e) }));
    } finally {
      setProbing(false);
    }
  };

  const toggleEnabled = async (on: boolean) => {
    try {
      await api.vulnintelSaveConfig({ enabled: on });
      toast.success(on ? liveUI("screen.7c424ed2928517") : liveUI("screen.c97ace8bc6be83"));
      load();
    } catch (e) {
      toast.error(liveUI("screen.72293f9e65cbd4", { v0: String(e) }));
    }
  };

  const cfg = status?.config;

  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <ShieldAlertIcon className="size-5" />
          <h1 className="text-lg font-semibold">{ui("screen.152ea9270e8d07")}</h1>
          {status && (
            <Badge variant={status.enabled ? "default" : "secondary"}>
              {status.enabled ? ui("screen.a8936de6e2a95e") : ui("screen.81aebbd944969d")}
            </Badge>
          )}
          {status?.sync_running && (
            <Badge variant="outline">
              <Loader2Icon className="mr-1 size-3 animate-spin" />
              {ui("screen.1bdda0ff939d66")}
            </Badge>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={probe} disabled={probing}>
            <PlugZapIcon className={"mr-1 size-4" + (probing ? " animate-pulse" : "")} />
            {ui("screen.d5061c58ddc3bb")}
          </Button>
          <Button variant="outline" size="sm" onClick={syncNow} disabled={syncing}>
            <RefreshCwIcon className={"mr-1 size-4" + (syncing ? " animate-spin" : "")} />
            {ui("screen.36e7a43a1c674e")}
          </Button>
          <Button variant="outline" size="sm" onClick={load} disabled={loading}>
            <RefreshCwIcon className={"mr-1 size-4" + (loading ? " animate-spin" : "")} />
            {ui("screen.aee88743413144")}
          </Button>
        </div>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {status && (
        <>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                  <DatabaseIcon className="size-4" /> {ui("screen.d28c18b3d43a8a")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-2xl font-semibold tabular-nums">{status.counts.cves.toLocaleString()}</p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">{ui("screen.fb67169d84bb84")}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-2xl font-semibold tabular-nums text-red-500">{status.counts.kev.toLocaleString()}</p>
              </CardContent>
            </Card>
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground">{ui("screen.79a99b3849b59b")}</CardTitle>
              </CardHeader>
              <CardContent>
                <p className="text-2xl font-semibold tabular-nums">{status.counts.pocs.toLocaleString()}</p>
              </CardContent>
            </Card>
          </div>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">{ui("screen.bc03f55f6d8a42")}</CardTitle>
              <CardDescription className="text-xs">{ui("screen.695957d66725d6")}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex items-center gap-3">
                <Switch checked={!!status.enabled} onCheckedChange={toggleEnabled} id="vi-enabled" />
                <Label htmlFor="vi-enabled" className="text-sm">
                  {ui("screen.9e4fe121d6a88c")}
                </Label>
                <span className="text-muted-foreground text-xs">{status.next_tick_hint}</span>
              </div>

              <div className="grid gap-3 sm:grid-cols-2">
                <div className="grid gap-1.5">
                  <Label htmlFor="vi-nvd" className="text-xs">
                    {cfg?.nvd_key_set ? ui("screen.cd71a968d16738") : ui("screen.588fd906aeb0f4")}
                  </Label>
                  <Input
                    id="vi-nvd"
                    type="password"
                    value={nvdKey}
                    onChange={(e) => setNvdKey(e.target.value)}
                    placeholder={cfg?.nvd_key_set ? "••••••••" : ui("screen.30ebcafd82e94a")}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="vi-gh" className="text-xs">
                    {cfg?.github_token_set ? ui("screen.3ba22e069636e9") : ui("screen.e5f4f82cb7c674")}
                  </Label>
                  <Input
                    id="vi-gh"
                    type="password"
                    value={ghToken}
                    onChange={(e) => setGhToken(e.target.value)}
                    placeholder={cfg?.github_token_set ? "••••••••" : ui("screen.a31ebeebfeb4c3")}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="vi-proxy" className="text-xs">
                    {ui("screen.63cb61ceef5728")}
                  </Label>
                  <Input
                    id="vi-proxy"
                    value={proxy}
                    onChange={(e) => setProxy(e.target.value)}
                    placeholder={cfg?.maintenance_proxy || "http://127.0.0.1:7897"}
                  />
                  {cfg?.maintenance_proxy && (
                    <span className="text-muted-foreground text-xs">
                      {ui("screen.2b965e000b3813", { v0: cfg.maintenance_proxy })}
                    </span>
                  )}
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="vi-backfill" className="text-xs">
                    {ui("screen.9719f34ed32220")}
                  </Label>
                  <Input
                    id="vi-backfill"
                    type="number"
                    min={0}
                    value={backfillDays}
                    onChange={(e) => setBackfillDays(Math.max(0, Number(e.target.value) || 0))}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="vi-runtime" className="text-xs">
                    {ui("screen.ab147551c2f676")}
                  </Label>
                  <Input
                    id="vi-runtime"
                    type="number"
                    min={5}
                    value={maxRuntimeMin}
                    onChange={(e) => setMaxRuntimeMin(Math.max(5, Number(e.target.value) || 5))}
                  />
                </div>
                <div className="flex items-end gap-3">
                  <div className="flex items-center gap-2">
                    <Switch checked={pocEnabled} onCheckedChange={setPocEnabled} id="vi-poc" />
                    <Label htmlFor="vi-poc" className="text-sm">
                      {ui("screen.f06113842d0ab0")}
                    </Label>
                  </div>
                </div>
              </div>

              <div className="flex justify-end">
                <Button size="sm" onClick={saveConfig} disabled={saving}>
                  <SaveIcon className="mr-1 size-4" />
                  {ui("screen.6e584e3d5ce64a")}
                </Button>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="text-sm font-medium">{ui("screen.2f377a47158122")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b text-left text-muted-foreground">
                      <th className="py-2 pr-4 font-medium">{ui("screen.186be2232e2c9b")}</th>
                      <th className="py-2 pr-4 font-medium">{ui("screen.d7a5f76dfcdade")}</th>
                      <th className="py-2 pr-4 font-medium">{ui("screen.1e11701d6ec73a")}</th>
                      <th className="py-2 pr-4 font-medium">{ui("screen.6320b4a8722a85")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {status.sources.length === 0 && (
                      <tr>
                        <td colSpan={4} className="py-3 text-muted-foreground">
                          {ui("screen.7dfe1ad4423aa0")}
                        </td>
                      </tr>
                    )}
                    {status.sources.map((s) => (
                      <tr key={s.source} className="border-b align-top last:border-0">
                        <td className="py-2 pr-4">{ui(SOURCE_LABEL[s.source] ?? s.source)}</td>
                        <td className="py-2 pr-4 tabular-nums">{fmtTime(s.last_sync_at)}</td>
                        <td className="py-2 pr-4 tabular-nums">{s.total_synced.toLocaleString()}</td>
                        <td className="py-2 pr-4">
                          {s.last_error ? (
                            <div className="space-y-1">
                              <Badge variant="destructive" className="gap-1">
                                <TriangleAlertIcon className="size-3" />
                                {ui("screen.7deaf76d44fad6")}
                              </Badge>
                              <p className="max-w-xl text-muted-foreground text-xs">{s.last_error}</p>
                              {s.error_hint && (
                                <p className="max-w-xl text-xs text-amber-600 dark:text-amber-400">
                                  {ui("screen.8fe974714f5005", { v0: s.error_hint })}
                                </p>
                              )}
                            </div>
                          ) : (
                            <Badge variant="default">{ui("screen.296de0e31f8c22")}</Badge>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-2">
              <CardTitle className="flex items-center gap-2 text-sm font-medium">
                <FolderGitIcon className="size-4" /> {ui("screen.647e00ce656f91")}
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              <p className="text-muted-foreground text-xs">
                {ui("screen.8cf3739efdd298", {
                  v0: status.nuclei?.dir || status.nuclei_templates_dir || ui("screen.80a57e03f0717f"),
                })}
              </p>
              {status.nuclei && (
                <>
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="secondary">{ui("screen.1d81e52c61728d", { v0: status.nuclei.templates })}</Badge>
                    {status.nuclei.is_git && <Badge variant="outline">{ui("screen.3ca6aec01101bf")}</Badge>}
                    {status.nuclei.updated_at && (
                      <span className="text-muted-foreground text-xs">
                        {ui("screen.f06a14160b9f81", { v0: fmtTime(status.nuclei.updated_at) })}
                      </span>
                    )}
                  </div>
                  <p className="text-muted-foreground text-xs">{status.nuclei.hint}</p>
                </>
              )}
            </CardContent>
          </Card>

          {probeRows.length > 0 && (
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="flex items-center gap-2 text-sm font-medium">
                  <ActivityIcon className="size-4" /> {ui("screen.de94f8b3ec85cd")}
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="overflow-x-auto">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b text-left text-muted-foreground">
                        <th className="py-2 pr-4 font-medium">{ui("screen.186be2232e2c9b")}</th>
                        <th className="py-2 pr-4 font-medium">{ui("screen.bb7ef73495dd8a")}</th>
                        <th className="py-2 pr-4 font-medium">{ui("screen.e77e3d58b0dcff")}</th>
                        <th className="py-2 pr-4 font-medium">{ui("screen.4262c45dc797bb")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {probeRows.map((r) => (
                        <tr key={r.source} className="border-b align-top last:border-0">
                          <td className="py-2 pr-4">{ui(SOURCE_LABEL[r.source] ?? r.source)}</td>
                          <td className="py-2 pr-4">
                            {r.ok ? (
                              <Badge variant="default">{ui("screen.7e3cf34858df8f", { v0: r.status })}</Badge>
                            ) : (
                              <Badge variant="destructive">{r.status ? `HTTP ${r.status}` : ui("screen.28384d7afd2e4f")}</Badge>
                            )}
                          </td>
                          <td className="py-2 pr-4 tabular-nums">{r.latency_ms} ms</td>
                          <td className="py-2 pr-4 text-xs">
                            {r.error && <p className="text-destructive">{r.error}</p>}
                            {r.hint && <p className="text-amber-600 dark:text-amber-400">{r.hint}</p>}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </CardContent>
            </Card>
          )}
        </>
      )}
    </div>
  );
}
