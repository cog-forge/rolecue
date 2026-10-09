import { AccountManagement } from "@/features/auth/components/account-management";
import { AdminSection } from "@/features/admin/components/admin-section";
export default function Page() {
  return (
    <>
      <AdminSection title="Users" description="Manage platform users." />
      <AccountManagement />
    </>
  );
}
