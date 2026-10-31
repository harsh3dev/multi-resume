import { cookies } from "next/headers";

import { AppShell } from "@/components/layout/app-shell";
import { SIDEBAR_COOKIE } from "@/components/layout/sidebar-state";

export default async function AppLayout({ children }: { children: React.ReactNode }) {
  // Read on the server so the first paint already has the right width.
  const collapsed = (await cookies()).get(SIDEBAR_COOKIE)?.value === "collapsed";
  return <AppShell defaultCollapsed={collapsed}>{children}</AppShell>;
}
