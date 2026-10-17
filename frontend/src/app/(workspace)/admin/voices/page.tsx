import { VoiceCatalog } from "@/features/admin/voices/components/voice-catalog";
import { parseVoiceFilters, type SearchParams } from "@/features/admin/voices/utils/voice-filters";

export default async function Page({ searchParams }: { searchParams: Promise<SearchParams> }) {
  return <VoiceCatalog filters={parseVoiceFilters(await searchParams)} />;
}
