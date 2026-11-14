"use client";

import Link from "next/link";

import { Button } from "@/components/ui/button";
import { useUITranslation } from "@/i18n/use-ui-translation";

export default function NotFound() {
  const { ui } = useUITranslation();
  return (
    <div className="flex h-dvh flex-col items-center justify-center space-y-2 text-center">
      <h1 className="font-semibold text-2xl">{ui("screen.e75737c1ab90e1")}</h1>
      <p className="text-muted-foreground">{ui("screen.39a4262a84700c")}</p>
      <Link prefetch={false} replace href="/function/tasks">
        <Button variant="outline">{ui("screen.f2e60784b2030a")}</Button>
      </Link>
    </div>
  );
}
