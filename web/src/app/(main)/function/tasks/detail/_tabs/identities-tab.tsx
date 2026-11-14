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
import { useUITranslation } from "@/i18n/use-ui-translation";
import { api, http } from "@/lib/api";

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
  const { ui, liveUI } = useUITranslation();
  const [items, setItems] = React.useState<Identity[]>([]);
  const [draft, setDraft] = React.useState<Partial<Identity>>({ ...empty });
  const [loading, setLoading] = React.useState(true);

  const load = React.useCallback(() => {
    http<Identity[]>(`/tasks/${taskId}/identities`)
      .then((data) => {
        setItems(data || []);
        setLoading(false);
      })
      .catch((e) => {
        toast.error(liveUI("screen.09a1e625b27738", { v0: (e as Error).message }));
        setLoading(false);
      });
  }, [taskId, liveUI]);

  React.useEffect(() => {
    load();
  }, [load]);

  const save = () => {
    if (!draft.name?.trim()) {
      toast.error(liveUI("screen.c3549f2bcb3f50"));
      return;
    }
    http<Identity>(`/tasks/${taskId}/identities`, { method: "POST", body: JSON.stringify({ ...draft, name: draft.name!.trim() }) })
      .then((it) => {
        setItems((prev) => [...prev, it]);
        setDraft({ ...empty });
        toast.success(liveUI("screen.3cb45b95836312"));
      })
      .catch((e) => toast.error(liveUI("screen.20e1b0f2590bc7", { v0: (e as Error).message })));
  };

  const toggle = (it: Identity, on: boolean) => {
    http<Identity>(`/tasks/${taskId}/identities/${it.id}`, { method: "PUT", body: JSON.stringify({ ...it, enabled: on }) })
      .then((updated) => {
        setItems((prev) => prev.map((x) => (x.id === it.id ? updated : x)));
      })
      .catch((e) => toast.error(liveUI("screen.4cdda2a76dca47", { v0: (e as Error).message })));
  };

  const remove = (it: Identity) => {
    http(`/tasks/${taskId}/identities/${it.id}`, { method: "DELETE" })
      .then(() => setItems((prev) => prev.filter((x) => x.id !== it.id)))
      .catch((e) => toast.error(liveUI("screen.77dcc9d08e857c", { v0: (e as Error).message })));
  };

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{ui("screen.d197da806d759f")}</CardTitle>
          <CardDescription>{ui("screen.e4acc6430dfda3")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-2 md:grid-cols-2">
            <div>
              <Label htmlFor="identity-name">{ui("screen.d44e9b3d3b31d3")}</Label>
              <Input
                id="identity-name"
                placeholder={ui("screen.edbb80aea815ae")}
                value={draft.name ?? ""}
                onChange={(e) => setDraft({ ...draft, name: e.target.value })}
              />
            </div>
            <div>
              <Label htmlFor="identity-kind">{ui("screen.ba40014ff496f6")}</Label>
              <Select value={draft.kind} onValueChange={(v) => setDraft({ ...draft, kind: v as Identity["kind"] })}>
                <SelectTrigger id="identity-kind">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="password">{ui("screen.164c7afc61581e")}</SelectItem>
                  <SelectItem value="cookie">Cookie</SelectItem>
                  <SelectItem value="token">Token / API Key</SelectItem>
                  <SelectItem value="header">{ui("screen.f3f7f9ecdd8d23")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          {draft.kind === "password" && (
            <div className="grid gap-2 md:grid-cols-2">
              <div>
                <Label htmlFor="identity-username">{ui("screen.311bb313fdeca6")}</Label>
                <Input id="identity-username" value={draft.username ?? ""} onChange={(e) => setDraft({ ...draft, username: e.target.value })} />
              </div>
              <div>
                <Label htmlFor="identity-password">{ui("screen.a621ab606db2a1")}</Label>
                <Input id="identity-password" type="password" value={draft.password ?? ""} onChange={(e) => setDraft({ ...draft, password: e.target.value })} />
              </div>
            </div>
          )}
          {draft.kind === "cookie" && (
            <div>
              <Label htmlFor="identity-cookie">{ui("screen.4b6c25b2047e36")}</Label>
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
              <Label htmlFor="identity-headers">{ui("screen.e5b1ae88e140ee")}</Label>
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
            <Label htmlFor="identity-notes">{ui("screen.daede9881787ab")}</Label>
            <Input id="identity-notes" placeholder={ui("screen.cb4f16e95e7df8")} value={draft.notes ?? ""} onChange={(e) => setDraft({ ...draft, notes: e.target.value })} />
          </div>
          <Button onClick={save}>{ui("screen.f9cca06953e647")}</Button>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{ui("screen.7fbacccdd73555", { v0: items.length })}</CardTitle>
          <CardDescription>{ui("screen.db2f4bccb89c8c")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {loading ? <p className="text-sm text-muted-foreground">{ui("screen.4927a53bcc886a")}</p> : null}
          {!loading && items.length === 0 ? <p className="text-sm text-muted-foreground">{ui("screen.b316d37aaf5dfa")}</p> : null}
          {items.map((it) => (
            <div key={it.id} className="flex items-center justify-between rounded border p-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="font-medium">{it.name}</span>
                  <code className="rounded bg-muted px-1.5 py-0.5 text-xs">{it.kind}</code>
                  {it.enabled ? <span className="text-xs text-green-600">{ui("screen.f4f0ead1116b5b")}</span> : <span className="text-xs text-muted-foreground">{ui("screen.7df5c456c765e4")}</span>}
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
                <Button size="sm" variant="destructive" onClick={() => remove(it)}>{ui("screen.2f9daa828907b9")}</Button>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  );
}
