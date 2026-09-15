package normalize

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/config"
	"github.com/GreenSupplyChain-DIH/cow-collector/internal/models"
)

var cameraIDPattern = regexp.MustCompile(`(?i)cam[0-9]+`)
var cowIDPattern = regexp.MustCompile(`^([0-9]+)(?:_|$)`)

func Records(cfg config.Config, payloads []map[string]any, receivedAt time.Time) ([]models.TelemetryRecord, int) {
	records := make([]models.TelemetryRecord, 0, len(payloads))
	rejected := 0

	for _, payload := range payloads {
		record, ok := normalizeRecord(payload, receivedAt)
		if !ok {
			rejected++
			continue
		}
		if !cfg.AllowAllCowIDs {
			if _, exists := cfg.TargetCowIDs[record.CowID]; !exists {
				rejected++
				continue
			}
		}
		records = append(records, record)
	}
	return records, rejected
}

func normalizeRecord(payload map[string]any, receivedAt time.Time) (models.TelemetryRecord, bool) {
	identifier, ok := findIdentifier(payload,
		"cow_id",
		"animalId",
		"animal_id",
		"s4agri:animalId.value",
	)
	if !ok {
		return models.TelemetryRecord{}, false
	}
	cowID, cameraID, ok := parseIdentifier(identifier)
	if !ok {
		return models.TelemetryRecord{}, false
	}
	if cameraID == "" {
		cameraID = findString(payload, "camera_id", "cameraId", "camera")
	}

	observedAt, ok := findTime(payload,
		"observedAt",
		"timestamp",
		"time",
	)
	bodyTemperatureObservedAt := findPropertyTime(payload, "bodyTemperature", "shepherd:BodyTemperature", "temperature")
	milkYieldObservedAt := findPropertyTime(payload, "milkYield", "s4agri:MilkYield")
	milkReductionRatioObservedAt := findPropertyTime(payload, "milkReductionRatio", "shepherd:milkReductionRatio")
	elevatedTempAlertObservedAt := findPropertyTime(payload, "elevatedBodyTemperatureAlert", "shepherd:elevatedBodyTemperatureAlert")
	milkAlertObservedAt := findPropertyTime(payload, "milkAlert", "shepherd:milkAlert")
	healthAlertObservedAt := findPropertyTime(payload, "healthAlert", "shepherd:healthAlert")
	observedAt = latestTime(observedAt, ok, receivedAt.UTC(), bodyTemperatureObservedAt, milkYieldObservedAt, milkReductionRatioObservedAt, elevatedTempAlertObservedAt, milkAlertObservedAt, healthAlertObservedAt)

	record := models.TelemetryRecord{
		ObservedAt:                   observedAt,
		CowID:                        cowID,
		CameraID:                     strings.ToLower(strings.TrimSpace(cameraID)),
		BodyTemperature:              findFloatPtr(payload, "bodyTemperature", "shepherd:BodyTemperature", "temperature"),
		BodyTemperatureObservedAt:    bodyTemperatureObservedAt,
		MilkYield:                    findFloatPtr(payload, "milkYield", "s4agri:MilkYield"),
		MilkYieldObservedAt:          milkYieldObservedAt,
		MilkReductionRatio:           findFloatPtr(payload, "milkReductionRatio", "shepherd:milkReductionRatio"),
		MilkReductionRatioObservedAt: milkReductionRatioObservedAt,
		ElevatedTempAlert:            findBool(payload, "elevatedBodyTemperatureAlert", "shepherd:elevatedBodyTemperatureAlert"),
		ElevatedTempAlertObservedAt:  elevatedTempAlertObservedAt,
		MilkAlert:                    findBool(payload, "milkAlert", "shepherd:milkAlert"),
		MilkAlertObservedAt:          milkAlertObservedAt,
		HealthAlert:                  defaultAlert(findString(payload, "healthAlert", "shepherd:healthAlert")),
		HealthAlertObservedAt:        healthAlertObservedAt,
		ReceivedAt:                   receivedAt.UTC(),
	}

	return record, true
}

