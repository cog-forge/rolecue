import type { useTranslations } from "next-intl";

// Translator for the shared form-validation messages. Schemas are built from it, so the message
// that reaches a field error is already in the active language: build them with
// `useMemo(() => createXSchema(v), [v])` where `v = useTranslations("validation")`.
export type Validation = ReturnType<typeof useTranslations<"validation">>;
