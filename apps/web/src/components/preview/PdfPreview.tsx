"use client";

import { ArrowLeft2, ArrowRight2, SearchZoomIn1, SearchZoomOut1 } from "iconsax-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Document, Page, pdfjs } from "react-pdf";
import "react-pdf/dist/Page/TextLayer.css";
import "react-pdf/dist/Page/AnnotationLayer.css";

import { Button } from "@/components/ui/button";
import { AppIcon } from "@/components/ui/icon";
import { Input } from "@/components/ui/input";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useEditor } from "@/lib/store/editor";

// The worker version must match react-pdf's pdfjs-dist exactly; the root
// dependency is pinned to the same version for this URL to resolve correctly.
pdfjs.GlobalWorkerOptions.workerSrc = new URL(
  "pdfjs-dist/build/pdf.worker.min.mjs",
  import.meta.url,
).toString();

const ZOOM_MIN = 50;
const ZOOM_MAX = 300;
const ZOOM_STEP = 10;

function ToolButton({
  label,
  onClick,
  disabled,
  children,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button variant="ghost" size="icon-sm" aria-label={label} disabled={disabled} onClick={onClick}>
            {children}
          </Button>
        }
      />
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

// PdfPreview renders the last good PDF with pdf.js text and annotation
// layers. A new compile is loaded off-screen and swapped in once ready, so
// the old preview stays visible while regenerating.
export function PdfPreview() {
  const { lastGoodPdfUrl, compileError } = useEditor();
  // `src` is the storage URL a PDF came from; `blob` is the local object URL
  // handed to pdf.js. We fetch the file ourselves so a network failure is an
  // error we handle, not an unhandled rejection from inside pdf.js.
  const [shown, setShown] = useState<{ src: string; blob: string } | null>(null);
  const blobRef = useRef<string | null>(null);
  // URL whose load failed; never preloaded again until a new one arrives.
  const [failedUrl, setFailedUrl] = useState<string | null>(null);
  const autoRetried = useRef<string | null>(null);
  const [numPages, setNumPages] = useState(0);
  const [page, setPage] = useState(1);
  const [pageInput, setPageInput] = useState("1");
  const [zoom, setZoom] = useState(100); // percent of the fit-to-width size
  const [width, setWidth] = useState(0);
  const wrapRef = useRef<HTMLDivElement>(null);
  // pdf.js paints on a <canvas>, which can't resolve `var(--pdf-paper)`;
  // read the token's computed value once instead. "white" is only the
  // pre-mount fallback.
  const [paper, setPaper] = useState("white");
  useEffect(() => {
    const v = getComputedStyle(document.documentElement).getPropertyValue("--pdf-paper").trim();
    if (v) setPaper(v);
  }, []);
  const pageRefs = useRef<Map<number, HTMLDivElement>>(new Map());

  // Fit pages to the column width.
  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const e of entries) setWidth(Math.floor(e.contentRect.width));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const setCurrent = useCallback((n: number) => {
    setPage(n);
    setPageInput(String(n));
  }, []);

  const goTo = useCallback(
    (n: number) => {
      const clamped = Math.min(Math.max(1, Math.round(n) || 1), Math.max(1, numPages));
      setCurrent(clamped);
      pageRefs.current.get(clamped)?.scrollIntoView({ block: "start" });
    },
    [numPages, setCurrent],
  );

  // Track the page nearest the top while scrolling.
  function onScroll() {
    const wrap = wrapRef.current;
    if (!wrap) return;
    const top = wrap.getBoundingClientRect().top;
    let best = 1;
    let bestDist = Infinity;
    pageRefs.current.forEach((el, n) => {
      const dist = Math.abs(el.getBoundingClientRect().top - top);
      if (dist < bestDist) {
        bestDist = dist;
        best = n;
      }
    });
    if (best !== page) setCurrent(best);
  }

  const onShownLoad = useCallback(
    (pdf: { numPages: number }) => {
      setNumPages(pdf.numPages);
      setPage((p) => Math.min(p, pdf.numPages));
    },
    [],
  );

  // Resizing the panes (or zooming) changes the target width continuously.
  // Re-rendering canvases on every tick flickers, so pages stay rendered at
  // `renderWidth` and are CSS-scaled to the live width; the canvases are
  // re-rendered once, after the width has settled.
  const liveWidth = Math.max(0, Math.floor(((width - 16) * zoom) / 100));
  const [renderWidth, setRenderWidth] = useState(0);
  useEffect(() => {
    if (liveWidth === 0) return;
    if (renderWidth === 0) {
      setRenderWidth(liveWidth);
      return;
    }
    const t = setTimeout(() => setRenderWidth(liveWidth), 250);
    return () => clearTimeout(t);
  }, [liveWidth, renderWidth]);
  const pageWidth = renderWidth;
  const cssScale = renderWidth > 0 && liveWidth > 0 ? liveWidth / renderWidth : 1;
  const failed = !!lastGoodPdfUrl && lastGoodPdfUrl === failedUrl;
  const loading = !!lastGoodPdfUrl && lastGoodPdfUrl !== shown?.src && !failed;

  // Fetch each new compile; the old preview stays visible until it is ready.
  // A failure is most often an expired presigned URL: refetch the draft once
  // for a fresh link, then surface the error with a manual retry.
  useEffect(() => {
    const url = lastGoodPdfUrl;
    if (!url || url === failedUrl || url === shown?.src) return;
    const ctrl = new AbortController();
    (async () => {
      try {
        const res = await fetch(url, { signal: ctrl.signal });
        if (!res.ok) throw new Error(`status ${res.status}`);
        const blob = URL.createObjectURL(new Blob([await res.blob()], { type: "application/pdf" }));
        if (ctrl.signal.aborted) {
          URL.revokeObjectURL(blob);
          return;
        }
        const previous = blobRef.current;
        blobRef.current = blob;
        setShown({ src: url, blob });
        // Let the old Document unmount before its object URL goes away.
        if (previous) setTimeout(() => URL.revokeObjectURL(previous), 5000);
      } catch {
        if (ctrl.signal.aborted) return;
        setFailedUrl(url);
        if (autoRetried.current !== url) {
          autoRetried.current = url;
          void useEditor.getState().refetch().catch(() => {});
        }
      }
    })();
    return () => ctrl.abort();
  }, [lastGoodPdfUrl, failedUrl, shown?.src]);

  useEffect(
    () => () => {
      if (blobRef.current) URL.revokeObjectURL(blobRef.current);
    },
    [],
  );

  // A draft with no PDF (yet) must not keep showing the previous one.
  useEffect(() => {
    if (lastGoodPdfUrl) return;
    if (blobRef.current) URL.revokeObjectURL(blobRef.current);
    blobRef.current = null;
    setShown(null);
    setFailedUrl(null);
    setNumPages(0);
    setPage(1);
    setPageInput("1");
  }, [lastGoodPdfUrl]);
  const hasPdf = shown !== null;

  return (
    <div className="relative flex h-full flex-col bg-muted/30">
      <div className="flex h-10 shrink-0 items-center gap-1 border-b border-border bg-background px-2">
        <ToolButton label="Previous page" disabled={!hasPdf || page <= 1} onClick={() => goTo(page - 1)}>
          <AppIcon icon={ArrowLeft2} />
        </ToolButton>
        <form
          className="flex items-center gap-1.5 text-xs text-muted-foreground"
          onSubmit={(e) => {
            e.preventDefault();
            goTo(Number(pageInput));
          }}
        >
          <Input
            aria-label="Page number"
            inputMode="numeric"
            value={pageInput}
            disabled={!hasPdf}
            onChange={(e) => setPageInput(e.target.value.replace(/\D/g, ""))}
            onBlur={() => goTo(Number(pageInput))}
            className="h-7 w-10 px-1 text-center text-xs"
          />
          <span aria-label={`of ${numPages} pages`}>/ {numPages || "–"}</span>
        </form>
        <ToolButton label="Next page" disabled={!hasPdf || page >= numPages} onClick={() => goTo(page + 1)}>
          <AppIcon icon={ArrowRight2} />
        </ToolButton>
        <div className="ml-auto flex items-center gap-1">
          <ToolButton
            label="Zoom out"
            disabled={!hasPdf || zoom <= ZOOM_MIN}
            onClick={() => setZoom((z) => Math.max(ZOOM_MIN, z - ZOOM_STEP))}
          >
            <AppIcon icon={SearchZoomOut1} />
          </ToolButton>
          <span className="w-10 text-center text-xs tabular-nums text-muted-foreground" aria-live="polite">
            {zoom}%
          </span>
          <ToolButton
            label="Zoom in"
            disabled={!hasPdf || zoom >= ZOOM_MAX}
            onClick={() => setZoom((z) => Math.min(ZOOM_MAX, z + ZOOM_STEP))}
          >
            <AppIcon icon={SearchZoomIn1} />
          </ToolButton>
        </div>
      </div>

      {compileError ? (
        <div className="m-2 rounded-lg border border-destructive/40 bg-destructive/10 p-2 text-xs text-destructive">
          Compile error: {compileError}
        </div>
      ) : null}

      {failed ? (
        <div
          role="alert"
          className="m-2 flex items-center justify-between gap-2 rounded-lg border border-destructive/40 bg-destructive/10 p-2 text-xs text-destructive"
        >
          Couldn&apos;t load the PDF preview.
          <Button
            size="xs"
            variant="outline"
            onClick={() => {
              autoRetried.current = null;
              setFailedUrl(null);
              void useEditor.getState().refetch().catch(() => {});
            }}
          >
            Retry
          </Button>
        </div>
      ) : null}

      <div ref={wrapRef} onScroll={onScroll} className="relative min-h-0 flex-1 overflow-auto py-2">
        {hasPdf ? (
          <div style={cssScale === 1 ? undefined : { zoom: cssScale }}>
          <Document
            key={shown.blob}
            file={shown.blob}
            onLoadSuccess={onShownLoad}
            onLoadError={() => {}}
            onSourceError={() => {}}
            loading={null}
          >
            {pageWidth > 0
              ? Array.from({ length: numPages }, (_, i) => (
                  <div
                    key={`page_${i + 1}`}
                    ref={(el) => {
                      if (el) pageRefs.current.set(i + 1, el);
                      else pageRefs.current.delete(i + 1);
                    }}
                    className="mx-auto mb-4 w-fit"
                  >
                    <Page
                      pageNumber={i + 1}
                      width={pageWidth}
                      canvasBackground={paper}
                      className="shadow-md"
                    />
                  </div>
                ))
              : null}
          </Document>
          </div>
        ) : loading || failed || !lastGoodPdfUrl ? (
          lastGoodPdfUrl ? (
            failed ? null : <PreviewSkeleton />
          ) : (
            <p className="p-8 text-center text-sm text-muted-foreground">
              The PDF preview appears here after the first compile.
            </p>
          )
        ) : null}
      </div>
    </div>
  );
}

