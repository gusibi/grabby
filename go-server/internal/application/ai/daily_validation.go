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
	payload, err := parseDailyReportPayload(raw, dateStr)
	if err != nil {
		return "", err
	}
	if err := validateDailyReportBudget(payload); err != nil {
		return "", err
	}
	return marshalDailyReportPayload(payload)
}

// normalizeDailyReportContentOverBudget is the last-resort path for a model
// that will not respect the item budget: it keeps the usable report and drops
// the weakest items instead of failing the whole generation.
func normalizeDailyReportContentOverBudget(raw string, dateStr string) (string, error) {
	payload, err := parseDailyReportPayload(raw, dateStr)
	if err != nil {
		return "", err
	}
	trimDailyReportToBudget(payload)
	if err := validateDailyReportBudget(payload); err != nil {
		return "", err
	}
	return marshalDailyReportPayload(payload)
}

func parseDailyReportPayload(raw string, dateStr string) (dailyReportPayload, error) {
	jsonText, err := extractFirstJSONObject(raw)
	if err != nil {
		return dailyReportPayload{}, err
	}

	var rawPayload rawDailyReportPayload
	if err := json.Unmarshal([]byte(jsonText), &rawPayload); err != nil {
		return dailyReportPayload{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return normalizeDailyReportPayload(rawPayload, dateStr)
}

func marshalDailyReportPayload(payload dailyReportPayload) (string, error) {
	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal normalized daily report: %w", err)
	}
	return string(out), nil
}

func validateDailyReportBudget(payload dailyReportPayload) error {
	if count := countDailyReportItems(payload.Sections); count > maxDailyReportItems {
		return fmt.Errorf("%w：正文共 %d 条，上限 %d 条", errDailyReportOverBudget, count, maxDailyReportItems)
	}
	return nil
}

func countDailyReportItems(sections map[string]dailyReportSection) int {
	count := 0
	for key, section := range sections {
		if key == dailyDashboardSection {
			continue
		}
		count += len(section.Items)
	}
	return count
}

// trimDailyReportToBudget removes the lowest-scored body items until the
// report fits. headline is protected so the editor's top picks survive; if
// even that cannot get under the budget, the remaining sections are trimmed.
func trimDailyReportToBudget(payload dailyReportPayload) {
	for countDailyReportItems(payload.Sections) > maxDailyReportItems {
		key, index := lowestPriorityReportItem(payload.Sections, true)
		if key == "" {
			key, index = lowestPriorityReportItem(payload.Sections, false)
		}
		if key == "" {
			return
		}
		section := payload.Sections[key]
		section.Items = append(section.Items[:index], section.Items[index+1:]...)
		if len(section.Items) == 0 {
			delete(payload.Sections, key)
			continue
		}
		payload.Sections[key] = section
	}
}

// lowestPriorityReportItem finds the item whose score is lowest. Ties break
// towards the later section key and the later position, keeping the report's
// lead material intact. When protectHeadline is set, the headline section is
// left alone.
func lowestPriorityReportItem(sections map[string]dailyReportSection, protectHeadline bool) (string, int) {
	bestKey := ""
	bestIndex := -1
	bestScore := 0
	for key, section := range sections {
		if key == dailyDashboardSection {
			continue
		}
		if protectHeadline && key == "headline" {
			continue
		}
		for index, item := range section.Items {
			score := parseReportScore(item.Score)
			if bestIndex == -1 || score < bestScore ||
				(score == bestScore && (key > bestKey || (key == bestKey && index > bestIndex))) {
				bestKey, bestIndex, bestScore = key, index, score
			}
		}
	}
	return bestKey, bestIndex
}

// parseReportScore pulls the first number out of the score field, accepting
// the shapes models actually emit: "9/10", "9", "评分 9".
func parseReportScore(raw string) int {
	value := 0
	seen := false
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			value = value*10 + int(r-'0')
			seen = true
			continue
		}
		if seen {
			break
		}
	}
	return value
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
