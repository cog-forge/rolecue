import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";
// Dates, numbers and money are formatted through src/lib/i18n so every screen follows the active locale.
const useI18nFormatters =
  "Format through useFormatters()/getFormatters() from @/lib/i18n instead, so the active locale applies.";
export default defineConfig([
  ...nextVitals,
  ...nextTs,
  {
    files: ["src/**/*.{ts,tsx}"],
    ignores: ["src/lib/i18n/**"],
    rules: {
      "no-restricted-syntax": [
        "error",
        {
          selector: "MemberExpression[object.name='Intl']",
          message: useI18nFormatters,
        },
        {
          selector: "CallExpression[callee.property.name=/^toLocale/]",
          message: useI18nFormatters,
        },
      ],
    },
  },
  globalIgnores([
    ".cache/**",
    "coverage/**",
    ".next/**",
    "next-env.d.ts",
    "playwright-report/**",
    "test-results/**",
  ]),
]);
