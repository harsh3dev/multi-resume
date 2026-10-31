"use client";

import { useCallback, useEffect, useState } from "react";

import { PageHeader, PageTitle } from "@/components/layout/page-header";
import { ResumeRow } from "@/components/dashboard/resume-row";
import { UploadDialog } from "@/components/dashboard/upload-dialog";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { apiJSON } from "@/lib/api/client";
import { supabaseConfigured } from "@/lib/supabase/client";
import { DocumentText, Home2 } from "iconsax-react";

export type ResumeSummary = {
  id: string;
  title: string;
  compiler: string;
  family: string | null;
  createdAt: string;
};

export default function DashboardPage() {
  const [resumes, setResumes] = useState<ResumeSummary[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [uploadOpen, setUploadOpen] = useState(false);

  const load = useCallback(async () => {
    try {
      const data = await apiJSON<{ resumes: ResumeSummary[] }>("/api/resumes");
      setResumes(data.resumes);
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load");
    }
  }, []);

  useEffect(() => {
    if (supabaseConfigured()) {
      void load();
    }
  }, [load]);

  if (!supabaseConfigured()) {
    return (
      <div className="mx-auto max-w-3xl p-6">
        <PageHeader>
          <PageTitle icon={Home2}>Dashboard</PageTitle>
        </PageHeader>
        <p className="text-sm text-muted-foreground">
          Supabase is not configured. Set NEXT_PUBLIC_SUPABASE_URL and
          NEXT_PUBLIC_SUPABASE_ANON_KEY to enable sign-in and uploads.
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl p-6">
      <PageHeader>
        <PageTitle icon={Home2}>Dashboard</PageTitle>
      </PageHeader>
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold tracking-tight">Your resumes</h2>
        <Button onClick={() => setUploadOpen(true)}>Upload resume</Button>
      </div>

      {error ? <p className="mt-3 text-sm text-destructive">{error}</p> : null}

      <div className="mt-6">
        {resumes === null ? (
          <div className="flex flex-col gap-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : resumes.length === 0 ? (
          <EmptyState
            icon={DocumentText}
            title="No resumes yet"
            description="Upload your LaTeX resume to start editing. Every upload gets an Original variation you can tailor per job."
            action={<Button size="sm" onClick={() => setUploadOpen(true)}>Upload resume</Button>}
          />
        ) : (
          <ul className="flex flex-col gap-2">
            {resumes.map((r) => (
              <ResumeRow key={r.id} resume={r} onChanged={() => void load()} />
            ))}
          </ul>
        )}
      </div>

      <UploadDialog
        open={uploadOpen}
        onOpenChange={setUploadOpen}
        onUploaded={() => void load()}
      />
    </div>
  );
}
