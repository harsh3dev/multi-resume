"use client";

import { DocumentText, Edit2, More, Trash } from "iconsax-react";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { DownloadMenu } from "@/components/editor/download-menu";
import { RecompileButton } from "@/components/editor/recompile-button";
import { ResumeSwitcher, VersionSwitcher } from "@/components/editor/breadcrumb-menus";
import { SaveStatus } from "@/components/editor/save-status";
import { Breadcrumbs } from "@/components/layout/breadcrumbs";
import { PageHeader } from "@/components/layout/page-header";
import { RenameDialog } from "@/components/rename-dialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { AppIcon } from "@/components/ui/icon";
import { DeleteVersionDialog } from "@/components/versions/delete-version-dialog";
import { renameResume, renameVersion } from "@/lib/rename";

// Rendered into the shell's header bar (see PageHeader), not its own bar.
export function EditorHeader({
  resumeId,
  variantId,
  resumeTitle,
  variationName,
  isDefault,
  onResumeRenamed,
  onVersionRenamed,
}: {
  resumeId: string;
  variantId: string;
  resumeTitle: string;
  variationName: string;
  isDefault: boolean;
  onResumeRenamed: (title: string) => void;
  onVersionRenamed: (name: string) => void;
}) {
  const router = useRouter();
  const [dialog, setDialog] = useState<"rename-version" | "rename-resume" | "delete-version" | null>(null);
  const close = (open: boolean) => !open && setDialog(null);

  return (
    <PageHeader>
      <div className="flex min-w-0 flex-1 items-center gap-2">
        <span className="flex shrink-0 items-center justify-center rounded-md bg-gradient-to-br from-light-primary to-primary p-[7px] text-primary-foreground">
          <AppIcon icon={DocumentText} size={14} variant="Bulk" />
        </span>
        <h1 className="sr-only">
          {resumeTitle ? `Edit ${resumeTitle} (${variationName})` : `Edit ${variationName}`}
        </h1>
        <Breadcrumbs
          items={[
            { label: "Dashboard", href: "/dashboard" },
            ...(resumeTitle
              ? [{ label: resumeTitle, node: <ResumeSwitcher resumeId={resumeId} title={resumeTitle} /> }]
              : []),
            {
              label: variationName,
              node: <VersionSwitcher resumeId={resumeId} variantId={variantId} name={variationName} />,
            },
          ]}
        />
        <DropdownMenu>
          <DropdownMenuTrigger
            render={
              <Button variant="ghost" size="icon-sm" aria-label="Rename or delete">
                <AppIcon icon={More} />
              </Button>
            }
          />
          <DropdownMenuContent align="start" className="w-48">
            <DropdownMenuItem onClick={() => setDialog("rename-version")}>
              <AppIcon icon={Edit2} /> Rename version
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => setDialog("rename-resume")}>
              <AppIcon icon={Edit2} /> Rename resume
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              variant="destructive"
              disabled={isDefault}
              onClick={() => setDialog("delete-version")}
            >
              <AppIcon icon={Trash} /> {isDefault ? "Default version can't be deleted" : "Delete version"}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <SaveStatus />
      <RecompileButton />
      <DownloadMenu name={variationName} />

      <RenameDialog
        open={dialog === "rename-version"}
        onOpenChange={close}
        title="Rename version"
        label="Version name"
        maxLength={120}
        initialValue={variationName}
        onSubmit={async (name) => {
          await renameVersion(variantId, name);
          onVersionRenamed(name);
        }}
      />
      <RenameDialog
        open={dialog === "rename-resume"}
        onOpenChange={close}
        title="Rename resume"
        description="This is the name shown on your dashboard."
        label="Resume name"
        maxLength={200}
        initialValue={resumeTitle}
        onSubmit={async (title) => {
          await renameResume(resumeId, title);
          onResumeRenamed(title);
        }}
      />
      {dialog === "delete-version" ? (
        <DeleteVersionDialog
          version={{ id: variantId, name: variationName }}
          open
          onOpenChange={close}
          onDeleted={() => router.push("/dashboard")}
        />
      ) : null}
    </PageHeader>
  );
}
