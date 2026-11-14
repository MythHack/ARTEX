"use client";

import * as React from "react";

import {
  ChevronRightIcon,
  DownloadIcon,
  Loader2Icon,
  MonitorCog,
  PlugIcon,
  RefreshCwIcon,
  TerminalIcon,
  XIcon,
} from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useUITranslation } from "@/i18n/use-ui-translation";
import { api } from "@/lib/api";
import type { HostenvStatus, HostTool, InstallJob, ProxyCheckResult } from "@/lib/types";

const TERMINAL_STATES = new Set(["done", "failed", "cancelled"]);

const KIND_LABEL: Record<HostTool["kind"], string> = {
  apt: "screen.bebeeab6a4d883",
  github: "GitHub Release",
  pip: "pip",
  manual: "screen.2d5b36c57c75dd",
};

// 攻击面分类。顺序按攻击流程排：先摸清面(网络/DNS)→ 再打(Web/凭据)
// → 深入(代码/利用/中间件)→ 辅助(隧道/外带/资料/通用)。
const CATEGORY_ORDER: HostTool["category"][] = [
  "network",
  "dns",
  "web",
  "cred",
  "code",
  "exploit",
  "middleware",
  "pivot",
  "oob",
  "material",
  "runtime",
  "",
];

const CATEGORY_LABEL: Record<HostTool["category"], string> = {
  network: "screen.447e8fdf4e13bf",
  dns: "screen.5e76e8cd948494",
  web: "screen.99708f8ab29952",
  cred: "screen.1ebe85e3e3cc44",
  code: "screen.2a1fec504c231b",
  exploit: "screen.8112954c1019f2",
  middleware: "screen.c0c8f13582a2ba",
  pivot: "screen.e6427776189ba7",
  oob: "screen.4afedb5f121919",
  material: "screen.c821f84ed574ba",
  runtime: "screen.d5fa24bfea8f5f",
  "": "screen.d2909f1647e7c8",
};

