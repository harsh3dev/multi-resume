"use client";

import { useState } from "react";

import { ThemeSwitch } from "@/components/theme-switch";
import { Button } from "@/components/ui/components/actions/button";
import { LogoTile } from "@/components/ui/components/display/logo";
import { createClient, supabaseConfigured } from "@/lib/supabase/client";

export default function LoginPage() {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  if (!supabaseConfigured()) {
    return (
      <main className="flex min-h-svh items-center justify-center">
        <p className="text-sm text-muted-foreground">
          Supabase is not configured. Set NEXT_PUBLIC_SUPABASE_URL and
          NEXT_PUBLIC_SUPABASE_ANON_KEY.
        </p>
      </main>
    );
  }

  async function signIn() {
    setLoading(true);
    setError(null);
    try {
      const supabase = createClient();
      const { error } = await supabase.auth.signInWithOAuth({
        provider: "google",
        options: { redirectTo: `${window.location.origin}/auth/callback` },
      });
      if (error) {
        setError("Sign-in failed. Please try again.");
        setLoading(false);
      }
    } catch {
      setError("Sign-in failed. Please try again.");
      setLoading(false);
    }
  }

  return (
    <main className="relative flex min-h-svh flex-col items-center justify-center gap-6 px-6">
      <div className="absolute right-4 top-4">
        <ThemeSwitch />
      </div>
      <div className="flex w-full max-w-xs flex-col items-center gap-6 rounded-xl border border-border bg-card p-8 shadow-sm">
        <div className="flex flex-col items-center gap-2 text-center">
          <LogoTile size={44} />
          <h1 className="font-serif text-3xl">Multi Resume</h1>
          <p className="text-sm text-muted-foreground">
            Sign in to edit and tailor your resumes.
          </p>
        </div>
        <Button onClick={signIn} loading={loading} size="lg" className="w-full">
          Continue with Google
        </Button>
        {error ? <p className="text-sm text-destructive">{error}</p> : null}
      </div>
    </main>
  );
}
