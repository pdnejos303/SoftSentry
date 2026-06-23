"use client";

import { useEffect, type ComponentType, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Link, usePathname, useRouter } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { canAccessPath, type Role } from "@/lib/permissions";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import { ThemeToggle } from "@/components/ThemeToggle";
import {
  BadgeCheck,
  Bell,
  DownloadCloud,
  FileText,
  KeyRound,
  LayoutDashboard,
  ListChecks,
  Loader2,
  LogOut,
  Monitor,
  Package,
  PackagePlus,
  ScrollText,
  ShieldAlert,
  ShieldBan,
  ShieldCheck,
  UserCircle,
  Users,
} from "lucide-react";

type NavLink = {
  href: string;
  label: string;
  icon: ComponentType<{ className?: string }>;
};
type NavSection = { title: string; links: NavLink[] };

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { status, user, logout } = useAuth();
  const router = useRouter();
  const t = useTranslations("nav");
  const pathname = usePathname();

  const role = user?.role as Role | undefined;

  useEffect(() => {
    if (status === "anonymous") router.replace("/login");
    // Demoted roles that deep-link (or get redirected) to a page they can't see
    // land back on the Overview rather than hitting a bare 403 from the API.
    else if (status === "authenticated" && role && !canAccessPath(role, pathname)) {
      router.replace("/");
    }
  }, [status, role, pathname, router]);

  if (status !== "authenticated") {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // Grouped so a long, flat list reads as an organized console rather than a
  // wall of links. Visibility is derived from the shared route-access matrix so
  // the nav and the path guard above can never drift apart; an entire section
  // disappears when a role can see none of its destinations.
  const allSections: NavSection[] = [
    {
      title: t("sectionMonitor"),
      links: [
        { href: "/", label: t("overview"), icon: LayoutDashboard },
        { href: "/machines", label: t("machines"), icon: Monitor },
        { href: "/alerts", label: t("alerts"), icon: Bell },
      ],
    },
    {
      title: t("sectionInventory"),
      links: [
        { href: "/recent-installs", label: t("recentInstalls"), icon: PackagePlus },
        { href: "/software", label: t("software"), icon: Package },
        { href: "/signatures", label: t("signatures"), icon: BadgeCheck },
        { href: "/vulnerabilities", label: t("vulnerabilities"), icon: ShieldAlert },
        { href: "/licenses", label: t("licenses"), icon: KeyRound },
      ],
    },
    {
      title: t("sectionGovernance"),
      links: [
        { href: "/policy/whitelist", label: t("whitelist"), icon: ListChecks },
        { href: "/policy/blacklist", label: t("blacklist"), icon: ShieldBan },
        { href: "/reports", label: t("reports"), icon: FileText },
        { href: "/deploy", label: t("deploy"), icon: DownloadCloud },
      ],
    },
    {
      title: t("sectionSystem"),
      links: [
        { href: "/settings/profile", label: t("profile"), icon: UserCircle },
        { href: "/settings/users", label: t("users"), icon: Users },
        { href: "/settings/audit-log", label: t("auditLog"), icon: ScrollText },
      ],
    },
  ];

  const sections = role
    ? allSections
        .map((s) => ({
          ...s,
          links: s.links.filter((l) => canAccessPath(role, l.href)),
        }))
        .filter((s) => s.links.length > 0)
    : [];

  const isActive = (href: string) =>
    href === "/" ? pathname === "/" : pathname.startsWith(href);

  const flatLinks = sections.flatMap((s) => s.links);
  const initials = ((user?.email ?? "?").split("@")[0] ?? "?")
    .slice(0, 2)
    .toUpperCase();

  return (
    <div className="flex min-h-screen bg-background">
      <aside className="hidden w-64 shrink-0 flex-col border-r border-sidebar-border bg-sidebar md:flex">
        <div className="flex items-center gap-2.5 px-5 pb-5 pt-6">
          <span className="flex h-9 w-9 items-center justify-center rounded-md bg-primary/12 text-primary">
            <ShieldCheck className="h-5 w-5" />
          </span>
          <span className="text-[17px] font-semibold tracking-tight text-sidebar-foreground">
            SoftSentry
          </span>
        </div>

        <nav className="flex-1 overflow-y-auto px-3 pb-6">
          {sections.map((section, i) => (
            <div key={section.title} className={cn(i === 0 ? "mt-1" : "mt-6")}>
              <p className="px-3 pb-2 text-[11px] font-semibold uppercase tracking-wider text-sidebar-muted/70">
                {section.title}
              </p>
              <div className="space-y-0.5">
                {section.links.map(({ href, label, icon: Icon }) => {
                  const active = isActive(href);
                  return (
                    <Link
                      key={href}
                      href={href}
                      aria-current={active ? "page" : undefined}
                      className={cn(
                        "group relative flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                        active
                          ? "bg-sidebar-active-bg text-sidebar-active"
                          : "text-sidebar-muted hover:bg-sidebar-accent hover:text-sidebar-foreground",
                      )}
                    >
                      {active && (
                        <span className="absolute left-0 top-1/2 h-5 w-[3px] -translate-y-1/2 rounded-r-full bg-sidebar-active" />
                      )}
                      <Icon className="h-[1.05rem] w-[1.05rem] shrink-0" />
                      {label}
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="sticky top-0 z-20 border-b border-border bg-background/85 backdrop-blur supports-[backdrop-filter]:bg-background/70">
          <div className="flex h-14 items-center gap-3 px-4 sm:px-6">
            {/* Mobile nav: scrollable chip row instead of amputating the menu. */}
            <nav className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto md:hidden">
              {flatLinks.map(({ href, label }) => {
                const active = isActive(href);
                return (
                  <Link
                    key={href}
                    href={href}
                    className={cn(
                      "shrink-0 rounded-md px-2.5 py-1.5 text-sm font-medium transition-colors",
                      active
                        ? "bg-sidebar-active-bg text-sidebar-active"
                        : "text-muted-foreground hover:bg-accent hover:text-foreground",
                    )}
                  >
                    {label}
                  </Link>
                );
              })}
            </nav>

            <div className="ml-auto flex items-center gap-2 sm:gap-3">
              <ThemeToggle />
              <LanguageSwitcher />

              <span className="hidden h-6 w-px bg-border sm:block" />

              <div className="hidden items-center gap-2.5 sm:flex">
                <span className="flex h-8 w-8 items-center justify-center rounded-full bg-primary/12 text-[11px] font-semibold text-primary">
                  {initials}
                </span>
                <div className="leading-tight">
                  <p className="max-w-[14rem] truncate text-sm font-medium text-foreground">
                    {user?.email}
                  </p>
                  {role && (
                    <p className="text-[11px] uppercase tracking-wide text-muted-foreground">
                      {role}
                    </p>
                  )}
                </div>
              </div>

              <Button variant="outline" size="sm" onClick={() => void logout()}>
                <LogOut className="mr-2 h-4 w-4" />
                <span className="hidden sm:inline">{t("logout")}</span>
              </Button>
            </div>
          </div>
        </header>

        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
