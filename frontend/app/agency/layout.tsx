import { RoleShell } from "@/components/layout/role-shell";

export default function AgencyLayout({ children }: { children: React.ReactNode }) {
  return <RoleShell role="AGENCY">{children}</RoleShell>;
}
