"use client";

import { use, useEffect, useState } from "react";

import { SectionList } from "@/components/editor/SectionList";
import { StylePanel } from "@/components/editor/StylePanel";
import { EditorError } from "@/components/editor/editor-error";
import { EditorHeader } from "@/components/editor/editor-header";
import { Workspace } from "@/components/editor/workspace";
import { ConversionIndicator } from "@/components/preview/conversion-indicator";
import { PdfPreview } from "@/components/preview/PdfPreview";
import { SectionToggles } from "@/components/editor/section-toggles";
import { SourceTab } from "@/components/source/SourceTab";
import { TailorPanel } from "@/components/tailor/TailorPanel";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { VersionList } from "@/components/versions/VersionList";
import { apiJSON } from "@/lib/api/client";
import { useEditor } from "@/lib/store/editor";
import { collapsibleIds, useSectionsUi } from "@/lib/store/sections-ui";
import type { Resume } from "@/types/generated";

const TABS = ["fields", "style", "source", "versions", "tailor"] as const;
type Tab = (typeof TABS)[number];

export default function EditorPage({
  params,
}: {
  params: Promise<{ resumeId: string; variantId: string }>;
}) {
  const { resumeId, variantId } = use(params);
  const { tree, loadDraft, setPdf } = useEditor();
  const [tab, setTab] = useState<Tab>("fields");
  const [error, setError] = useState<"missing" | "failed" | null>(null);
  const [attempt, setAttempt] = useState(0); // bump to retry the load
  const [variationName, setVariationName] = useState("");
  const [resumeTitle, setResumeTitle] = useState("");
  const [isDefault, setIsDefault] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setError(null);
    // The store is module-level and outlives navigation. Without this the
    // previous resume's tree and PDF stay on screen until (and unless) the new
    // draft loads, and late responses for the old draft could land on the new.
    useEditor.getState().reset();
    useSectionsUi.getState().setIds([]);
    (async () => {
      try {
        const res = await apiJSON<{
          draftId: string;
          variationName: string;
          resumeTitle?: string;
          isDefault?: boolean;
          tree: Resume;
          revision: number;
          sourceTex: string;
          pdfUrl?: string;
        }>(`/api/resumes/${resumeId}/variations/${variantId}`);
        if (!cancelled) {
          loadDraft(res.draftId, res.tree, res.revision, res.sourceTex);
          // Register before the list mounts so sections start open without a flash.
          useSectionsUi.getState().setIds(collapsibleIds(res.tree.header, res.tree.sections));
          setVariationName(res.variationName);
          setResumeTitle(res.resumeTitle ?? "");
          setIsDefault(res.isDefault ?? false);
          setPdf(res.pdfUrl ?? null);
        }
      } catch (e) {
        // The API answers 404 "not found" for a missing or foreign version.
        if (!cancelled) setError(e instanceof Error && e.message === "not found" ? "missing" : "failed");
      }
    })();
    return () => {
      cancelled = true;
    };
    // setPdf identity is stable in the zustand store; loadDraft is the trigger.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resumeId, variantId, loadDraft, attempt]);

  if (error) {
    return <EditorError kind={error} resumeId={resumeId} onRetry={() => setAttempt((n) => n + 1)} />;
  }
  if (!tree) {
    return <div className="p-8 text-sm text-muted-foreground">Loading editor…</div>;
  }

  const edit = (
    <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)} className="h-full gap-0">
      <div className="flex shrink-0 items-center justify-between gap-2 border-b border-border px-3 py-2">
        <TabsList>
          {TABS.map((name) => (
            <TabsTrigger key={name} value={name} className="px-3 capitalize">
              {name}
            </TabsTrigger>
          ))}
        </TabsList>
        {tab === "fields" ? <SectionToggles /> : null}
      </div>
      <div className="min-h-0 flex-1 overflow-auto">
        <TabsContent value="fields">
          <SectionList sections={tree.sections} header={tree.header} />
        </TabsContent>
        <TabsContent value="style">
          <StylePanel styles={tree.styles} />
        </TabsContent>
        {/* SourceTab stays mounted (hidden) so tab switches keep edits. */}
        <TabsContent value="source" keepMounted className="h-full data-hidden:hidden">
          <div className="flex h-full flex-col">
            <SourceTab />
          </div>
        </TabsContent>
        <TabsContent value="versions">
          <VersionList
            resumeId={resumeId}
            variantId={variantId}
            onRenamed={(id, name) => id === variantId && setVariationName(name)}
          />
        </TabsContent>
        <TabsContent value="tailor">
          <TailorPanel />
        </TabsContent>
      </div>
    </Tabs>
  );

  return (
    <div className="flex h-full flex-col">
      <EditorHeader
        resumeId={resumeId}
        variantId={variantId}
        resumeTitle={resumeTitle}
        variationName={variationName}
        isDefault={isDefault}
        onResumeRenamed={setResumeTitle}
        onVersionRenamed={setVariationName}
      />
      <Workspace edit={edit} preview={<PdfPreview />} divider={<ConversionIndicator />} />
    </div>
  );
}
