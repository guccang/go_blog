package persistence

import (
	"encoding/json"
	"sort"
	"strings"
	"time"
)

type HookInsightFilter struct {
	Feature   string
	EventType string
	Status    string
}

type HookInsightSummary struct {
	TotalEvents int     `json:"total_events"`
	TodayEvents int     `json:"today_events"`
	ActiveDays  int     `json:"active_days"`
	SuccessRate float64 `json:"success_rate"`
}

type HookInsightTimelineItem struct {
	ID         int64  `json:"id"`
	EventType  string `json:"event_type"`
	Feature    string `json:"feature"`
	ObjectType string `json:"object_type"`
	ObjectID   string `json:"object_id"`
	Title      string `json:"title"`
	Query      string `json:"query"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
}

type HookInsightHeatCell struct {
	Date  string `json:"date"`
	Hour  int    `json:"hour"`
	Count int    `json:"count"`
}

type HookInsightFeatureStat struct {
	Feature string `json:"feature"`
	Count   int    `json:"count"`
	Success int    `json:"success"`
	Failed  int    `json:"failed"`
}

type HookInsightPathStat struct {
	Steps []string `json:"steps"`
	Count int      `json:"count"`
}

type HookInsights struct {
	Days              int                       `json:"days"`
	GeneratedAt       string                    `json:"generated_at"`
	Summary           HookInsightSummary        `json:"summary"`
	Timeline          []HookInsightTimelineItem `json:"timeline"`
	Heatmap           []HookInsightHeatCell     `json:"heatmap"`
	Features          []HookInsightFeatureStat  `json:"features"`
	Paths             []HookInsightPathStat     `json:"paths"`
	AvailableFeatures []string                  `json:"available_features"`
	AvailableEvents   []string                  `json:"available_events"`
}

type hookInsightEvent struct {
	BlogHookEvent
	Status string
	Time   time.Time
}

func GetHookInsights(account string, days int, filter HookInsightFilter) (HookInsights, error) {
	if days < 1 || days > 30 {
		days = 7
	}
	now := time.Now()
	start := now.AddDate(0, 0, -(days - 1))
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	rows, err := requireSQLite().Query(`SELECT id,account,session_id,event_type,feature,object_type,object_id,title,query,context_json,result_json,created_at
		FROM (
			SELECT id,account,session_id,event_type,feature,object_type,object_id,title,query,context_json,result_json,created_at
			FROM blog_hooks WHERE account=? AND created_at>=? ORDER BY id DESC LIMIT 5000
		) ORDER BY id ASC`, account, start.Format("2006-01-02 15:04:05"))
	if err != nil {
		return HookInsights{}, err
	}
	defer rows.Close()

	events := make([]BlogHookEvent, 0, 256)
	for rows.Next() {
		var event BlogHookEvent
		if err := rows.Scan(&event.ID, &event.Account, &event.SessionID, &event.EventType, &event.Feature, &event.ObjectType, &event.ObjectID, &event.Title, &event.Query, &event.ContextJSON, &event.ResultJSON, &event.CreatedAt); err != nil {
			return HookInsights{}, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return HookInsights{}, err
	}
	return buildHookInsights(events, days, filter, now), nil
}

func buildHookInsights(events []BlogHookEvent, days int, filter HookInsightFilter, now time.Time) HookInsights {
	location := now.Location()
	today := now.Format("2006-01-02")
	allFeatures := map[string]struct{}{}
	allEvents := map[string]struct{}{}
	filtered := make([]hookInsightEvent, 0, len(events))
	for _, event := range events {
		if event.Feature != "" {
			allFeatures[event.Feature] = struct{}{}
		}
		if event.EventType != "" {
			allEvents[event.EventType] = struct{}{}
		}
		status := hookResultStatus(event.ResultJSON)
		if filter.Feature != "" && event.Feature != filter.Feature {
			continue
		}
		if filter.EventType != "" && event.EventType != filter.EventType {
			continue
		}
		if filter.Status != "" && status != filter.Status {
			continue
		}
		occurredAt, err := time.ParseInLocation("2006-01-02 15:04:05", event.CreatedAt, location)
		if err != nil {
			continue
		}
		filtered = append(filtered, hookInsightEvent{BlogHookEvent: event, Status: status, Time: occurredAt})
	}

	insights := HookInsights{
		Days:              days,
		GeneratedAt:       now.Format("2006-01-02 15:04:05"),
		Timeline:          []HookInsightTimelineItem{},
		Heatmap:           []HookInsightHeatCell{},
		Features:          []HookInsightFeatureStat{},
		Paths:             []HookInsightPathStat{},
		AvailableFeatures: sortedStringSet(allFeatures),
		AvailableEvents:   sortedStringSet(allEvents),
	}
	insights.Summary.TotalEvents = len(filtered)

	activeDays := map[string]struct{}{}
	heatmap := map[string]int{}
	featureStats := map[string]*HookInsightFeatureStat{}
	successCount := 0
	failedCount := 0
	for index := len(filtered) - 1; index >= 0; index-- {
		event := filtered[index]
		date := event.Time.Format("2006-01-02")
		activeDays[date] = struct{}{}
		heatmap[date+"-"+event.Time.Format("15")]++
		if date == today {
			insights.Summary.TodayEvents++
			if len(insights.Timeline) < 100 {
				insights.Timeline = append(insights.Timeline, HookInsightTimelineItem{
					ID: event.ID, EventType: event.EventType, Feature: event.Feature,
					ObjectType: event.ObjectType, ObjectID: event.ObjectID, Title: event.Title,
					Query: event.Query, Status: event.Status, CreatedAt: event.CreatedAt,
				})
			}
		}
		feature := event.Feature
		if feature == "" {
			feature = "未分类"
		}
		stat := featureStats[feature]
		if stat == nil {
			stat = &HookInsightFeatureStat{Feature: feature}
			featureStats[feature] = stat
		}
		stat.Count++
		switch event.Status {
		case "success":
			stat.Success++
			successCount++
		case "error":
			stat.Failed++
			failedCount++
		}
	}
	insights.Summary.ActiveDays = len(activeDays)
	if successCount+failedCount > 0 {
		insights.Summary.SuccessRate = float64(successCount) * 100 / float64(successCount+failedCount)
	} else {
		insights.Summary.SuccessRate = -1
	}

	for key, count := range heatmap {
		parts := strings.Split(key, "-")
		hour := 0
		if len(parts) == 4 {
			hour = int(parts[3][0]-'0')*10 + int(parts[3][1]-'0')
		}
		insights.Heatmap = append(insights.Heatmap, HookInsightHeatCell{
			Date: strings.Join(parts[:3], "-"), Hour: hour, Count: count,
		})
	}
	sort.Slice(insights.Heatmap, func(i, j int) bool {
		if insights.Heatmap[i].Date == insights.Heatmap[j].Date {
			return insights.Heatmap[i].Hour < insights.Heatmap[j].Hour
		}
		return insights.Heatmap[i].Date < insights.Heatmap[j].Date
	})

	for _, stat := range featureStats {
		insights.Features = append(insights.Features, *stat)
	}
	sort.Slice(insights.Features, func(i, j int) bool {
		if insights.Features[i].Count == insights.Features[j].Count {
			return insights.Features[i].Feature < insights.Features[j].Feature
		}
		return insights.Features[i].Count > insights.Features[j].Count
	})
	if len(insights.Features) > 10 {
		insights.Features = insights.Features[:10]
	}
	insights.Paths = buildHookPaths(filtered)
	return insights
}

func hookResultStatus(raw string) string {
	var value struct {
		Status string `json:"status"`
	}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return "unknown"
	}
	switch strings.ToLower(strings.TrimSpace(value.Status)) {
	case "success", "ok", "completed":
		return "success"
	case "error", "failed", "failure":
		return "error"
	default:
		return "unknown"
	}
}

func buildHookPaths(events []hookInsightEvent) []HookInsightPathStat {
	counts := map[string]int{}
	var segment []string
	var previous hookInsightEvent
	flush := func() {
		if len(segment) < 2 {
			segment = nil
			return
		}
		windowSize := 3
		if len(segment) < windowSize {
			windowSize = 2
		}
		for index := 0; index+windowSize <= len(segment); index++ {
			counts[strings.Join(segment[index:index+windowSize], "\x00")]++
		}
		segment = nil
	}
	for index, event := range events {
		if index > 0 && (event.SessionID != previous.SessionID || event.Time.Sub(previous.Time) > 30*time.Minute) {
			flush()
		}
		step := event.Feature
		if step == "" {
			step = event.EventType
		}
		if len(segment) == 0 || segment[len(segment)-1] != step {
			segment = append(segment, step)
		}
		previous = event
	}
	flush()

	paths := make([]HookInsightPathStat, 0, len(counts))
	for key, count := range counts {
		paths = append(paths, HookInsightPathStat{Steps: strings.Split(key, "\x00"), Count: count})
	}
	sort.Slice(paths, func(i, j int) bool {
		if paths[i].Count == paths[j].Count {
			return strings.Join(paths[i].Steps, "\x00") < strings.Join(paths[j].Steps, "\x00")
		}
		return paths[i].Count > paths[j].Count
	})
	if len(paths) > 5 {
		paths = paths[:5]
	}
	return paths
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
