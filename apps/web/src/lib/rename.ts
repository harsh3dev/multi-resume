import { apiJSON } from "@/lib/api/client";
import { toast } from "@/lib/store/toast";

/** Turns an API error code into a sentence for a toast. */
function reason(e: unknown, what: string): string | undefined {
  if (e instanceof Error && e.message === "name_taken") return `Another ${what} already has that name.`;
  return e instanceof Error ? e.message : undefined;
}

/** Rename a version (variation). Toasts on success and failure; rethrows so a dialog stays open. */
export async function renameVersion(id: string, name: string): Promise<void> {
  try {
    await apiJSON(`/api/variations/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    toast.success("Version renamed", name);
  } catch (e) {
    toast.error("Could not rename version", reason(e, "version"));
    throw e;
  }
}

/** Rename a resume. */
export async function renameResume(id: string, title: string): Promise<void> {
  try {
    await apiJSON(`/api/resumes/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title }),
    });
    toast.success("Resume renamed", title);
  } catch (e) {
    toast.error("Could not rename resume", reason(e, "resume"));
    throw e;
  }
}
