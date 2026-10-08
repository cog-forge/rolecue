import type { Metadata } from "next";
import { RoutePlaceholder } from "@/components/feedback/route-placeholder";

export const metadata: Metadata = {
  title: "Terms of Service",
  description: "Review the terms that apply to using RoleCue.",
};

export default function TermsPage() {
  return (
    <RoutePlaceholder
      title="Terms of Service"
      description="Terms of service documentation will be published before production launch."
    />
  );
}
