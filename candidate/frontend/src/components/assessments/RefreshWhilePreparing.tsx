"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import type { Assessment } from "@/lib/dashboard/data";

/**
 * While any invitation's task is still being written for the candidate,
 * the page asks for its data again every ten seconds, so Start appears on
 * its own once the task is ready. Renders nothing.
 */
export function RefreshWhilePreparing({ items }: { items: Assessment[] }) {
  const router = useRouter();
  const preparing = items.some((a) => a.status === "invited" && (a.taskStatus === "pending" || a.taskStatus === "generating"));
  useEffect(() => {
    if (!preparing) return;
    const timer = setInterval(() => router.refresh(), 10_000);
    return () => clearInterval(timer);
  }, [preparing, router]);
  return null;
}
