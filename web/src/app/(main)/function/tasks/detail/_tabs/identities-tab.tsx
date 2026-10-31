"use client";

import * as React from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { api } from "@/lib/api";

interface Identity {
  id: number;
  task_id: number;
  name: string;
  kind: "password" | "cookie" | "token" | "header";
  username?: string;
  password?: string;
  cookie?: string;
  token?: string;
  headers?: Record<string, string>;
  notes?: string;
  enabled: boolean;
}

const empty: Partial<Identity> = { kind: "password", enabled: true };

export function IdentitiesTab({ taskId }: { taskId: string }) {
  const [items, setItems] = React.useState<Identity[]>([]);
  const [draft, setDraft] = React.useState<Partial<Identity>>({ ...empty });
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(() => {
    api
      .get<Identity[]>(`/api/tasks/${taskId}/identities`)
      .then((data) => {
        setItems(data || []);
        setLoading(false);
      })
      .catch((e) => {
        toast.error(`加载身份失败：${(e as Error).message}`);
        setLoading(false);
      });
  }, [taskId]);

  React.useEffect(() => {
    load();
  }, [load]);

  const save = () => {
    if (!draft.name?.trim()) {
      toast.error("身份名称不能为空");
      return;
    }
    api
      .post<Identity>(`/api/tasks/${taskId}/identities`, { ...draft, name: draft.name!.trim() })
      .then((it) => {
        setItems((prev) => [...prev, it]);
        setDraft({ ...empty });
        toast.success("身份已保存");
      })
      .catch((e) => toast.error(`保存失败：${(e as Error).message}`));
  };

  const toggle = (it: Identity, on: boolean) => {
    api
      .put<Identity>(`/api/tasks/${taskId}/identities/${it.id}`, { ...it, enabled: on })
      .then((updated) => {
        setItems((prev) => prev.map((x) => (x.id === it.id ? updated : x)));
      })
      .catch((e) => toast.error(`更新失败：${(e as Error).message}`));
  };

  const remove = (it: Identity) => {
    api
      .delete(`/api/tasks/${taskId}/identities/${it.id}`)
      .then(() => setItems((prev) => prev.filter((x) => x.id !== it.id)))
      .catch((e) => toast.error(`删除失败：${(e as Error).message}`));
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>新增身份</CardTitle>
          <CardDescription>
            配置后 Planner 可在意图中绑定身份（identity_id），Worker 将强制使用该身份的会话执行。
            未配置或 identity_id=0 时按匿名执行；匿名意图与带身份意图应并行派发，不能互相替代。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-2 md:grid-cols-2">
            <div>
              <Label htmlFor="identity-name">名称</Label>
              <Input
                id="identity-name"
                placeholder="如：普通用户A / 管理员B"
                value={draft.name ?? ""}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              />
            </div>
            <div>
              <Label htmlFor="identity-kind">类型</Label>
              <Select value={draft.kind} onValueChange={(v) => setDraft({ ...draft, kind: v as Identity["kind"] })}>
                <SelectTrigger id="identity-kind">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="password">账号密码</SelectItem>
                  <SelectItem value="cookie">Cookie</SelectItem>
                  <SelectItem value="token">Token / API Key</SelectItem>
                  <SelectItem value="header">自定义请求头</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          {draft.kind === "password" && (
            <div className="grid gap-2 md:grid-cols-2">
              <div>
                <Label htmlFor="identity-username">账号</Label>
                <Input id="identity-username" value={draft.username ?? ""} onChange={(e) => setDraft({ ...draft, username: e.target.value })} />
              </div>
              <div>
                <Label htmlFor="identity-password">密码</Label>
                <Input id="identity-password" type="password" value={draft.password ?? ""} onChange={(e) => setDraft({ ...draft, password: e.target.value })} />
              </div>
            </div>
          )}
          {draft.kind === "cookie" && (
            <div>
              <Label htmlFor="identity-cookie">Cookie 字符串</Label>
              <Textarea id="identity-cookie" rows={2} placeholder="session=xxx; token=yyy" value={draft.cookie ?? ""} onChange={(e) => setDraft({ ...draft, cookie: e.target.value })} />
            </div>
          )}
          {draft.kind === "token" && (
            <div>
              <Label htmlFor="identity-token">Token / API Key</Label>
              <Input id="identity-token" value={draft.token ?? ""} onChange={(e) => setDraft({ ...draft, token: e.target.value })} />
            </div>
          )}
          {draft.kind === "header" && (
            <div>
              <Label htmlFor="identity-headers">自定义请求头（JSON）</Label>
              <Textarea id="identity-headers" rows={2} placeholder='{"X-User-Role": "admin"}' value={draft.headers ? JSON.stringify(draft.headers) : ""} onChange={(e) => {
                try {
                  setDraft({ ...draft, headers: JSON.parse(e.target.value || "{}") });
                } catch {
                  // keep raw text while user is typing
                }
              }} />
            </div>
          )}
          <div>
            <Label htmlFor="identity-notes">备注</Label>
            <Input id="identity-notes" placeholder="用途说明（如：普通用户，用于水平越权对比）" value={draft.notes ?? ""} onChange={(e) => setDraft({ ...draft, notes: e.target.value })} />
          </div>
          <Button onClick={save}>保存身份</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>已配置身份（{items.length}）</CardTitle>
          <CardDescription>启用后 Planner 可在 add_intent 中绑定 identity_id；Worker 将强制使用该身份的会话执行。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {loading ? <p className="text-sm text-muted-foreground">加载中…</p> : null}
          {!loading && items.length === 0 ? <p className="text-sm text-muted-foreground">尚未配置身份，当前全部按匿名执行。</p> : null}
          {items.map((it) => (
            <div key={it.id} className="flex items-center justify-between rounded border p-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{it.name}</span>
                  <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{it.kind}</code>
                  {it.enabled ? <span className="text-xs text-green-600">启用</span> : <span className="text-xs text-muted-foreground">禁用</span>}
                </div>
                {it.notes ? <p className="truncate text-xs text-muted-foreground">{it.notes}</p> : null}
                <p className="truncate text-xs text-muted-foreground">
                  {it.kind === "password" && it.username ? `${it.username} / ${it.password ? "••••••" : ""}` : null}
                  {it.kind === "cookie" && it.cookie ? `Cookie: ${it.cookie.slice(0, 60)}${it.cookie.length > 60 ? "…" : ""}` : null}
                  {it.kind === "token" && it.token ? `Token: ${it.token.slice(0, 24)}${it.token.length > 24 ? "…" : ""}` : null}
                  {it.kind === "header" && it.headers ? `Headers: ${JSON.stringify(it.headers).slice(0, 60)}` : null}
                </p>
              </div>
              <div className="flex items-center gap-2">
                <Switch checked={it.enabled} onCheckedChange={(v) => toggle(it, v)} />
                <Button size="sm" variant="destructive" onClick={() => remove(it)}>删除</Button>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
