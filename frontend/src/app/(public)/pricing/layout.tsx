import { SiteShell } from "@/components/layout/site-shell";
import { publicNavigation } from "@/config/navigation";

export default function PricingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <SiteShell navigation={publicNavigation} label="Public">
      {children}
    </SiteShell>
  );
}
