"use client";

import { useEffect, type ReactNode } from "react";
import { useTranslations } from "next-intl";
import { Link, usePathname, useRouter } from "@/i18n/routing";
import { useAuth } from "@/lib/auth";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { LanguageSwitcher } from "@/components/LanguageSwitcher";
import {
  BadgeCheck,
  Bell,
  FileText,
  KeyRound,
  LayoutDashboard,
  ListChecks,
  Loader2,
  LogOut,
  Monitor,
  Package,
  ScrollText,
  ShieldAlert,
  ShieldBan,
  ShieldCheck,
  UserCircle,
  Users,
} from "lucide-react";

export default function DashboardLayout({ children }: { children: ReactNode }) {
  const { status, user, logout } = useAuth();
  const router = useRouter();
  const t = useTranslations("nav");
  const pathname = usePathname();

  useEffect(() => {
    if (status === "anonymous") router.replace("/login");
  }, [status, router]);

  if (status !== "authenticated") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  const isAdmin = user?.role === "admin";
  const links = [
    { href: "/", label: t("overview"), icon: LayoutDashboard },
    { href: "/machines", label: t("machines"), icon: Monitor },
    { href: "/software", label: t("software"), icon: Package },
    { href: "/signatures", label: t("signatures"), icon: BadgeCheck },
    { href: "/vulnerabilities", label: t("vulnerabilities"), icon: ShieldAlert },
    { href: "/licenses", label: t("licenses"), icon: KeyRound },
    { href: "/policy/whitelist", label: t("whitelist"), icon: ListChecks },
    { href: "/policy/blacklist", label: t("blacklist"), icon: ShieldBan },
    { href: "/alerts", label: t("alerts"), icon: Bell },
    { href: "/reports", label: t("reports"), icon: FileText },
    { href: "/settings/profile", label: t("profile"), icon: UserCircle },
    ...(isAdmin
      ? [
          { href: "/settings/users", label: t("users"), icon: Users },
          { href: "/settings/audit-log", label: t("auditLog"), icon: ScrollText },
        ]
      : []),
  ];

  return (
    <div className="flex min-h-screen">
      <aside className="hidden w-60 shrink-0 border-r bg-muted/20 p-4 md:block">
        <div className="mb-8 flex items-center gap-2 px-2">
          <ShieldCheck className="h-6 w-6 text-primary" />
          <span className="text-lg font-bold">SoftSentry</span>
        </div>
        <nav className="space-y-1">
          {links.map(({ href, label, icon: Icon }) => {
            const active = href === "/" ? pathname === "/" : pathname.startsWith(href);
            return (
              <Link
                key={href}
                href={href}
                className={cn(
                  "flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                  active
                    ? "bg-primary text-primary-foreground"
                    : "text-muted-foreground hover:bg-muted hover:text-foreground",
                )}
              >
                <Icon className="h-4 w-4" />
                {label}
              </Link>
            );
          })}
        </nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex items-center justify-between border-b px-6 py-3">
          <div className="flex gap-4 md:hidden">
            {links.map(({ href, label }) => (
              <Link key={href} href={href} className="text-sm font-medium hover:underline">
                {label}
              </Link>
            ))}
          </div>
          <div className="ml-auto flex items-center gap-3">
            <span className="text-sm text-muted-foreground">{user?.email}</span>
            <LanguageSwitcher />
            <Button variant="outline" size="sm" onClick={() => void logout()}>
              <LogOut className="mr-2 h-4 w-4" />
              {t("logout")}
            </Button>
          </div>
        </header>
        <main className="flex-1 overflow-auto p-6">{children}</main>
      </div>
    </div>
  );
}
