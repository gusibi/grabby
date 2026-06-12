package ai

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

type dailyReportPayload struct {
	Title    string                        `json:"title"`
	Date     string                        `json:"date"`
	Editor   string                        `json:"editor,omitempty"`
	Sections map[string]dailyReportSection `json:"sections"`
}

type dailyReportSection struct {
	Title string            `json:"title"`
	Items []dailyReportItem `json:"items"`
}

type dailyReportItem struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Source  string `json:"source,omitempty"`
	Link    string `json:"link,omitempty"`
	Score   string `json:"score,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type rawDailyReportPayload struct {
	Title    string          `json:"title"`
	Date     string          `json:"date"`
	Editor   string          `json:"editor"`
	Sections json.RawMessage `json:"sections"`
}

type rawDailyReportSection struct {
	Title   string           `json:"title"`
	Heading string           `json:"heading"`
	Items   []map[string]any `json:"items"`
}

func normalizeDailyReportContent(raw string, dateStr string) (string, error) {
	jsonText, err := extractFirstJSONObject(raw)
	if err != nil {
		return "", err
	}

	var rawPayload rawDailyReportPayload
	if err := json.Unmarshal([]byte(jsonText), &rawPayload); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	payload, err := normalizeDailyReportPayload(rawPayload, dateStr)
	if err != nil {
		return "", err
	}

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal normalized daily report: %w", err)
	}
	return string(out), nil
}

func normalizeDailyReportPayload(rawPayload rawDailyReportPayload, dateStr string) (dailyReportPayload, error) {
	payload := dailyReportPayload{
		Title:  strings.TrimSpace(rawPayload.Title),
		Date:   strings.TrimSpace(rawPayload.Date),
		Editor: strings.TrimSpace(rawPayload.Editor),
	}
	if payload.Date == "" {
		payload.Date = dateStr
	}
	if payload.Title == "" {
		payload.Title = "Grabby AI 智能日报 · " + payload.Date
	}
	if len(rawPayload.Sections) == 0 {
		return payload, fmt.Errorf("missing sections")
	}

	sections, err := normalizeDailyReportSections(rawPayload.Sections)
	if err != nil {
		return payload, err
	}
	if len(sections) == 0 {
		return payload, fmt.Errorf("sections must contain at least one non-empty section")
	}
	payload.Sections = sections
	return payload, nil
}

func normalizeDailyReportSections(raw json.RawMessage) (map[string]dailyReportSection, error) {
	var byKey map[string]rawDailyReportSection
	if err := json.Unmarshal(raw, &byKey); err == nil && byKey != nil {
		return normalizeSectionMap(byKey), nil
	}

	var list []rawDailyReportSection
	if err := json.Unmarshal(raw, &list); err == nil && list != nil {
		sections := make(map[string]rawDailyReportSection)
		for i, section := range list {
			sections[fmt.Sprintf("section_%d", i)] = section
		}
		return normalizeSectionMap(sections), nil
	}

	return nil, fmt.Errorf("sections must be an object or array")
}

func normalizeSectionMap(rawSections map[string]rawDailyReportSection) map[string]dailyReportSection {
	keys := make([]string, 0, len(rawSections))
	for key := range rawSections {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	sections := make(map[string]dailyReportSection)
	for _, key := range keys {
		rawSection := rawSections[key]
		section := dailyReportSection{
			Title: strings.TrimSpace(firstNonEmpty(rawSection.Title, rawSection.Heading)),
			Items: normalizeDailyReportItems(rawSection.Items),
		}
		if section.Title == "" {
			section.Title = key
		}
		if len(section.Items) == 0 {
			continue
		}
		sections[key] = section
	}
	return sections
}

func normalizeDailyReportItems(rawItems []map[string]any) []dailyReportItem {
	items := make([]dailyReportItem, 0, len(rawItems))
	for _, rawItem := range rawItems {
		item := dailyReportItem{
			Title:   valueAsString(rawItem["title"]),
			Summary: valueAsString(rawItem["summary"]),
			Source:  valueAsString(rawItem["source"]),
			Link:    firstNonEmpty(valueAsString(rawItem["link"]), valueAsString(rawItem["url"])),
			Score:   firstNonEmpty(valueAsString(rawItem["score"]), valueAsString(rawItem["rating"])),
			Comment: firstNonEmpty(valueAsString(rawItem["comment"]), valueAsString(rawItem["commentary"])),
		}
		if item.Title == "" || item.Summary == "" {
			continue
		}
		items = append(items, item)
	}
	return items
}

func extractFirstJSONObject(raw string) (string, error) {
	start := strings.Index(raw, "{")
	if start == -1 {
		return "", fmt.Errorf("response does not contain a JSON object")
	}

	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}

		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return raw[start : i+1], nil
			}
		}
	}
	return "", fmt.Errorf("response contains an incomplete JSON object")
}

func valueAsString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int(v)) {
			return fmt.Sprintf("%d", int(v))
		}
		return strings.TrimSpace(fmt.Sprintf("%.1f", v))
	case int:
		return fmt.Sprintf("%d", v)
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
