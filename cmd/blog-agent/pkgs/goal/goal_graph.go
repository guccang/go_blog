package goal

import (
	"blog"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// GoalGraph contains the recent cross-period goals needed by the panorama view.
type GoalGraph struct {
	Year    int               `json:"year"`
	Goals   []*Goal           `json:"goals"`
	Current map[string]string `json:"current"`
}

// GetGoalGraph returns a bounded panorama instead of every historical daily record.
func GetGoalGraph(account string, year int) (*GoalGraph, error) {
	daily, weekly, monthly, yearly := CurrentPeriods()
	current := map[string]string{
		LevelDaily: daily, LevelWeekly: weekly, LevelMonthly: monthly, LevelYearly: yearly,
	}
	limits := map[string]int{LevelDaily: 14, LevelWeekly: 12, LevelMonthly: 12, LevelYearly: 1}
	levels := []string{LevelYearly, LevelMonthly, LevelWeekly, LevelDaily}
	result := &GoalGraph{Year: year, Goals: []*Goal{}, Current: current}

	for _, level := range levels {
		goals := make([]*Goal, 0)
		for _, entry := range blog.ListByTitlePrefixWithAccount(account, "目标_"+level+"_") {
			var item Goal
			if err := json.Unmarshal([]byte(entry.Content), &item); err != nil {
				continue
			}
			if year > 0 && !strings.HasPrefix(item.Period, fmt.Sprintf("%d", year)) {
				continue
			}
			for i := range item.Tasks {
				normalizeTaskPlanning(&item.Tasks[i])
			}
			goals = append(goals, &item)
		}
		sort.SliceStable(goals, func(i, j int) bool { return goals[i].Period > goals[j].Period })
		if limit := limits[level]; len(goals) > limit {
			goals = goals[:limit]
		}
		result.Goals = append(result.Goals, goals...)
	}
	return result, nil
}
