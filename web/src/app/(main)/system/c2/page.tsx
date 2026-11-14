"use client";

import * as React from "react";
import { toast } from "sonner";
import { PlayIcon, RadioTower, SquareIcon, RefreshCwIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { useUITranslation } from "@/i18n/use-ui-translation";
import { api } from "@/lib/api";
import type { C2Config, ListenerSession, ListenerView } from "@/lib/types";

const MODE_LABEL: Record<string, string> = {
  direct: "screen.558ca3d73f0ec3",
  ssh: "screen.411fda6a943cb4",
  manual: "screen.958edf57483b89",
};

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function fmtTime(iso: string): string {
  const d = new Date(iso);
  return isNaN(d.getTime()) ? iso : d.toLocaleString();
}

export default function C2Page() {
  const { ui, liveUI } = useUITranslation();
  // ── C2 出口配置 ──
  const [cfg, setCfg] = React.useState<C2Config | null>(null);
  const [mode, setMode] = React.useState<string>("direct");
  const [host, setHost] = React.useState("");
  const [ssh, setSsh] = React.useState("");
  const [pass, setPass] = React.useState("");
  const [key, setKey] = React.useState("");
  const [iface, setIface] = React.useState("");
  const [hasPass, setHasPass] = React.useState(false);
  const [savingCfg, setSavingCfg] = React.useState(false);

  // ── 监听器 ──
  const [listeners, setListeners] = React.useState<ListenerView[]>([]);
  const [name, setName] = React.useState("");
  const [port, setPort] = React.useState("");
  const [starting, setStarting] = React.useState(false);
  const [pendingStop, setPendingStop] = React.useState<ListenerView | null>(null);
  const [stopping, setStopping] = React.useState(false);
  const [expanded, setExpanded] = React.useState<string | null>(null);
  const [refreshing, setRefreshing] = React.useState(false);

  const loadCfg = React.useCallback(async () => {
    try {
      const c = await api.c2Config();
      setCfg(c);
      setMode(c.mode || "direct");
      setHost(c.host || "");
      setSsh(c.ssh || "");
      setKey(c.key || "");
      setIface(c.iface || "");
      setHasPass(c.has_pass);
      setPass("");
    } catch (e) {
      toast.error(liveUI("screen.f6fca8fde47099", { v0: (e as Error).message }));
    }
  }, [liveUI]);

  const loadListeners = React.useCallback(async () => {
    setRefreshing(true);
    try {
      const r = await api.listeners();
      setListeners(r.listeners ?? []);
    } catch (e) {
      toast.error(liveUI("screen.4bcd7850af6eeb", { v0: (e as Error).message }));
    } finally {
      setRefreshing(false);
    }
  }, [liveUI]);

  React.useEffect(() => {
    void loadCfg();
    void loadListeners();
    // 监听器状态与会话在变（目标回连随时进来），10s 轮询与任务列表同节奏。
    const t = setInterval(() => void loadListeners(), 10_000);
    return () => clearInterval(t);
  }, [loadCfg, loadListeners]);

  async function saveCfg() {
    setSavingCfg(true);
    try {
      const patch: Record<string, string> = { mode };
      if (mode === "manual" || mode === "ssh") patch.host = host;
      if (mode === "ssh") {
        patch.ssh = ssh;
        patch.key = key;
        // 密码留空=不变更；要清空输入一个空格再 trim 的歧义不值得——这里约定
        // 留空即不变，后端 has_pass 提示当前是否已设。
        if (pass !== "") patch.password = pass;
      }
      if (mode === "direct") patch.iface = iface;
      const saved = await api.saveC2Config(patch);
      setCfg(saved);
      setHasPass(saved.has_pass);
      setPass("");
      toast.success(liveUI("screen.060dddf55dee27"));
    } catch (e) {
      toast.error(liveUI("screen.20e1b0f2590bc7", { v0: (e as Error).message }));
    } finally {
      setSavingCfg(false);
    }
  }

  async function startListener() {
    const p = Number(port);
    const nm = name.trim();
    if (!nm) {
      toast.error(liveUI("screen.38c2ed175e70bc"));
      return;
    }
    if (!Number.isInteger(p) || p < 1024 || p > 65535) {
      toast.error(liveUI("screen.5655b6d1e29b3a"));
      return;
    }
    setStarting(true);
    try {
      const r = await api.startListener(nm, p);
      toast.success(
        liveUI("screen.009ee02dfdca75", { v0: r.name, v1: r.connect_back.lhost, v2: r.connect_back.lport }),
      );
      setName("");
      setPort("");
      await loadListeners();
      setExpanded(r.name);
    } catch (e) {
      toast.error(liveUI("screen.e3c3ddff13abc1", { v0: (e as Error).message }));
    } finally {
      setStarting(false);
    }
  }

  async function stopListener(l: ListenerView) {
    setStopping(true);
    try {
      const r = await api.stopListener(l.name);
      toast.success(liveUI("screen.078c9237b27e4e", { v0: r.stopped, v1: r.sessions.length }));
      setPendingStop(null);
      await loadListeners();
    } catch (e) {
      toast.error(liveUI("screen.8db4fd2d271d4e", { v0: (e as Error).message }));
    } finally {
      setStopping(false);
    }
  }

  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-4">
      {/* ── C2 出口配置 ── */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <RadioTower className="size-4" /> {ui("screen.4fd8f95547862d")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4">
          <p className="text-muted-foreground text-sm">
            {ui("screen.9ce8be9b326844")}
            <b>direct</b>
            {ui("screen.6a177a280d3886")}
            <b>ssh</b>
            {ui("screen.9aac7b2e715a05")}
            <b>manual</b>
            {ui("screen.ce8241ae862b0c")} <code>listener_start</code> {ui("screen.7c435f6743a836")}
          </p>
          <div className="grid gap-2 sm:max-w-sm">
            <Label>{ui("screen.404fd6c8177e0a")}</Label>
            <Select value={mode} onValueChange={setMode}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="direct">{ui("screen.10db06d3d3e201")}</SelectItem>
                <SelectItem value="ssh">{ui("screen.c9db95815576f6")}</SelectItem>
                <SelectItem value="manual">{ui("screen.b63305f19d126c")}</SelectItem>
              </SelectContent>
            </Select>
          </div>
          {(mode === "ssh" || mode === "manual") && (
            <div className="grid gap-2 sm:max-w-sm">
              <Label>{ui("screen.f535f68d7cbe50")}</Label>
              <Input value={host} onChange={(e) => setHost(e.target.value)} placeholder={ui("screen.33db8dc3bea133")} />
            </div>
          )}
          {mode === "ssh" && (
            <>
              <div className="grid gap-2 sm:max-w-sm">
                <Label>{ui("screen.532c5e5e4b14fa")}</Label>
                <Input value={ssh} onChange={(e) => setSsh(e.target.value)} placeholder="root@1.2.3.4:22" />
              </div>
              <div className="grid gap-2 sm:max-w-sm">
                <Label>
                  {ui("screen.6f0890a1b0b0b5")}
                  {hasPass ? ui("screen.7e51bb5e26adf8") : ""}
                </Label>
                <Input
                  type="password"
                  value={pass}
                  onChange={(e) => setPass(e.target.value)}
                  placeholder={hasPass ? "••••••••" : ui("screen.0eb502fd3103f9")}
                />
              </div>
              <div className="grid gap-2 sm:max-w-sm">
                <Label>{ui("screen.734686f9eac07f")}</Label>
                <Input value={key} onChange={(e) => setKey(e.target.value)} placeholder="/root/.ssh/id_rsa" />
              </div>
            </>
          )}
          {mode === "direct" && (
            <div className="grid gap-2 sm:max-w-sm">
              <Label>{ui("screen.b21b550553d63f")}</Label>
              <Input value={iface} onChange={(e) => setIface(e.target.value)} placeholder={ui("screen.1d0161cfe06b39")} />
            </div>
          )}
          <div>
            <Button onClick={saveCfg} disabled={savingCfg}>
              {savingCfg ? ui("screen.ff509c9ba052a2") : ui("screen.6e584e3d5ce64a")}
            </Button>
            {cfg && (
              <span className="text-muted-foreground ml-3 text-sm">
                {ui("screen.55e54c335c596e", { v0: ui(MODE_LABEL[cfg.mode] ?? cfg.mode) })}
                {cfg.ssh ? ` · ${cfg.ssh}` : ""}
              </span>
            )}
          </div>
        </CardContent>
      </Card>

      {/* ── 监听器 ── */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center justify-between text-base">
            <span className="flex items-center gap-2">
              <RadioTower className="size-4" /> {ui("screen.5ac0c406f84d5b", { v0: listeners.length })}
            </span>
            <Button size="sm" variant="outline" onClick={() => void loadListeners()} disabled={refreshing}>
              <RefreshCwIcon className={refreshing ? "animate-spin" : ""} /> {ui("screen.aee88743413144")}
            </Button>
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-4">
          <div className="grid items-end gap-2 sm:grid-cols-[1fr_140px_auto]">
            <div className="grid gap-1">
              <Label>{ui("screen.7d71d1644ea7d0")}</Label>
              <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="shell-main" />
            </div>
            <div className="grid gap-1">
              <Label>{ui("screen.910bf737a892f3")}</Label>
              <Input value={port} onChange={(e) => setPort(e.target.value)} placeholder="4444" inputMode="numeric" />
            </div>
            <Button onClick={() => void startListener()} disabled={starting}>
              <PlayIcon /> {starting ? ui("screen.fc89857ef91751") : ui("screen.49b50f08b0ea5a")}
            </Button>
          </div>

          {listeners.length === 0 && (
            <p className="text-muted-foreground text-sm">
              {ui("screen.ea43637993a375")} <code>listener_start</code> {ui("screen.f6576daf9c78f5")}
            </p>
          )}

          {listeners.map((l) => (
            <div key={l.name} className="rounded-lg border">
              <div className="flex flex-wrap items-center gap-2 p-3">
                <Badge variant="secondary">{l.name}</Badge>
                <span className="font-mono text-sm">:{l.port}</span>
                <span className="text-muted-foreground text-sm">{ui("screen.3c386034377baf", { v0: fmtTime(l.started_at) })}</span>
                <Badge variant={l.sessions.some((s) => s.open) ? "default" : "outline"}>
                  {ui("screen.4e5cc0414b0115", { v0: l.sessions.length })}
                </Badge>
                <div className="grow" />
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setExpanded(expanded === l.name ? null : l.name)}
                >
                  {expanded === l.name ? ui("screen.3992c4aada6279") : ui("screen.f2b4237b35f0b3")}
                </Button>
                <Button size="sm" variant="outline" className="text-destructive" onClick={() => setPendingStop(l)}>
                  <SquareIcon /> {ui("screen.ca4d973c0b006b")}
                </Button>
              </div>
              {expanded === l.name && (
                <div className="border-t">
                  {l.sessions.length === 0 ? (
                    <p className="text-muted-foreground p-3 text-sm">{ui("screen.017b362c1f15a6")}</p>
                  ) : (
                    l.sessions.map((s) => <SessionRow key={s.id} s={s} />)
                  )}
                </div>
              )}
            </div>
          ))}
        </CardContent>
      </Card>

      {/* ── 停止确认 ── */}
      <AlertDialog open={!!pendingStop} onOpenChange={(o) => { if (!o) setPendingStop(null); }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{ui("screen.01f3498f35cefb", { v0: pendingStop?.name ?? "" })}</AlertDialogTitle>
            <AlertDialogDescription>
              {ui("screen.79828ef5309b07")}
              {pendingStop?.sessions.length ?? 0}
              {ui("screen.2bcd7da1b57cfb")}
              {ui("screen.230b67d01d4100")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={stopping}>{ui("screen.2cd0f3be8738a8")}</AlertDialogCancel>
            <AlertDialogAction
              disabled={stopping}
              onClick={(e) => { e.preventDefault(); if (pendingStop) void stopListener(pendingStop); }}
            >
              {stopping ? ui("screen.02099d7931da52") : ui("screen.ca4d973c0b006b")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function SessionRow({ s }: { s: ListenerSession }) {
  const { ui } = useUITranslation();
  const [openView, setOpenView] = React.useState(false);
  return (
    <div className="border-b px-3 py-2 last:border-b-0">
      <button
        className="flex w-full flex-wrap items-center gap-2 text-left text-sm"
        onClick={() => setOpenView(!openView)}
      >
        <Badge variant={s.open ? "default" : "secondary"}>{s.open ? ui("screen.b9086662b1df1f") : ui("screen.1f0ac6953e0411")}</Badge>
        <span className="font-mono">{s.remote}</span>
        <span className="text-muted-foreground">{ui("screen.6bc3cbfb542cec", { v0: fmtTime(s.connected_at) })}</span>
        <span className="text-muted-foreground">{fmtBytes(Number(s.bytes_in))}</span>
        <span className="text-muted-foreground grow text-right text-xs">{openView ? ui("screen.afd4b783536b82") : ui("screen.9001d54474d625")}</span>
      </button>
      {openView && (
        <pre className="bg-muted mt-2 max-h-64 overflow-auto rounded p-2 text-xs whitespace-pre-wrap">
          {s.preview || ui("screen.ee039d2f4c85e1")}
        </pre>
      )}
    </div>
  );
}
