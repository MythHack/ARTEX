"use client";

import { useTranslation } from "react-i18next";

import { type StatusDomain, statusMeta, type Tone, toneClasses, toneDot } from "@/lib/status";
import { cn } from "@/lib/utils";

export function StatusBadge({
  domain,
  value,
  dot = false,
  stopReason,
  className,
  title,
}: {
  domain: StatusDomain;
  value: string;
  dot?: boolean;
  stopReason?: string;
  className?: string;
  /** 悬停提示。用于把「为什么是这个状态」摆在状态徽章上（如意图的中断原因）。 */
  title?: string;
}) {
  const { t } = useTranslation();
  const meta = statusMeta(domain, value, t);
  return (
    <span
      title={title}
      className={cn(
        "inline-flex items-center gap-1.5 whitespace-nowrap rounded-md border px-2 py-0.5 font-medium text-xs",
        toneClasses[meta.tone],
        className,
      )}
    >
      {dot && <span className={cn("size-1.5 rounded-full", toneDot[meta.tone])} />}
      {domain === "task" && value === "timeout" && stopReason === "no_result"
        ? t("taskUI.noResultStopped")
        : meta.label}
    </span>
  );
}

export function ToneDot({ tone }: { tone: Tone }) {
  return <span className={cn("size-2 rounded-full", toneDot[tone])} />;
}
