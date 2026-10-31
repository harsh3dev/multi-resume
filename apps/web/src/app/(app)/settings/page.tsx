"use client";

import { useEffect, useState } from "react";

import { Setting2 } from "iconsax-react";

import { PageHeader, PageTitle } from "@/components/layout/page-header";
import { ThemeSwitch } from "@/components/theme-switch";
import { Separator } from "@/components/ui/separator";
import { apiJSON } from "@/lib/api/client";

export default function SettingsPage() {
  const [email, setEmail] = useState<string | null>(null);

  useEffect(() => {
    apiJSON<{ userId: string; email: string }>("/api/me")
      .then((me) => setEmail(me.email))
      .catch(() => setEmail(null));
  }, []);

  return (
    <div className="mx-auto max-w-2xl p-6">
      <PageHeader>
        <PageTitle icon={Setting2}>Settings</PageTitle>
      </PageHeader>

      <section className="flex flex-col gap-1">
        <h2 className="text-sm font-semibold">Account</h2>
        <p className="text-sm text-muted-foreground">{email ?? "…"}</p>
      </section>

      <Separator className="my-6" />

      <section className="flex items-center justify-between">
        <div className="flex flex-col gap-1">
          <h2 className="text-sm font-semibold">Appearance</h2>
          <p className="text-sm text-muted-foreground">
            Switch between light and dark. Defaults to your system setting.
          </p>
        </div>
        <ThemeSwitch />
      </section>
    </div>
  );
}