func findPropertyTime(payload map[string]any, keys ...string) *time.Time {
	for _, key := range keys {
		value, ok := lookup(payload, key)
		if !ok {
			continue
		}
		property, ok := value.(map[string]any)
		if !ok {
			continue
		}
		observedAt, ok := property["observedAt"].(string)
		if !ok {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, observedAt)
		if err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}

func latestTime(explicit time.Time, hasExplicit bool, fallback time.Time, timestamps ...*time.Time) time.Time {
	var latest time.Time
	if hasExplicit {
		latest = explicit.UTC()
	}
	for _, timestamp := range timestamps {
		if timestamp != nil && timestamp.After(latest) {
			latest = timestamp.UTC()
		}
	}
	if latest.IsZero() {
		return fallback.UTC()
	}
	return latest
}

func findIdentifier(payload map[string]any, keys ...string) (any, bool) {
	for _, key := range keys {
		value, ok := lookup(payload, key)
		if !ok {
			continue
		}
		if nested, ok := value.(map[string]any); ok {
			value, ok = nested["value"]
			if !ok {
				continue
			}
		}
		return value, true
	}
	return nil, false
}

func parseIdentifier(value any) (int, string, bool) {
	var text string
	switch typed := value.(type) {
	case int:
		return typed, "", true
	case float64:
		return int(typed), "", true
	case string:
		text = strings.TrimSpace(typed)
	default:
		return 0, "", false
	}
	if text == "" {
		return 0, "", false
	}

	cowIDText := text
	if match := cowIDPattern.FindStringSubmatch(text); len(match) == 2 {
		cowIDText = match[1]
	}
	cowID, err := strconv.Atoi(cowIDText)
	if err != nil {
		return 0, "", false
	}
	cameraID := cameraIDPattern.FindString(strings.ToLower(text))
	return cowID, cameraID, true
}

func defaultAlert(value string) string {
	if strings.TrimSpace(value) == "" {
		return "NONE"
	}
	return strings.ToUpper(strings.TrimSpace(value))
}

func findString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := lookup(payload, key); ok {
			switch typed := value.(type) {
			case string:
				return typed
			case map[string]any:
				if nested, ok := typed["value"].(string); ok {
					return nested
				}
			}
		}
	}
	return ""
}

func findInt(payload map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := lookup(payload, key); ok {
			switch typed := value.(type) {
			case int:
				return typed, true
			case float64:
				return int(typed), true
			case string:
				parsed, err := strconv.Atoi(strings.TrimSpace(typed))
				if err == nil {
					return parsed, true
				}
			case map[string]any:
				if nested, ok := typed["value"]; ok {
					return findInt(map[string]any{"value": nested}, "value")
				}
			}
		}
	}
	return 0, false
}

func findFloatPtr(payload map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if value, ok := lookup(payload, key); ok {
			switch typed := value.(type) {
			case float64:
				return &typed
			case int:
				asFloat := float64(typed)
				return &asFloat
			case string:
				parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
				if err == nil {
					return &parsed
				}
			case map[string]any:
				if nested, ok := typed["value"]; ok {
					return findFloatPtr(map[string]any{"value": nested}, "value")
				}
			}
		}
	}
	return nil
}

func findBool(payload map[string]any, keys ...string) bool {
	for _, key := range keys {
		if value, ok := lookup(payload, key); ok {
			switch typed := value.(type) {
			case bool:
				return typed
			case string:
				return strings.EqualFold(strings.TrimSpace(typed), "true")
			case map[string]any:
				if nested, ok := typed["value"]; ok {
					return findBool(map[string]any{"value": nested}, "value")
				}
			}
		}
	}
	return false
}

func findTime(payload map[string]any, keys ...string) (time.Time, bool) {
	for _, key := range keys {
		value, ok := lookup(payload, key)
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			if nested, nestedOK := value.(map[string]any); nestedOK {
				if inner, ok := nested["value"].(string); ok {
					text = inner
				} else {
					continue
				}
			} else {
				continue
			}
		}

		parsed, err := time.Parse(time.RFC3339, text)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func lookup(payload map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	var current any = payload
	for _, part := range parts {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func ValidateTableNames(schema string, table string) error {
	if strings.TrimSpace(schema) == "" || strings.TrimSpace(table) == "" {
		return fmt.Errorf("schema and table names are required")
	}
	return nil
}
