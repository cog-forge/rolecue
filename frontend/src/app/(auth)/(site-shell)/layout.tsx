import { SiteShell } from "@/components/layout/site-shell";
import { publicNavigation } from "@/config/navigation";

export default function AuthLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SiteShell navigation={publicNavigation} label="Account">
      {children}
    </SiteShell>
  );
}