// 宿主环境:工具体检。二进制部署形态下系统里装了什么,agent 就能经 Bash
// 直接调什么;此页把家底显式化,缺失项可一键安装(由特权助手 artex-hostd 执行)。
export default function HostEnvPage() {
  const { ui, liveUI } = useUITranslation();
  const [inv, setInv] = React.useState<HostenvStatus | null>(null);
  const [loaded, setLoaded] = React.useState(false);
  const [proxyInput, setProxyInput] = React.useState("");
  const [savingProxy, setSavingProxy] = React.useState(false);
  const [testingProxy, setTestingProxy] = React.useState(false);
  const [proxyCheck, setProxyCheck] = React.useState<ProxyCheckResult | null>(null);
  // 按工具 id 跟踪安装任务：批量「一键装齐」会把 N 个任务一起塞进助手队列
  // （助手串行执行），所以不再是一个单槽位 job。
  const [jobsByTool, setJobsByTool] = React.useState<Record<string, InstallJob>>({});
  const [selectedJobTool, setSelectedJobTool] = React.useState<string | null>(null);
  const [batchBusy, setBatchBusy] = React.useState(false);
  const logRef = React.useRef<HTMLPreElement | null>(null);

  const refresh = React.useCallback(() => {
    api
      .hostenvStatus()
      .then((s) => {
        setInv(s);
        setProxyInput(s.installer.proxy ?? "");
      })
      .catch(() => undefined)
      .finally(() => setLoaded(true));
  }, []);

  React.useEffect(() => {
    refresh();
  }, [refresh]);

  const activeJobs = React.useMemo(
    () => Object.values(jobsByTool).filter((j) => !TERMINAL_STATES.has(j.state)),
    [jobsByTool],
  );
  const installing = activeJobs.length > 0 || batchBusy;

  // 安装任务轮询：所有进行中的任务每 1.2s 并行拉一次；全部到终态后刷新体检。
  const refreshedAllRef = React.useRef(true);
  React.useEffect(() => {
    if (activeJobs.length === 0) {
      refreshedAllRef.current = true;
      return;
    }
    const timer = setInterval(async () => {
      const results = await Promise.allSettled(
        activeJobs.map((j) => api.hostenvJob(j.id)),
      );
      let sawTerminal = false;
      setJobsByTool((prev) => {
        const next = { ...prev };
        for (const r of results) {
          if (r.status !== "fulfilled") continue; // 轮询失败不打断，下一轮再试
          const j = r.value;
          const before = next[j.tool_id];
          if (!before || before.state === j.state) continue;
          next[j.tool_id] = j;
          if (TERMINAL_STATES.has(j.state)) {
            sawTerminal = true;
            if (j.state === "done") toast.success(liveUI("screen.951327e22d00ea", { v0: j.tool_name }));
            else if (j.state === "failed") toast.error(liveUI("screen.5e737b815ef107", { v0: j.tool_name }));
            else toast.info(liveUI("screen.b9a0e85adef5a7", { v0: j.tool_name }));
          }
        }
        return next;
      });
      // 有任务到终态就刷新一次体检（后续轮次里 refresh 会幂等地再对齐）。
      if (sawTerminal) refresh();
    }, 1200);
    return () => clearInterval(timer);
  }, [activeJobs, refresh, liveUI]);

  React.useEffect(() => {
    if (logRef.current) logRef.current.scrollTop = logRef.current.scrollHeight;
  }, []);

  const saveProxy = async () => {
    setSavingProxy(true);
    try {
      await api.setSettings({ install_proxy: proxyInput.trim() });
      toast.success(liveUI("screen.e87a609aceef53"));
      refresh();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : liveUI("screen.6309a3bb5ba4c7"));
    } finally {
      setSavingProxy(false);
    }
  };

  // 代理会失效，所以改完必须能立刻验证通路——否则只能等装工具失败才发现。
  // 测的是输入框里的值（还没保存），保存与否都能先看清楚对不对。
  const testProxy = async () => {
    setTestingProxy(true);
    setProxyCheck(null);
    try {
      const r = await api.hostenvProxyCheck(proxyInput.trim());
      setProxyCheck(r);
      if (r.ok)
        toast.success(
          liveUI("screen.45add8b9e44eb3") + (r.exit_ip ? liveUI("screen.4887977f27da7c", { v0: r.exit_ip }) : ""),
        );
      else toast.error(liveUI("screen.df4e22d0f18503", { v0: r.error || r.hint || liveUI("screen.a1a9da9ee6ed04") }));
    } catch (e) {
      toast.error(e instanceof Error ? e.message : liveUI("screen.77c9e582e85583"));
    } finally {
      setTestingProxy(false);
    }
  };

  const install = async (tool: HostTool) => {
    try {
      const j = await api.hostenvInstall(tool.id);
      setJobsByTool((prev) => ({ ...prev, [tool.id]: j }));
      setSelectedJobTool(tool.id);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : liveUI("screen.eb05e9208a070e"));
    }
  };

  // 一键装齐：把一批工具全部塞进助手队列（助手串行执行，无需等待单个完成）。
  const installMany = async (tools: HostTool[]) => {
    const ids = tools.map((t) => t.id);
    if (ids.length === 0) return;
    setBatchBusy(true);
    try {
      const r = await api.hostenvInstallBatch(ids);
      setJobsByTool((prev) => {
        const next = { ...prev };
        const byId = new Map(tools.map((t) => [t.id, t]));
        for (const j of r.started) {
          const t = byId.get(j.tool_id);
          if (!t) continue;
          next[j.tool_id] = {
            ...j,
            id: j.job_id,
            tool_id: t.id,
            tool_name: t.name,
            state: "queued",
            exit_code: 0,
            started_at: new Date().toISOString(),
            log: "",
          } as InstallJob;
        }
        return next;
      });
      if (r.started.length > 0) setSelectedJobTool(r.started[0].tool_id);
      if (r.skipped.length > 0) {
        toast.warning(
          liveUI("screen.981bb7933f6adf", {
            v0: r.started.length,
            v1: r.skipped.length,
            v2: r.skipped
              .slice(0, 3)
              .map((x) => x.tool_id)
              .join("、"),
            v3: r.skipped.length > 3 ? liveUI("screen.8ab274ae26fa17") : "",
          }),
        );
      } else {
        toast.success(liveUI("screen.1f28b6fd8b54d0", { v0: r.started.length }));
      }
    } catch (e) {
      toast.error(e instanceof Error ? e.message : liveUI("screen.963e9ea05b6dca"));
    } finally {
      setBatchBusy(false);
    }
  };

  const cancel = async () => {
    const j = selectedJobTool ? jobsByTool[selectedJobTool] : null;
    if (!j) return;
    try {
      await api.hostenvCancel(j.id);
    } catch {
      // 忽略：任务可能刚好自己结束了
    }
  };

  const available = inv?.tools.filter((t) => t.available) ?? [];
  const missing = inv?.tools.filter((t) => !t.available) ?? [];
  const installerReady = inv?.installer.available ?? false;

  // 按攻击面分组。目录里有近 40 个工具，平铺成一张表没法看——用户要回答的是
  // 「Web 侧我有哪些家伙、缺哪些」，不是「一共有 39 行」。分组顺序固定按攻击流程，
  // 不随可用状态变化，避免刷新后行位置跳动。
  const grouped = React.useMemo(() => {
    const tools = inv?.tools ?? [];
    const byCat = new Map<HostTool["category"], HostTool[]>();
    for (const t of tools) {
      const c = t.category ?? "";
      const arr = byCat.get(c);
      if (arr) arr.push(t);
      else byCat.set(c, [t]);
    }
    const groups: { category: HostTool["category"]; tools: HostTool[] }[] = [];
    for (const c of CATEGORY_ORDER) {
      const ts = byCat.get(c);
      if (ts && ts.length > 0) groups.push({ category: c, tools: ts });
    }
    return groups;
  }, [inv?.tools]);

  // 可一键安装的缺失项：给「一键装齐」用。
  const missingInstallable = React.useMemo(
    () => (inv?.tools ?? []).filter((t) => !t.available && t.installable),
    [inv?.tools],
  );
  const toolJobState = (id: string): InstallJob["state"] | null =>
    jobsByTool[id]?.state ?? null;
  const selectedJob = selectedJobTool ? (jobsByTool[selectedJobTool] ?? null) : null;

  // 折叠状态：默认全展开。用户来这里通常是查「缺什么」，藏起来反而添乱。
  const [collapsed, setCollapsed] = React.useState<Set<string>>(new Set());
  const toggleCat = React.useCallback((c: string) => {
    setCollapsed((prev) => {
      const next = new Set(prev);
      if (next.has(c)) next.delete(c);
      else next.add(c);
      return next;
    });
  }, []);

  return (
    <div className="flex flex-col gap-4 p-4 lg:p-6">
      <Card>
        <CardHeader className="flex-row items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2 text-sm">
              <MonitorCog className="size-4" /> {ui("screen.fbc6c2b01710f1")}
            </CardTitle>
            <CardDescription className="mt-1">{ui("screen.a88cfc12b05f72")}</CardDescription>
          </div>
          <div className="flex items-center gap-2">
            {inv && (
              <Badge variant={inv.deploy_mode === "docker" ? "secondary" : "outline"}>
                {inv.deploy_mode === "docker" ? ui("screen.7cabcc483eb40c") : ui("screen.c1dd4700a63e33")}
              </Badge>
            )}
            {installerReady && missingInstallable.length > 0 && (
              <Button
                size="sm"
                variant="default"
                disabled={installing}
                onClick={() => installMany(missingInstallable)}
              >
                {installing ? <Loader2Icon className="animate-spin" /> : <DownloadIcon />}
                {ui("screen.a1e2404f1730e4", { v0: missingInstallable.length })}
              </Button>
            )}
            <Button size="sm" variant="outline" onClick={refresh}>
              <RefreshCwIcon /> {ui("screen.aee88743413144")}
            </Button>
          </div>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          {!loaded ? (
            <div className="space-y-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : (
            <>
              {/* 装机代理：只作用于安装动作,与目标流量/搜索/LLM 代理无关 */}
              <div className="flex flex-col gap-2 rounded-lg border bg-muted/30 p-3">
                <div className="flex items-center justify-between gap-2">
                  <Label htmlFor="install-proxy" className="text-sm">
                    {ui("screen.ec4a7500846f5d")}
                  </Label>
                  <div className="flex items-center gap-2">
                    {proxyCheck && (
                      <Badge
                        variant={proxyCheck.ok ? "default" : "destructive"}
                        title={proxyCheck.hint || proxyCheck.error || ""}
                      >
                        {proxyCheck.ok
                          ? ui("screen.4d99c976beb882") +
                            (proxyCheck.latency_ms ? ` · ${proxyCheck.latency_ms}ms` : "")
                          : ui("screen.460b3574e4bdb8")}
                      </Badge>
                    )}
                    <span className="text-muted-foreground text-xs">
                      {installerReady ? ui("screen.94e5c43fb66db0") : ui("screen.e4d06c9c4871d3")}
                    </span>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Input
                    id="install-proxy"
                    autoComplete="off"
                    className="font-mono text-xs"
                    placeholder={ui("screen.50f6fb3e284225")}
                    value={proxyInput}
                    disabled={!loaded || savingProxy}
                    onChange={(e) => {
                      setProxyInput(e.target.value);
                      setProxyCheck(null);
                    }}
                  />
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={testProxy}
                    disabled={!loaded || testingProxy}
                  >
                    {testingProxy ? <Loader2Icon className="animate-spin" /> : <PlugIcon />}
                    {ui("screen.6aa8f49cc992df")}
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={saveProxy}
                    disabled={!loaded || savingProxy}
                  >
                    {ui("screen.a3030bf8f16dc6")}
                  </Button>
                </div>
                {proxyCheck && !proxyCheck.ok && (
                  <p className="text-destructive text-xs">
                    {proxyCheck.error || ui("screen.da2d68cde96dd1")}
                    {proxyCheck.hint ? ` — ${proxyCheck.hint}` : ""}
                  </p>
                )}
                {proxyCheck?.ok && proxyCheck.exit_ip && (
                  <p className="text-muted-foreground text-xs">
                    {ui("screen.db92b1d35eb173")} <span className="font-mono">{proxyCheck.exit_ip}</span>
                  </p>
                )}
                <p className="text-muted-foreground text-xs">{ui("screen.1abf63ba92e0d6")}</p>
              </div>

              {!installerReady && inv?.installer.reason && (
                <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
                  {inv.installer.reason}
                  {inv.deploy_mode === "docker" && (
                    <span className="text-muted-foreground">{ui("screen.7fb282d1989f8d")}</span>
                  )}
                </div>
              )}

              {installerReady && inv?.installer.upgrade_required && (
                <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-sm">
                  {ui("screen.af11dfce168a1e")}{" "}
                  <code className="font-mono">install.sh</code> {ui("screen.f801196be8eeb5")}
                </div>
              )}

              <p className="text-muted-foreground text-sm">
                {ui("screen.4d99c976beb882")} <span className="font-medium text-foreground">{available.length}</span> / {inv?.tools.length ?? 0}
                {missing.length > 0 && (
                  <span className="ml-3">
                    {ui("screen.65364e6fd8c7b5")} {missing.length} {ui("screen.49ccde43a15497")}
                    {installerReady ? ui("screen.697438e1bec810") : ""}
                  </span>
                )}
              </p>

              <div className="flex flex-col gap-3">
                {grouped.map((g) => {
                  const isCollapsed = collapsed.has(g.category);
                  const okCount = g.tools.filter((t) => t.available).length;
                  return (
                    <div key={g.category} className="rounded-lg border">
                      <div className="flex w-full items-center gap-2 px-3 py-2">
                        <button
                          type="button"
                          onClick={() => toggleCat(g.category)}
                          className="flex flex-1 items-center gap-2 text-left hover:bg-muted/50"
                        >
                          <ChevronRightIcon
                            className={
                              "size-4 shrink-0 text-muted-foreground transition-transform" +
                              (isCollapsed ? "" : "rotate-90")
                            }
                          />
                          <span className="font-medium text-sm">{ui(CATEGORY_LABEL[g.category])}</span>
                          <span className="text-muted-foreground text-xs">
                            {okCount}/{g.tools.length} {ui("screen.4d99c976beb882")}
                            {g.tools.length - okCount > 0 && (
                              <span className="ml-1 text-amber-600 dark:text-amber-400">
                                {ui("screen.ba2acccacf879d", { v0: g.tools.length - okCount })}
                              </span>
                            )}
                          </span>
                        </button>
                        {installerReady &&
                          g.tools.some((t) => !t.available && t.installable) && (
                            <Button
                              size="sm"
                              variant="ghost"
                              className="h-7 px-2 text-xs"
                              disabled={installing}
                              onClick={() =>
                                installMany(g.tools.filter((t) => !t.available && t.installable))
                              }
                            >
                              {ui("screen.820548af0f25d0")}
                            </Button>
                          )}
                      </div>
                      {!isCollapsed && (
                        <Table>
                          <TableHeader>
                            <TableRow>
                              <TableHead className="w-32">{ui("screen.5ca6730d441522")}</TableHead>
                              <TableHead>{ui("screen.05b36669c4ad9a")}</TableHead>
                              <TableHead className="w-24">{ui("screen.6320b4a8722a85")}</TableHead>
                              <TableHead>{ui("screen.9b5189009269a6")}</TableHead>
                              <TableHead className="w-28 text-right">{ui("screen.ed31fbb483ee1b")}</TableHead>
                            </TableRow>
                          </TableHeader>
                          <TableBody>
                            {g.tools.map((t) => (
                              <TableRow key={t.id}>
                                <TableCell className="font-mono text-sm">{t.bin}</TableCell>
                                <TableCell className="text-muted-foreground text-sm">{t.purpose}</TableCell>
                                <TableCell>
                                  {t.available ? <Badge>{ui("screen.4d99c976beb882")}</Badge> : <Badge variant="outline">{ui("screen.65364e6fd8c7b5")}</Badge>}
                                </TableCell>
                                <TableCell className="text-muted-foreground text-xs">
                                  {t.available ? (
                                    "—"
                                  ) : (
                                    // break-all：缺失项的提示里可能带长 URL（如 msfconsole 的官方文档），
                                    // 不换行会把表格撑到横向溢出，需要左右拖动才看得到第一列。
                                    <span className="break-all">
                                      <span className="mr-1 text-foreground/70">{ui(KIND_LABEL[t.kind])}</span>
                                      {t.install}
                                    </span>
                                  )}
                                </TableCell>
                                <TableCell className="text-right">
                                  {!t.available && t.installable && installerReady && (
                                    <Button
                                      size="sm"
                                      variant="outline"
                                      disabled={installing}
                                      onClick={() => install(t)}
                                    >
                                      {toolJobState(t.id) &&
                                      !TERMINAL_STATES.has(toolJobState(t.id)!) ? (
                                        <Loader2Icon className="animate-spin" />
                                      ) : (
                                        <DownloadIcon />
                                      )}
                                      {toolJobState(t.id) === "queued"
                                        ? ui("screen.d6f766f2adf532")
                                        : toolJobState(t.id) === "running"
                                          ? ui("screen.f9d5b2b63e5a47")
                                          : ui("screen.e8f88f51ccb085")}
                                    </Button>
                                  )}
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      )}
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* 安装进度：批量时每个工具一个页签，选中哪个显示哪个的日志 */}
      {selectedJob && (
        <Card>
          <CardHeader className="flex-row items-center justify-between">
            <div className="min-w-0 flex-1">
              <CardTitle className="flex items-center gap-2 text-sm">
                <TerminalIcon className="size-4" /> {ui("screen.421f0e796c878a")}
              </CardTitle>
              <CardDescription className="mt-2 flex flex-wrap items-center gap-1.5">
                {Object.values(jobsByTool)
                  .sort((a, b) => a.tool_name.localeCompare(b.tool_name))
                  .map((j) => (
                    <button
                      key={j.tool_id}
                      type="button"
                      onClick={() => setSelectedJobTool(j.tool_id)}
                      className="focus:outline-none"
                      title={`${j.tool_name} · ${j.state}`}
                    >
                      <Badge
                        variant={
                          j.tool_id === selectedJobTool
                            ? "default"
                            : j.state === "failed"
                              ? "destructive"
                              : "outline"
                        }
                      >
                        {j.state === "running" && <Loader2Icon className="mr-1 size-3 animate-spin" />}
                        {j.tool_name}
                      </Badge>
                    </button>
                  ))}
              </CardDescription>
            </div>
            <div className="flex items-center gap-2">
              {selectedJob && !TERMINAL_STATES.has(selectedJob.state) && (
                <Button size="sm" variant="outline" onClick={cancel}>
                  <XIcon /> {ui("screen.2cd0f3be8738a8")}
                </Button>
              )}
              {selectedJob && TERMINAL_STATES.has(selectedJob.state) && (
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    if (selectedJobTool) {
                      setJobsByTool((prev) => {
                        const next = { ...prev };
                        delete next[selectedJobTool];
                        return next;
                      });
                    }
                    setSelectedJobTool(null);
                  }}
                >
                  {ui("common.close")}
                </Button>
              )}
            </div>
          </CardHeader>
          <CardContent>
            <CardDescription className="mb-2 flex items-center gap-2">
              <span className="font-medium text-foreground">{selectedJob.tool_name}</span>
              <Badge
                variant={
                  selectedJob.state === "done"
                    ? "default"
                    : selectedJob.state === "failed"
                      ? "destructive"
                      : "secondary"
                }
              >
                {selectedJob.state}
              </Badge>
              {selectedJob.error && <span className="text-destructive">{selectedJob.error}</span>}
            </CardDescription>
            <pre
              ref={logRef}
              className="max-h-80 overflow-auto rounded-md bg-muted p-3 font-mono text-xs leading-relaxed"
            >
              {selectedJob.log || ui("screen.01cb8eea7d66d5")}
            </pre>
          </CardContent>
        </Card>
      )}
    </div>
  );
}
