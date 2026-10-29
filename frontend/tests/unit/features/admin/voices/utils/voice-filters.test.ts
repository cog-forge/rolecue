import { describe, expect, it } from "vitest";
import {
  MAX_VOICE_QUERY,
  defaultVoiceFilters,
  parseVoiceFilters,
  voiceApiParams,
  voiceSearch,
} from "@/features/admin/voices/utils/voice-filters";
import { voicePageEnvelopeSchema } from "@/features/admin/voices/schemas/voice-schema";

describe("voice filters", () => {
  it("parses valid values and falls back on bad ones", () => {
    expect(
      parseVoiceFilters({ status: "inactive", q: "  Ava ", page: "3" }),
    ).toEqual({
      status: "inactive",
      q: "Ava",
      page: 3,
    });
    expect(
      parseVoiceFilters({ status: "archived", page: "-1", q: ["a", "b"] }),
    ).toEqual(defaultVoiceFilters);
    expect(parseVoiceFilters({ q: "a".repeat(MAX_VOICE_QUERY + 1) }).q).toBe(
      "",
    );
    for (const bad of ["1e9", "0x10", "2.5", "0", "999999999"])
      expect(parseVoiceFilters({ page: bad }).page).toBe(1);
  });

  it("keeps defaults out of the URL", () => {
    expect(voiceSearch(defaultVoiceFilters)).toBe("");
    expect(voiceSearch({ status: "active", q: "Ava 50%", page: 2 })).toBe(
      "?status=active&q=Ava+50%25&page=2",
    );
  });

  it("builds the Go query", () => {
    expect(voiceApiParams({ status: "all", q: "", page: 3 })).toEqual({
      status: "all",
      limit: 9,
      offset: 18,
    });
    expect(voiceApiParams({ status: "active", q: "Ava", page: 1 })).toEqual({
      status: "active",
      limit: 9,
      offset: 0,
      q: "Ava",
    });
  });
});

describe("voice schema", () => {
  const voice = {
    id: "33333333-3333-4333-8333-333333333333",
    name: "Ava",
    is_active: true,
    provider: "azure",
    provider_voice_id: "en-US-AvaNeural",
    locale: "en-US",
    gender: "Female",
    interview_count: 2,
    job_posting_count: 0,
    in_use: true,
  };
  it("accepts the Go page contract and rejects drift", () => {
    const envelope = {
      success: true,
      data: { items: [voice], pagination: { limit: 20, offset: 0, total: 1 } },
    };
    expect(voicePageEnvelopeSchema.parse(envelope).data.items[0]?.name).toBe(
      "Ava",
    );
    expect(() =>
      voicePageEnvelopeSchema.parse({
        ...envelope,
        data: { ...envelope.data, items: [{ ...voice, in_use: "yes" }] },
      }),
    ).toThrow();
  });
  it("accepts legacy voices without provider metadata but not a missing field", () => {
    const legacy = {
      ...voice,
      provider: null,
      provider_voice_id: null,
      locale: null,
      gender: null,
    };
    const page = (item: unknown) => ({
      success: true,
      data: { items: [item], pagination: { limit: 20, offset: 0, total: 1 } },
    });
    expect(
      voicePageEnvelopeSchema.parse(page(legacy)).data.items[0]?.locale,
    ).toBeNull();
    const withoutLocale: Partial<typeof voice> = { ...voice };
    delete withoutLocale.locale;
    expect(() => voicePageEnvelopeSchema.parse(page(withoutLocale))).toThrow();
  });
});
