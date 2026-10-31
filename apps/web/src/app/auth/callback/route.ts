import { NextResponse } from "next/server";

import { createClient } from "@/lib/supabase/server";

// OAuth PKCE callback: exchange the code for a session, then go home.
export async function GET(request: Request) {
  const { searchParams, origin } = new URL(request.url);
  const code = searchParams.get("code");
  if (code) {
    try {
      const supabase = await createClient();
      await supabase.auth.exchangeCodeForSession(code);
    } catch {
      return NextResponse.redirect(`${origin}/login`);
    }
  }
  return NextResponse.redirect(`${origin}/`);
}
