"use client";

import { Gallery, GalleryAdd } from "@/icons";
import { useRef, useState } from "react";

import { Badge } from "@/components/ui/components/display/badge";
import { Button } from "@/components/ui/components/actions/button";
import { AppIcon } from "@/icons";
import { apiFetch } from "@/lib/api/client";
import { useEditor } from "@/lib/store/editor";
import { toast } from "@/lib/store/toast";
import type { AssetRef } from "@/types/generated";

const MAX_BYTES = 5 * 1024 * 1024;

/**
 * Every image the .tex references. Until the user uploads their own, the
 * preview shows a grey placeholder so the template still compiles.
 */
export function ImagesPanel({ resumeId, assets }: { resumeId: string; assets: AssetRef[] }) {
  // One row per distinct path: a template can reference the same file twice.
  const unique = assets.filter((a, i) => assets.findIndex((b) => b.path === a.path) === i);
  const missing = unique.filter((a) => !a.uploaded).length;

  return (
    <div className="flex flex-col gap-3 p-3">
      <p className="text-xs text-muted-foreground">
        {missing > 0
          ? `${missing} of ${unique.length} image${unique.length === 1 ? "" : "s"} still use a placeholder. Upload yours and the preview updates.`
          : "All images are uploaded. Upload again to replace one."}
      </p>
      <ul className="flex flex-col gap-2">
        {unique.map((a) => (
          <ImageRow key={a.path} resumeId={resumeId} asset={a} />
        ))}
      </ul>
    </div>
  );
}

function ImageRow({ resumeId, asset }: { resumeId: string; asset: AssetRef }) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  async function upload(file: File | undefined) {
    if (!file) return;
    if (!/^image\/(png|jpe?g)$/.test(file.type)) {
      toast.error("Unsupported image", "Use a PNG or JPEG.");
      return;
    }
    if (file.size > MAX_BYTES) {
      toast.error("Image too large", "The limit is 5 MB.");
      return;
    }
    const { draftId } = useEditor.getState();
    setBusy(true);
    try {
      const form = new FormData();
      form.append("file", file);
      form.append("path", asset.path);
      form.append("draftId", draftId);
      const res = await apiFetch(`/api/resumes/${resumeId}/assets`, { method: "POST", body: form });
      if (!res.ok) {
        const body = (await res.json().catch(() => null)) as { error?: string } | null;
        throw new Error(body?.error ?? "upload failed");
      }
      // Pull the draft back for the refreshed `uploaded` flags and the stale
      // marker; the preview itself updates when the user presses Save.
      await useEditor.getState().refetch();
      toast.success("Image uploaded", "Click Sync PDF to update the preview.");
    } catch (e) {
      toast.error("Could not upload image", e instanceof Error ? e.message : undefined);
    } finally {
      setBusy(false);
      if (inputRef.current) inputRef.current.value = "";
    }
  }

  return (
    <li className="flex items-center gap-3 rounded-lg border border-border bg-card p-3">
      <AppIcon icon={Gallery} variant="Bulk" size="lg" className="text-muted-foreground" />
      <div className="min-w-0 flex-1">
        <p className="truncate font-mono text-xs">{asset.path}</p>
        <Badge variant={asset.uploaded ? "secondary" : "outline"} className="mt-1">
          {asset.uploaded ? "Uploaded" : "Placeholder"}
        </Badge>
      </div>
      <input
        ref={inputRef}
        type="file"
        accept="image/png,image/jpeg"
        className="hidden"
        onChange={(e) => void upload(e.target.files?.[0])}
      />
      <Button variant="outline" size="sm" loading={busy} onClick={() => inputRef.current?.click()}>
        <AppIcon icon={GalleryAdd} />
        {asset.uploaded ? "Replace" : "Upload"}
      </Button>
    </li>
  );
}
