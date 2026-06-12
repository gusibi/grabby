import type { JsonDailyReportData } from "@/types";

type NormalizedSection = { title: string; items: unknown[] };
type RawSection = {
  title?: string;
  heading?: string;
  items?: unknown[];
};

const extractFirstJsonObject = (content: string): string | null => {
  const startIdx = content.indexOf("{");
  if (startIdx === -1) return null;

  let depth = 0;
  let inString = false;
  let escaped = false;

  for (let i = startIdx; i < content.length; i += 1) {
    const char = content[i];

    if (inString) {
      if (escaped) {
        escaped = false;
      } else if (char === "\\") {
        escaped = true;
      } else if (char === "\"") {
        inString = false;
      }
      continue;
    }

    if (char === "\"") {
      inString = true;
    } else if (char === "{") {
      depth += 1;
    } else if (char === "}") {
      depth -= 1;
      if (depth === 0) {
        return content.substring(startIdx, i + 1);
      }
    }
  }

  return null;
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const normalizeReportData = (reportData: any): JsonDailyReportData | null => {
  if (!reportData || !reportData.sections) return null;

  if (Array.isArray(reportData.sections)) {
    const rawSections = reportData.sections as RawSection[];
    const sections = rawSections.reduce<Record<string, NormalizedSection>>(
      (acc: Record<string, NormalizedSection>, section: RawSection, index: number) => {
        if (!section || !Array.isArray(section.items)) return acc;
        acc[`section_${index}`] = {
          title: section.title || section.heading || "",
          items: section.items,
        };
        return acc;
      },
      {}
    );
    return { ...reportData, sections };
  }

  return reportData;
};

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

    const jsonCandidate = extractFirstJsonObject(rawContent);
    if (jsonCandidate) {
      const reportData = JSON.parse(jsonCandidate);
      return normalizeReportData(reportData);
    }
  } catch (e) {
    console.error("Failed to parse report content as JSON", e);
  }

  return null;
};
