"use client";

import { ArrowDown2 } from "iconsax-react";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { CheckMark } from "@/components/ui/check-mark";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AppIcon } from "@/components/ui/icon";
import { apiJSON } from "@/lib/api/client";

interface Option {
  id: string;
  label: string;
  hint?: string;
}

/**
 * A breadcrumb crumb that opens a menu of its siblings. Options load when the
 * menu opens (so they are current), and the active one is ticked.
 */
function CrumbMenu({
  label,
  menuLabel,
  activeId,
  load,
  onSelect,
}: {
  label: string;
  menuLabel: string;
  activeId: string;
  load: () => Promise<Option[]>;
  onSelect: (id: string) => void;
}) {
  const [options, setOptions] = useState<Option[] | null>(null);
  const [failed, setFailed] = useState(false);

  async function refresh() {
    setFailed(false);
    try {
      setOptions(await load());
    } catch {
      setFailed(true);
      setOptions((cur) => cur ?? []);
    }
  }

  return (
    <DropdownMenu onOpenChange={(open) => open && void refresh()}>
      <DropdownMenuTrigger
        aria-label={`${menuLabel}: ${label}`}
        className="flex min-w-0 max-w-56 items-center gap-1 rounded-md px-1.5 py-0.5 text-sm outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-2 focus-visible:ring-ring/50 aria-expanded:bg-accent"
      >
        <span className="truncate">{label}</span>
        <AppIcon icon={ArrowDown2} size={12} className="text-muted-foreground" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-64">
        {options === null ? (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">Loading…</p>
        ) : failed ? (
          <p className="px-2 py-1.5 text-xs text-destructive">Couldn&apos;t load the list.</p>
        ) : options.length === 0 ? (
          <p className="px-2 py-1.5 text-xs text-muted-foreground">Nothing else here.</p>
        ) : (
          options.map((o) => (
            <DropdownMenuItem key={o.id} onClick={() => o.id !== activeId && onSelect(o.id)}>
              <span className="min-w-0 flex-1 truncate">{o.label}</span>
              {o.hint ? <span className="text-xs text-muted-foreground">{o.hint}</span> : null}
              {o.id === activeId ? <CheckMark size={14} /> : null}
            </DropdownMenuItem>
          ))
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** Resume crumb: switch to another resume (opens its default version). */
export function ResumeSwitcher({ resumeId, title }: { resumeId: string; title: string }) {
  const router = useRouter();
  return (
    <CrumbMenu
      label={title}
      menuLabel="Switch resume"
      activeId={resumeId}
      load={async () => {
        const { resumes } = await apiJSON<{ resumes: { id: string; title: string }[] }>("/api/resumes");
        return resumes.map((r) => ({ id: r.id, label: r.title }));
      }}
      // /resume/:id redirects to that resume's default version.
      onSelect={(id) => router.push(`/resume/${id}`)}
    />
  );
}

/** Version crumb: switch to another version of this resume. */
export function VersionSwitcher({
  resumeId,
  variantId,
  name,
}: {
  resumeId: string;
  variantId: string;
  name: string;
}) {
  const router = useRouter();
  return (
    <CrumbMenu
      label={name}
      menuLabel="Switch version"
      activeId={variantId}
      load={async () => {
        const { variations } = await apiJSON<{ variations: { id: string; name: string; isDefault: boolean }[] }>(
          `/api/resumes/${resumeId}/variations`,
        );
        return [...variations]
          .sort((a, b) => Number(b.isDefault) - Number(a.isDefault))
          .map((v) => ({ id: v.id, label: v.name, hint: v.isDefault ? "Default" : undefined }));
      }}
      onSelect={(id) => router.push(`/resume/${resumeId}/${id}`)}
    />
  );
}
