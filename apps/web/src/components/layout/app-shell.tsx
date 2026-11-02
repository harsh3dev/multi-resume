"use client";

import { AnimatePresence, motion } from "framer-motion";
import { Home2, Logout, Setting2 } from "@/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";

import {
  HeaderSlotProvider,
  HeaderSlotTarget,
} from "@/components/layout/page-header";
import {
  SidebarProvider,
  SidebarTrigger,
  useSidebar,
} from "@/components/layout/sidebar-state";
import { NotificationBell } from "@/components/layout/notification-bell";
import { ThemeSwitch } from "@/components/theme-switch";
import { AppIcon, type IconsaxIcon } from "@/icons";
import { IconButton } from "@/components/ui/components/actions/icon-button";
import { LogoTile } from "@/components/ui/components/display/logo";
import { Separator } from "@/components/ui/components/navigation/separator";
import { UserAvatar } from "@/components/ui/components/navigation/user-avatar";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/components/actions/tooltip";
import { useMotion } from "@/lib/components/motion";
import { createClient } from "@/lib/supabase/client";
import { cn } from "@/lib/utils/utils";

interface NavItem {
  href: string;
  label: string;
  icon: IconsaxIcon;
  /** Other route prefixes that belong under this item (e.g. the editor). */
  alsoActiveFor?: string[];
}

const NAV_GROUPS: { title: string; items: NavItem[] }[] = [
  {
    title: "Platform",
    items: [
      { href: "/dashboard", label: "Dashboard", icon: Home2, alsoActiveFor: ["/resume"] },
      { href: "/settings", label: "Settings", icon: Setting2 },
    ],
  },
];

function NavLink({
  item,
  collapsed,
  onNavigate,
}: {
  item: NavItem;
  collapsed: boolean;
  onNavigate?: () => void;
}) {
  const pathname = usePathname();
  const active = [item.href, ...(item.alsoActiveFor ?? [])].some(
    (prefix) => pathname === prefix || pathname.startsWith(prefix + "/"),
  );
  const link = (
    <Link
      href={item.href}
      onClick={onNavigate}
      aria-current={active ? "page" : undefined}
      aria-label={collapsed ? item.label : undefined}
      className={cn(
        "flex h-8 items-center gap-2 rounded-md text-[13px] font-medium tracking-tighter transition-colors outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring",
        collapsed ? "mx-auto w-8 justify-center" : "px-2",
        active
          ? "bg-primary/10 text-sidebar-foreground inset-shadow-bevel-soft dark:bg-primary/15 [&_svg]:text-primary"
          : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-accent-foreground",
      )}
    >
      <AppIcon icon={item.icon} size="md" variant="Bulk" />
      {collapsed ? null : <span className="truncate">{item.label}</span>}
    </Link>
  );
  if (!collapsed) return link;
  return (
    <Tooltip>
      <TooltipTrigger render={link} />
      <TooltipContent side="right">{item.label}</TooltipContent>
    </Tooltip>
  );
}

function AccountRow({ collapsed }: { collapsed: boolean }) {
  const router = useRouter();
  const [account, setAccount] = useState<{ name: string | null; email: string | null }>({
    name: null,
    email: null,
  });

  useEffect(() => {
    let active = true;
    createClient()
      .auth.getUser()
      .then(({ data }) => {
        if (!active) return;
        const user = data.user;
        const profile = user?.user_metadata;
        const name = [profile?.full_name, profile?.name].find(
          (value): value is string => typeof value === "string" && value.trim().length > 0,
        );
        setAccount({ name: name?.trim() ?? null, email: user?.email ?? null });
      })
      .catch(() => {
        if (active) setAccount({ name: null, email: null });
      });
    return () => { active = false; };
  }, []);

  async function signOut() {
    await createClient().auth.signOut();
    router.push("/login");
    router.refresh();
  }

  return (
    <div className={cn("flex items-center gap-2", collapsed && "flex-col")}>
      <UserAvatar value={account.email ?? "user"} />
      {collapsed ? null : (
        <div className="grid min-w-0 flex-1 text-left leading-tight">
          <span className="truncate text-xs font-medium tracking-tight">{account.name ?? account.email ?? "Account"}</span>
          {account.name && account.email ? (
            <span className="truncate text-[11px] text-sidebar-foreground/50">{account.email}</span>
          ) : null}
        </div>
      )}
      <IconButton icon={Logout} label="Sign out" size="icon-sm" onClick={() => void signOut()} />
    </div>
  );
}

