import { describe, expect, it } from "vitest";
import {
  formatVND,
  formatCoins,
  formatCoinAmount,
  formatDateTime,
  formatDateOnly,
  formatPercentChange,
  sanitizeCSVCell,
  encodeCSVRow,
} from "@/features/admin/billing/lib/format";
import { formattersFor } from "@tests/helpers/intl";

describe("Billing Format Utilities", () => {
  describe("formatVND", () => {
    it("formats amounts in VND with dot separators and space before ₫", () => {
      expect(formatVND(100000)).toBe("100.000 ₫");
      expect(formatVND(50000)).toBe("50.000 ₫");
      expect(formatVND(2600000)).toBe("2.600.000 ₫");
      expect(formatVND(0)).toBe("0 ₫");
      expect(formatVND(100)).toBe("100 ₫");
    });

    it("handles negative amounts correctly", () => {
      expect(formatVND(-50000)).toBe("-50.000 ₫");
    });

    it("handles null and undefined gracefully", () => {
      expect(formatVND(null)).toBe("0 ₫");
      expect(formatVND(undefined)).toBe("0 ₫");
    });
  });

  describe("formatCoins", () => {
    it("formats coins with labels and locale grouping", () => {
      const fmt = formattersFor("en");
      expect(formatCoins(1, fmt)).toBe("1 coin");
      expect(formatCoins(2, fmt)).toBe("2 coins");
      expect(formatCoins(100, fmt)).toBe("100 coins");
      expect(formatCoins(1500, fmt)).toBe("1,500 coins");
      expect(formatCoins(0, fmt)).toBe("0 coins");
      expect(formatCoins(null, fmt)).toBe("0 coins");
    });

    it("handles negative coins", () => {
      expect(formatCoins(-20, formattersFor("en"))).toBe("-20 coins");
    });

    it("uses the unit and grouping of the active language", () => {
      const fmt = formattersFor("vi");
      expect(formatCoins(1, fmt)).toBe("1 xu");
      expect(formatCoins(1500, fmt)).toBe("1.500 xu");
    });
  });

  describe("formatCoinAmount", () => {
    it("formats coin values without currency label", () => {
      expect(formatCoinAmount(1500)).toBe("1,500");
      expect(formatCoinAmount(1500, "vi")).toBe("1.500");
      expect(formatCoinAmount(20)).toBe("20");
    });
  });

  describe("formatPercentChange", () => {
    it("formats positive, negative, and neutral percentages", () => {
      const pos = formatPercentChange(12.5);
      expect(pos.text).toBe("+12.5%");
      expect(pos.isPositive).toBe(true);
      expect(pos.isNeutral).toBe(false);

      const neg = formatPercentChange(-3.2);
      expect(neg.text).toBe("-3.2%");
      expect(neg.isPositive).toBe(false);
      expect(neg.isNeutral).toBe(false);

      const zero = formatPercentChange(0);
      expect(zero.text).toBe("0.0%");
      expect(zero.isNeutral).toBe(true);
    });
  });

  describe("formatDateOnly and formatDateTime in UTC+7", () => {
    it("formats ISO date string into dd/MM/yyyy", () => {
      expect(formatDateOnly("2026-10-24")).toBe("24/10/2026");
    });

    it("formats timestamps into UTC+7 Vietnam time", () => {
      // 2026-10-24T03:00:00Z is 10:00 in UTC+7
      const formatted = formatDateTime("2026-10-24T03:00:00Z");
      expect(formatted).toBe("24/10/2026 10:00");
    });
  });

  describe("sanitizeCSVCell and encodeCSVRow", () => {
    it("prefixes formula injection characters with a single quote", () => {
      expect(sanitizeCSVCell("=SUM(A1:A2)")).toBe("'=SUM(A1:A2)");
      expect(sanitizeCSVCell("+12345")).toBe("'+12345");
      expect(sanitizeCSVCell("-999")).toBe("'-999");
      expect(sanitizeCSVCell("@cmd")).toBe("'@cmd");
      expect(sanitizeCSVCell("\tmalicious")).toBe("'\tmalicious");
      expect(sanitizeCSVCell("\rmalicious")).toBe("'\rmalicious");
      expect(sanitizeCSVCell("Starter Pack")).toBe("Starter Pack");
      expect(sanitizeCSVCell("")).toBe("");
    });

    it("encodes CSV rows with formula sanitization on strings while preserving negative numbers", () => {
      expect(encodeCSVRow(["=1+1", 'Pack "Pro"', -20, 50000])).toBe(
        '"\'=1+1","Pack ""Pro""","-20","50000"',
      );
    });
  });
});
