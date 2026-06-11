import type { JsonDailyReportData } from "@/types";

export const parseDailyReportContent = (content?: string): JsonDailyReportData | null => {
  try {
    let rawContent = (content || "").trim();
    if (rawContent.startsWith("```json")) {
      rawContent = rawContent.substring(7);
      if (rawContent.endsWith("```")) {
        rawContent = rawContent.substring(0, rawContent.length - 3);
      }
      rawContent = rawContent.trim();
    } else if (rawContent.startsWith("```")) {
      rawContent = rawContent.substring(3);
      if (rawContent.endsWith("```")) {
        rawContent = rawContent.substring(0, rawContent.length - 3);
      }
      rawContent = rawContent.trim();
    }

    const startIdx = rawContent.indexOf("{");
    const endIdx = rawContent.lastIndexOf("}");
    if (startIdx !== -1 && endIdx !== -1 && endIdx > startIdx) {
      const jsonCandidate = rawContent.substring(startIdx, endIdx + 1);
      const reportData = JSON.parse(jsonCandidate);
      if (reportData && reportData.sections) {
        return reportData;
      }
    }
  } catch (e) {
    console.error("Failed to parse report content as JSON", e);
  }

  return null;
};
