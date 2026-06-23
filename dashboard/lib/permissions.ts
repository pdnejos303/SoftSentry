// Single source of truth for role-based route access (mirrors backend gates in
// app/api/v1/__init__.py). Used by the dashboard nav (which links to show) and
// the layout guard (which paths a role may open directly).
//
// Roles:
//   dev    — superuser, sees and does everything
//   admin  — Overview + Machines + Recent installs + Software + Signatures +
//            Vulnerabilities + Deploy (no Licenses / Policy / Alerts / Reports /
//            Users / Audit Log)
//   viewer — broad read-only (everything except Deploy / Users / Audit Log)

export type Role = "dev" | "admin" | "viewer";

const ALL: Role[] = ["dev", "admin", "viewer"];

/**
 * Route → roles allowed to open it. Order matters: most-specific prefixes first
 * so `/settings/users` is checked before a hypothetical `/settings`. The "/"
 * entry (Overview) is matched exactly, never as a prefix.
 */
export const ROUTE_ACCESS: { prefix: string; roles: Role[] }[] = [
  { prefix: "/settings/users", roles: ["dev"] },
  { prefix: "/settings/audit-log", roles: ["dev"] },
  { prefix: "/settings/profile", roles: ALL },
  { prefix: "/deploy", roles: ["dev", "admin"] },
  { prefix: "/machines", roles: ALL },
  { prefix: "/recent-installs", roles: ["dev", "admin", "viewer"] },
  { prefix: "/software", roles: ["dev", "admin", "viewer"] },
  { prefix: "/signatures", roles: ["dev", "admin", "viewer"] },
  { prefix: "/vulnerabilities", roles: ["dev", "admin", "viewer"] },
  { prefix: "/licenses", roles: [] },
  { prefix: "/policy", roles: ["dev", "viewer"] },
  { prefix: "/alerts", roles: ["dev", "viewer"] },
  { prefix: "/reports", roles: ["dev", "viewer"] },
  { prefix: "/", roles: ALL }, // Overview — exact match only
];

function ruleFor(pathname: string): { prefix: string; roles: Role[] } | undefined {
  return ROUTE_ACCESS.find((r) =>
    r.prefix === "/"
      ? pathname === "/"
      : pathname === r.prefix || pathname.startsWith(`${r.prefix}/`),
  );
}

/**
 * Whether `role` may open `pathname`. `pathname` must be locale-stripped (as
 * returned by next-intl's usePathname). Unknown paths default to allowed so a
 * new page isn't accidentally locked out before it's added to the matrix.
 */
export function canAccessPath(role: Role, pathname: string): boolean {
  const rule = ruleFor(pathname);
  return rule ? rule.roles.includes(role) : true;
}
