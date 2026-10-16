import type { Role } from "@/types";

/** UX-only route guard. The Go server remains the sole authority for permissions (guide 6.4). */
export function requiredRoleFor(pathname: string): Role | null {
  if (pathname === "/parent" || pathname.startsWith("/parent/")) return "PARENT";
  if (pathname === "/agency" || pathname.startsWith("/agency/")) return "AGENCY";
  if (pathname === "/admin" || pathname.startsWith("/admin/")) return "ADMIN";
  return null;
}

export const roleHome: Record<Role, string> = { PARENT: "/parent", AGENCY: "/agency", ADMIN: "/admin" };

export interface NavItem { href: string; key: string; icon: string }

export const navByRole: Record<Role, NavItem[]> = {
  PARENT: [
    { href: "/parent", key: "tracker", icon: "route" },
    { href: "/parent/documents", key: "documents", icon: "file" },
    { href: "/parent/referral", key: "referral", icon: "heart" },
    { href: "/parent/messages", key: "messages", icon: "message" },
    { href: "/parent/assistant", key: "assistant", icon: "sparkles" },
    { href: "/parent/profile", key: "profile", icon: "user" },
  ],
  AGENCY: [
    { href: "/agency", key: "cases", icon: "folder" },
    { href: "/agency/children", key: "children", icon: "baby" },
  ],
  ADMIN: [
    { href: "/admin", key: "radar", icon: "radar" },
    { href: "/admin/child-alerts", key: "childAlerts", icon: "bell" },
    { href: "/admin/agencies", key: "agencies", icon: "building" },
    { href: "/admin/knowledge", key: "knowledge", icon: "book" },
    { href: "/admin/scams", key: "scams", icon: "shield" },
    { href: "/admin/audit", key: "audit", icon: "scroll" },
  ],
};
