"use client";

import { Add, DocumentText, Home2 } from "@/icons";
import { useCallback, useEffect, useState } from "react";

import { ResumeRow } from "@/components/dashboard/resume-row";
import { UploadDialog } from "@/components/dashboard/upload-dialog";
import { PageHeader, PageTitle } from "@/components/layout/page-header";
import { Button } from "@/components/ui/components/actions/button";
import { EmptyState } from "@/components/ui/components/display/empty-state";
import { AppIcon } from "@/icons";
import { Skeleton } from "@/components/ui/components/display/skeleton";
import { apiJSON } from "@/lib/api/client";
import { createClient, supabaseConfigured } from "@/lib/supabase/client";

const GREETINGS = [
  (name: string) => `Hi ${name}!`,
  (name: string) => `Welcome back, ${name}!`,
  (name: string) => `Ready to build, ${name}?`,
  (name: string) => `Good to see you, ${name}!`,
  (name: string) => `Let's get to work, ${name}!`,
];

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
  const [greeting, setGreeting] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      const data = await apiJSON<{ resumes: ResumeSummary[] }>("/api/resumes");
      setResumes(data.resumes);
    } catch (e) {
      setError(e instanceof Error ? e.message : "failed to load");
    }
  }, []);

  useEffect(() => {
    if (!supabaseConfigured()) return;
    void load();
    let active = true;
    void createClient().auth.getUser().then(({ data }) => {
      if (!active || !data.user) return;
      const profile = data.user.user_metadata;
      const fullName = [profile?.full_name, profile?.name].find(
        (value): value is string => typeof value === "string" && value.trim().length > 0,
      );
      const firstName = fullName?.trim().split(/\s+/)[0] ?? "there";
      const day = Math.floor(Date.now() / 86_400_000);
      setGreeting(GREETINGS[day % GREETINGS.length](firstName));
    }).catch(() => {
      if (active) setGreeting("Welcome back!");
    });
    return () => { active = false; };
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
        <PageTitle icon={Home2}>{greeting ?? "Dashboard"}</PageTitle>
      </PageHeader>
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold tracking-tight">Your resumes</h2>
        <Button onClick={() => setUploadOpen(true)}>
          <AppIcon icon={Add} />
          New resume
        </Button>
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
