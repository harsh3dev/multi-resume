"use client";

import { CloseCircle, DocumentUpload } from "@/icons";
import { useRouter } from "next/navigation";
import { useRef, useState, type DragEvent } from "react";

import { Button } from "@/components/ui/components/actions/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogIcon,
  DialogTitle,
} from "@/components/ui/components/overlays/dialog";
import { AppIcon } from "@/icons";
import { IconButton } from "@/components/ui/components/actions/icon-button";
import { apiFetch } from "@/lib/api/client";
import { toast } from "@/lib/store/toast";
import { cn } from "@/lib/utils/utils";

// Matches the backend limit (maxTexSize), so an oversized file is rejected
// here instead of after the upload.
const MAX_BYTES = 2 * 1024 * 1024;

function validate(file: File): string | null {
  if (!file.name.toLowerCase().endsWith(".tex")) return "Only .tex files are supported.";
  if (file.size === 0) return "That file is empty.";
  if (file.size > MAX_BYTES) return "That file is larger than 2 MB.";
  return null;
}

function formatSize(bytes: number): string {
  return bytes < 1024 ? `${bytes} B` : `${(bytes / 1024).toFixed(1)} KB`;
}

const hasFiles = (e: DragEvent) => Array.from(e.dataTransfer.types).includes("Files");

type UploadResponse = {
  resumeId: string;
  variationId: string;
  draftId: string;
  report: { sections: number; entries: number; bullets: number; readOnlyRegions: number };
  missingImages: string[];
};

export function UploadDialog({
  open,
  onOpenChange,
  onUploaded,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUploaded: () => void;
}) {
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [dragging, setDragging] = useState(false);
  // dragenter/dragleave also fire for the zone's children; count them so the
  // highlight doesn't flicker as the pointer crosses the icon and text.
  const dragDepth = useRef(0);

  function reset() {
    setFile(null);
    setError(null);
    setUploading(false);
    setDragging(false);
    dragDepth.current = 0;
    if (inputRef.current) inputRef.current.value = "";
  }

  // One entry point for both the file picker and a drop.
  function pick(files: FileList | File[] | null | undefined) {
    const list = files ? Array.from(files) : [];
    if (list.length === 0) return;
    if (list.length > 1) {
      setError("Drop one .tex file at a time.");
      return;
    }
    const problem = validate(list[0]);
    if (problem) {
      setError(problem);
      return;
    }
    setError(null);
    setFile(list[0]);
  }

  function clearFile() {
    setFile(null);
    setError(null);
    if (inputRef.current) inputRef.current.value = "";
  }

  const dropHandlers = {
    onDragEnter: (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      dragDepth.current += 1;
      setDragging(true);
    },
    onDragOver: (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault(); // required, or the browser refuses the drop
      e.dataTransfer.dropEffect = "copy";
    },
    onDragLeave: (e: DragEvent) => {
      if (!hasFiles(e)) return;
      dragDepth.current = Math.max(0, dragDepth.current - 1);
      if (dragDepth.current === 0) setDragging(false);
    },
    onDrop: (e: DragEvent) => {
      if (!hasFiles(e)) return;
      e.preventDefault();
      dragDepth.current = 0;
      setDragging(false);
      if (!uploading) pick(e.dataTransfer.files);
    },
  };

  async function upload() {
    if (!file) return;
    setUploading(true);
    setError(null);
    try {
      const form = new FormData();
      form.append("file", file);
      const res = await apiFetch("/api/resumes", { method: "POST", body: form });
      const body = (await res.json().catch(() => null)) as
        | (Partial<UploadResponse> & { error?: string })
        | null;
      if (!res.ok || !body?.report) {
        throw new Error(body?.error ?? "upload failed");
      }
      const r = body.report;
      const detail =
        `${r.sections} sections, ${r.entries} entries, ${r.bullets} bullets.` +
        (r.readOnlyRegions > 0 ? ` ${r.readOnlyRegions} read-only region(s).` : "") +
        (body.missingImages && body.missingImages.length > 0
          ? ` Missing images: ${body.missingImages.join(", ")}`
          : "");
      toast.success("Resume uploaded", detail);
      onOpenChange(false);
      reset();
      onUploaded();
      // Straight into the editor on the Original version (which opens on the
      // Images tab when the template needs pictures).
      if (body.resumeId && body.variationId) {
        router.push(`/resume/${body.resumeId}/${body.variationId}`);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "upload failed");
      toast.error("Upload failed", e instanceof Error ? e.message : undefined);
    } finally {
      setUploading(false);
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        onOpenChange(next);
        if (!next) reset();
      }}
    >
      <DialogContent
        // A file dropped on the dialog but outside the zone would otherwise make the
        // browser navigate to it; treat the whole dialog as a drop target instead.
        {...dropHandlers}
      >
        <DialogHeader className="flex-row items-center gap-3">
          <DialogIcon>
            <AppIcon icon={DocumentUpload} size="lg" variant="Bulk" />
          </DialogIcon>
          <div className="flex flex-col gap-1">
            <DialogTitle>Upload resume</DialogTitle>
            <DialogDescription>
              A .tex file using a supported template. It becomes the Original variation.
            </DialogDescription>
          </div>
        </DialogHeader>

        <input
          ref={inputRef}
          type="file"
          accept=".tex"
          className="hidden"
          onChange={(e) => {
            pick(e.target.files);
            e.target.value = ""; // allow re-picking the same file after clearing
          }}
        />
        <button
          type="button"
          disabled={uploading}
          onClick={() => inputRef.current?.click()}
          {...dropHandlers}
          className={cn(
            "flex flex-col items-center gap-2 rounded-lg border border-dashed p-6 text-center transition-colors outline-none focus-visible:border-foreground/40",
            dragging
              ? "border-primary bg-primary/10"
              : "border-input hover:border-foreground/30 hover:bg-muted/50",
            error && !dragging && "border-destructive/60",
          )}
        >
          <AppIcon
            icon={DocumentUpload}
            size="lg"
            variant={dragging ? "Bulk" : "Linear"}
            className={dragging ? "text-primary" : "text-muted-foreground"}
          />
          <span className={cn("text-sm", dragging ? "font-medium text-primary" : "text-muted-foreground")}>
            {dragging ? "Drop your .tex file to add it" : "Drag a .tex file here, or click to choose one"}
          </span>
          <span className="text-xs text-muted-foreground/80">.tex only, up to 2 MB</span>
        </button>

        {file ? (
          <div className="flex items-center gap-2 rounded-lg bg-muted px-3 py-2 text-sm">
            <AppIcon icon={DocumentUpload} className="text-muted-foreground" />
            <span className="min-w-0 flex-1 truncate font-medium">{file.name}</span>
            <span className="text-xs text-muted-foreground">{formatSize(file.size)}</span>
            <IconButton icon={CloseCircle} label="Remove file" size="icon-xs" disabled={uploading} onClick={clearFile} />
          </div>
        ) : null}
        {error ? (
          <p role="alert" className="text-xs text-destructive">
            {error}
          </p>
        ) : null}

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button loading={uploading} disabled={!file} onClick={() => void upload()}>
            Upload
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
