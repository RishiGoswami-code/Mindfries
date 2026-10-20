import type { Metadata } from "next";
import { fetchV2Products, v2Ready } from "@/lib/v2";
import { V2AssignView } from "@/components/admin/V2AssignView";

export const metadata: Metadata = { title: "Assign V2 task" };

export const dynamic = "force-dynamic";

export default async function V2Page() {
  const ready = v2Ready();
  const products = ready ? await fetchV2Products() : [];
  return (
    <div className="space-y-6">
      <V2AssignView products={products} engineReady={ready} />
    </div>
  );
}
