import { RoleShell } from "@/components/layout/role-shell";

export default function ParentLayout({ children }: { children: React.ReactNode }) {
  return <RoleShell role="PARENT">{children}</RoleShell>;
}