function SidebarContent({ collapsed, onNavigate }: { collapsed: boolean; onNavigate?: () => void }) {
  return (
    <div className="flex h-full flex-col bg-sidebar text-sidebar-foreground">
      <div className={cn("flex h-12 shrink-0 items-center gap-2 px-3", collapsed && "justify-center px-0")}>
        <Link href="/" onClick={onNavigate} className="flex items-center gap-2 select-none" aria-label="Multi Resume home">
          <LogoTile />
          {collapsed ? null : <span className="font-serif text-xl font-semibold">Multi Resume</span>}
        </Link>
      </div>
      <nav className="flex-1 overflow-y-auto p-2">
        {NAV_GROUPS.map((group) => (
          <div key={group.title} className="flex flex-col gap-0.5">
            {collapsed ? null : (
              <p className="flex h-8 items-center px-2 text-xs font-medium text-sidebar-foreground/50 select-none">
                {group.title}
              </p>
            )}
            {group.items.map((item) => (
              <NavLink key={item.href} item={item} collapsed={collapsed} onNavigate={onNavigate} />
            ))}
          </div>
        ))}
      </nav>
      <div className="shrink-0 border-t border-sidebar-border p-2">
        <AccountRow collapsed={collapsed} />
      </div>
    </div>
  );
}

function Shell({ children }: { children: ReactNode }) {
  const { collapsed, mobileOpen, setMobileOpen } = useSidebar();
  const { reduced, transition } = useMotion();

  return (
    // Base layer: the sidebar colour. The main area below is a raised panel
    // sitting on top of it.
    <div className="flex h-svh bg-sidebar text-foreground">
      <aside
        data-collapsed={collapsed}
        className="hidden shrink-0 transition-[width] duration-200 ease-out motion-reduce:transition-none md:block w-64 data-[collapsed=true]:w-14"
      >
        <SidebarContent collapsed={collapsed} />
      </aside>

      <AnimatePresence>
        {mobileOpen ? (
          <>
            <motion.div
              key="backdrop"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.15 }}
              className="fixed inset-0 z-40 bg-background/60 backdrop-blur-sm md:hidden"
              onClick={() => setMobileOpen(false)}
            />
            <motion.div
              key="drawer"
              initial={reduced ? { opacity: 0 } : { x: -256 }}
              animate={reduced ? { opacity: 1 } : { x: 0 }}
              exit={reduced ? { opacity: 0 } : { x: -256 }}
              transition={transition}
              className="fixed inset-y-0 left-0 z-50 w-64 md:hidden"
            >
              <SidebarContent collapsed={false} onNavigate={() => setMobileOpen(false)} />
            </motion.div>
          </>
        ) : null}
      </AnimatePresence>

      <div className="flex min-w-0 flex-1 md:py-2 md:pr-2">
        <div className="flex min-w-0 flex-1 flex-col overflow-hidden bg-background md:rounded-xl md:border md:border-transparent md:shadow-panel dark:md:border-border">
          <header className="flex h-12 shrink-0 items-center gap-2 border-b border-border px-3 select-none">
            <SidebarTrigger />
            <Separator orientation="vertical" className="min-h-4" />
            <HeaderSlotTarget />
            <NotificationBell />
            <ThemeSwitch />
          </header>
          <main className="min-h-0 flex-1 overflow-auto">{children}</main>
        </div>
      </div>
    </div>
  );
}

export function AppShell({
  children,
  defaultCollapsed = false,
}: {
  children: ReactNode;
  defaultCollapsed?: boolean;
}) {
  return (
    <SidebarProvider defaultCollapsed={defaultCollapsed}>
      <HeaderSlotProvider>
        <Shell>{children}</Shell>
      </HeaderSlotProvider>
    </SidebarProvider>
  );
}
