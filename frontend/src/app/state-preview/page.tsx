import { notFound } from "next/navigation";
import { StatePreview } from "./state-preview";

export default async function Page({
  searchParams,
}: {
  searchParams: Promise<{ state?: string }>;
}) {
  if (process.env.NODE_ENV !== "development") notFound();
  return <StatePreview initialView={(await searchParams).state} />;
}