// Shaped like a resume page (US Letter ratio) so the layout doesn't jump when
// the real PDF arrives. Bars are only just darker than the paper.
const SKELETON_SECTIONS = [
  { title: "w-1/4", lines: ["w-full", "w-[92%]", "w-[85%]"] },
  { title: "w-1/5", lines: ["w-full", "w-[88%]", "w-full", "w-[70%]"] },
  { title: "w-1/3", lines: ["w-[95%]", "w-full", "w-[80%]"] },
  { title: "w-1/4", lines: ["w-full", "w-[90%]", "w-[60%]"] },
] as const;

function Bar({ className }: { className: string }) {
  return <div className={`h-2.5 animate-pulse rounded-sm bg-pdf-skeleton motion-reduce:animate-none ${className}`} />;
}

function PreviewSkeleton() {
  return (
    <div
      aria-busy
      aria-label="Loading preview"
      className="mx-auto mb-4 flex aspect-[8.5/11] w-[min(100%-1rem,44rem)] flex-col gap-7 rounded-sm bg-pdf-paper px-12 py-14 shadow-md"
    >
      <div className="flex flex-col items-center gap-2.5">
        <div className="h-5 w-1/3 animate-pulse rounded-sm bg-pdf-skeleton motion-reduce:animate-none" />
        <Bar className="w-1/2" />
      </div>
      {SKELETON_SECTIONS.map((section, i) => (
        <div key={i} className="flex flex-col gap-2.5">
          <div className={`h-3.5 animate-pulse rounded-sm bg-pdf-skeleton motion-reduce:animate-none ${section.title}`} />
          {section.lines.map((w, j) => (
            <Bar key={j} className={w} />
          ))}
        </div>
      ))}
    </div>
  );
}
