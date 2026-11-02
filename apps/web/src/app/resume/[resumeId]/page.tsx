"use client";

import { Warning2 } from "@/icons";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { use, useEffect, useState } from "react";

import { buttonVariants } from "@/components/ui/components/actions/button";
import { EmptyState } from "@/components/ui/components/display/empty-state";
import { apiJSON } from "@/lib/api/client";

/**
 * Legacy route: /resume/[id] predates variations. Resolve the resume's
 * default ("Original") variation and forward to the canonical editor route.
 */
export default function LegacyResumeRedirect({
  params,
}: {
  params: Promise<{ resumeId: string }>;
}) {
  const { resumeId } = use(params);
  const router = useRouter();
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    let cancelled = false;
    apiJSON<{ variations: { id: string; isDefault: boolean }[] }>(
      `/api/resumes/${resumeId}/variations`,
    )
      .then((res) => {
        if (cancelled) return;
        const target =
          res.variations.find((v) => v.isDefault) ?? res.variations[0];
        if (target) {
          router.replace(`/resume/${resumeId}/${target.id}`);
        } else {
          setFailed(true);
        }
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [resumeId, router]);

  if (failed) {
    return (
      <main role="alert" className="flex min-h-svh items-center justify-center p-6">
        <EmptyState
          tone="destructive"
          icon={Warning2}
          title="Resume not found"
          description="This resume no longer exists. It may have been deleted, or the link is out of date."
          action={
            <Link href="/dashboard" className={buttonVariants({ variant: "outline", size: "sm" })}>
              Back to dashboard
            </Link>
          }
        />
      </main>
    );
  }
  return <main className="p-8 text-sm text-muted-foreground">Opening editor…</main>;
}
