import { WizardNavigation } from "@/features/interview-setup/components/wizard-navigation";
export default function WizardLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-8">
      <WizardNavigation />
      {children}
    </div>
  );
}
