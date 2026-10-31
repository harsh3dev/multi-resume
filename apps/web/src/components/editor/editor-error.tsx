"use client";

import { DocumentText, Warning2 } from "iconsax-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import { PageHeader, PageTitle } from "@/components/layout/page-header";
import { Button, buttonVariants } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { apiJSON } from "@/lib/api/client";

type Version = { id: string; name: string; isDefault: boolean };

/**
 * Shown when the editor can't load.
 *  - "missing": the version (or its resume) doesn't exist, e.g. it was deleted
 *    or the link is stale. If the resume still exists we offer its other
 *    versions, so the user is never left at a dead end.
 *  - "failed": network or server trouble; retrying may work.
 */
export function EditorError({
  kind,
  resumeId,
  onRetry,
}: {
  kind: "missing" | "failed";
  resumeId: string;
  onRetry: () => void;
}) {
  // undefined = still checking, null = the resume itself is gone.
  const [versions, setVersions] = useState<Version[] | null | undefined>(
    kind === "missing" ? undefined : null,
  );

  useEffect(() => {
    if (kind !== "missing") return;
    let live = true;
    apiJSON<{ variations: Version[] }>(`/api/resumes/${resumeId}/variations`)
      .then((r) => live && setVersions(r.variations.length > 0 ? r.variations : null))
      .catch(() => live && setVersions(null));
    return () => {
      live = false;
    };
  }, [kind, resumeId]);

  const dashboard = (
    <Link href="/dashboard" className={buttonVariants({ variant: "outline", size: "sm" })}>
      Back to dashboard
    </Link>
  );

  let title: string;
  let description: string;
  let actions: React.ReactNode;

  if (kind === "failed") {
    title = "Couldn't load the editor";
    description = "Something went wrong while loading this version. Check your connection and try again.";
    actions = (
      <div className="flex items-center gap-2">
        <Button size="sm" onClick={onRetry}>
          Try again
        </Button>
        {dashboard}
      </div>
    );
  } else if (versions === undefined) {
    title = "Version not found";
    description = "Checking whether the rest of this resume is still there…";
    actions = dashboard;
  } else if (versions === null) {
    title = "Resume not found";
    description = "This resume no longer exists. It may have been deleted, or the link is out of date.";
    actions = dashboard;
  } else {
    const sorted = [...versions].sort((a, b) => Number(b.isDefault) - Number(a.isDefault));
    title = "Version not found";
    description = "This version no longer exists. It may have been deleted, but the resume is still here.";
    actions = (
      <div className="flex flex-wrap items-center justify-center gap-2">
        {sorted.slice(0, 3).map((v, i) => (
          <Link
            key={v.id}
            href={`/resume/${resumeId}/${v.id}`}
            className={buttonVariants({ variant: i === 0 ? "default" : "outline", size: "sm" })}
          >
            Open {v.name}
          </Link>
        ))}
        {dashboard}
      </div>
    );
  }

  return (
    <>
      <PageHeader>
        <PageTitle icon={DocumentText}>Editor</PageTitle>
      </PageHeader>
      <div role="alert" className="flex h-full items-center justify-center p-6">
        <EmptyState tone="destructive" icon={Warning2} title={title} description={description} action={actions} />
      </div>
    </>
  );
}
