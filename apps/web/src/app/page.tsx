import Link from "next/link";

import { ThemeSwitch } from "@/components/theme-switch";
import { Button } from "@/components/ui/button";
import { LogoTile } from "@/components/ui/logo";
import { createClient } from "@/lib/supabase/server";

export default async function LandingPage() {
  // The CTA switches to "Go to dashboard" for signed-in users. When Supabase
  // isn't configured (local scaffolding), treat everyone as signed out.
  let signedIn = false;
  try {
    const supabase = await createClient();
    const {
      data: { user },
    } = await supabase.auth.getUser();
    signedIn = user !== null;
  } catch {
    signedIn = false;
  }

  return (
    <main className="flex min-h-svh flex-col">
      <header className="flex h-14 items-center justify-between px-6">
        <span className="flex items-center gap-2">
          <LogoTile size={28} />
          <span className="font-serif text-lg">Multi Resume</span>
        </span>
        <div className="flex items-center gap-2">
          {!signedIn ? (
            <Button variant="ghost" size="sm" render={<Link href="/login" />}>
              Sign in
            </Button>
          ) : null}
          <ThemeSwitch />
        </div>
      </header>

      <section className="flex flex-1 flex-col items-center justify-center gap-6 px-6 text-center">
        <h1 className="max-w-2xl font-serif text-5xl leading-tight sm:text-6xl">
          Tailor your resume to <em className="text-muted-foreground">every</em> job.
        </h1>
        <p className="max-w-md text-sm text-muted-foreground sm:text-base">
          Edit your LaTeX resume through fields — no TeX required. Keep a
          variation per application and let AI adapt your bullets to the job
          description.
        </p>
        <Button
          size="lg"
          render={<Link href={signedIn ? "/dashboard" : "/login"} />}
        >
          {signedIn ? "Go to dashboard" : "Get started"}
        </Button>
      </section>

      <footer className="flex h-12 items-center justify-center text-xs text-muted-foreground">
        Built on your own LaTeX template.
      </footer>
    </main>
  );
}
