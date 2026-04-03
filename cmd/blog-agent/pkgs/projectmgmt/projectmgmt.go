package projectmgmt

import (
	"blog"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"sort"
	"strings"
	"time"
)

const (
	projectBlogPrefix = "projectmgmt_"
	projectBlogTag    = "projectmgmt"
	timeLayout        = "2006-01-02 15:04:05"
	dateLayout        = "2006-01-02"
)

type Project struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Priority    string            `json:"priority"`
	Owner       string            `json:"owner"`
	Tags        []string          `json:"tags,omitempty"`
	StartDate   string            `json:"start_date,omitempty"`
	EndDate     string            `json:"end_date,omitempty"`
	CreatedAt   string            `json:"created_at"`
	UpdatedAt   string            `json:"updated_at"`
	Goals       []Goal            `json:"goals"`
	OKRs        []OKR             `json:"okrs"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type Goal struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Progress    int    `json:"progress"`
	Priority    string `json:"priority"`
	StartDate   string `json:"start_date,omitempty"`
	EndDate     string `json:"end_date,omitempty"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type OKR struct {
	ID         string      `json:"id"`
	Objective  string      `json:"objective"`
	Status     string      `json:"status"`
	Progress   int         `json:"progress"`
	Period     string      `json:"period,omitempty"`
	KeyResults []KeyResult `json:"key_results"`
	CreatedAt  string      `json:"created_at"`
	UpdatedAt  string      `json:"updated_at"`
}

type KeyResult struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	MetricType   string  `json:"metric_type"`
	TargetValue  float64 `json:"target_value"`
	CurrentValue float64 `json:"current_value"`
	Unit         string  `json:"unit,omitempty"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type ProjectSummary struct {
	TotalProjects  int `json:"total_projects"`
	PlanningCount  int `json:"planning_count"`
	ActiveCount    int `json:"active_count"`
	OnHoldCount    int `json:"on_hold_count"`
	CompletedCount int `json:"completed_count"`
	CancelledCount int `json:"cancelled_count"`
	OverdueCount   int `json:"overdue_count"`
	ActiveOKRCount int `json:"active_okr_count"`
	CompletedOKRs  int `json:"completed_okr_count"`
	TotalGoalCount int `json:"total_goal_count"`
	TotalKRCount   int `json:"total_key_result_count"`
}

func nowString() string {
	return time.Now().Format(timeLayout)
}

func generateID(prefix string) string {
	return fmt.Sprintf("%s%d", prefix, time.Now().UnixNano())
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, tag)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func validateStatus(status string, allowed ...string) error {
	if status == "" {
		return fmt.Errorf("status is required")
	}
	for _, item := range allowed {
		if status == item {
			return nil
		}
	}
	return fmt.Errorf("invalid status: %s", status)
}

func validatePriority(priority string) error {
	switch priority {
	case "low", "medium", "high", "urgent":
		return nil
	default:
		return fmt.Errorf("invalid priority: %s", priority)
	}
}

func validateOptionalDate(date string) error {
	if strings.TrimSpace(date) == "" {
		return nil
	}
	if _, err := time.Parse(dateLayout, date); err != nil {
		return fmt.Errorf("invalid date format %q, expected YYYY-MM-DD", date)
	}
	return nil
}

func validateProject(p *Project) error {
	if p == nil {
		return fmt.Errorf("project is nil")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("project name is required")
	}
	if err := validateStatus(p.Status, "planning", "active", "on_hold", "completed", "cancelled"); err != nil {
		return err
	}
	if err := validatePriority(p.Priority); err != nil {
		return err
	}
	if err := validateOptionalDate(p.StartDate); err != nil {
		return err
	}
	if err := validateOptionalDate(p.EndDate); err != nil {
		return err
	}
	return nil
}

func validateGoal(goal *Goal) error {
	if goal == nil {
		return fmt.Errorf("goal is nil")
	}
	if strings.TrimSpace(goal.Title) == "" {
		return fmt.Errorf("goal title is required")
	}
	if err := validateStatus(goal.Status, "pending", "in_progress", "completed", "cancelled"); err != nil {
		return err
	}
	if goal.Progress < 0 || goal.Progress > 100 {
		return fmt.Errorf("goal progress must be between 0 and 100")
	}
	if err := validatePriority(goal.Priority); err != nil {
		return err
	}
	if err := validateOptionalDate(goal.StartDate); err != nil {
		return err
	}
	if err := validateOptionalDate(goal.EndDate); err != nil {
		return err
	}
	return nil
}

func validateKeyResult(kr *KeyResult) error {
	if kr == nil {
		return fmt.Errorf("key result is nil")
	}
	if strings.TrimSpace(kr.Title) == "" {
		return fmt.Errorf("key result title is required")
	}
	if strings.TrimSpace(kr.MetricType) == "" {
		return fmt.Errorf("metric_type is required")
	}
	if err := validateStatus(kr.Status, "pending", "in_progress", "completed", "cancelled"); err != nil {
		return err
	}
	return nil
}

func validateOKR(okr *OKR) error {
	if okr == nil {
		return fmt.Errorf("okr is nil")
	}
	if strings.TrimSpace(okr.Objective) == "" {
		return fmt.Errorf("objective is required")
	}
	if err := validateStatus(okr.Status, "draft", "active", "at_risk", "completed", "cancelled"); err != nil {
		return err
	}
	if okr.Progress < 0 || okr.Progress > 100 {
		return fmt.Errorf("okr progress must be between 0 and 100")
	}
	for i := range okr.KeyResults {
		if err := validateKeyResult(&okr.KeyResults[i]); err != nil {
			return err
		}
	}
	return nil
}

func projectTitle(projectID string) string {
	return projectBlogPrefix + projectID
}

func prepareProjectForSave(project *Project, account string, isNew bool) (*Project, error) {
	if project == nil {
		return nil, fmt.Errorf("project is nil")
	}
	copyProject := *project
	copyProject.Name = strings.TrimSpace(copyProject.Name)
	copyProject.Description = strings.TrimSpace(copyProject.Description)
	copyProject.Owner = strings.TrimSpace(copyProject.Owner)
	copyProject.Tags = normalizeTags(copyProject.Tags)
	if copyProject.Owner == "" {
		copyProject.Owner = account
	}
	if copyProject.ID == "" {
		copyProject.ID = generateID("proj_")
	}
	if copyProject.Status == "" {
		copyProject.Status = "planning"
	}
	if copyProject.Priority == "" {
		copyProject.Priority = "medium"
	}
	now := nowString()
	if isNew {
		copyProject.CreatedAt = now
	}
	if copyProject.CreatedAt == "" {
		copyProject.CreatedAt = now
	}
	copyProject.UpdatedAt = now
	if copyProject.Goals == nil {
		copyProject.Goals = []Goal{}
	}
	if copyProject.OKRs == nil {
		copyProject.OKRs = []OKR{}
	}
	if copyProject.Metadata == nil {
		copyProject.Metadata = map[string]string{}
	}
	for i := range copyProject.Goals {
		if err := validateGoal(&copyProject.Goals[i]); err != nil {
			return nil, err
		}
	}
	for i := range copyProject.OKRs {
		if err := validateOKR(&copyProject.OKRs[i]); err != nil {
			return nil, err
		}
	}
	if err := validateProject(&copyProject); err != nil {
		return nil, err
	}
	return &copyProject, nil
}

func saveProjectWithAccount(account string, project *Project, isNew bool) error {
	prepared, err := prepareProjectForSave(project, account, isNew)
	if err != nil {
		return err
	}
	content, err := json.MarshalIndent(prepared, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project: %w", err)
	}
	udb := &module.UploadedBlogData{
		Title:    projectTitle(prepared.ID),
		Content:  string(content),
		Tags:     projectBlogTag,
		AuthType: module.EAuthType_private,
		Account:  account,
	}
	existing := blog.GetBlogWithAccount(account, udb.Title)
	if existing == nil {
		if ret := blog.AddBlogWithAccount(account, udb); ret != 0 {
			return fmt.Errorf("failed to create project")
		}
	} else {
		if ret := blog.ModifyBlogWithAccount(account, udb); ret != 0 {
			return fmt.Errorf("failed to update project")
		}
	}
	return nil
}

func parseProjectFromBlog(content string) (*Project, error) {
	var project Project
	if err := json.Unmarshal([]byte(content), &project); err != nil {
		return nil, fmt.Errorf("parse project: %w", err)
	}
	if project.Goals == nil {
		project.Goals = []Goal{}
	}
	if project.OKRs == nil {
		project.OKRs = []OKR{}
	}
	if project.Metadata == nil {
		project.Metadata = map[string]string{}
	}
	return &project, nil
}

func findGoalIndex(project *Project, goalID string) int {
	for i := range project.Goals {
		if project.Goals[i].ID == goalID {
			return i
		}
	}
	return -1
}

func findOKRIndex(project *Project, okrID string) int {
	for i := range project.OKRs {
		if project.OKRs[i].ID == okrID {
			return i
		}
	}
	return -1
}

func findKRIndex(okr *OKR, krID string) int {
	for i := range okr.KeyResults {
		if okr.KeyResults[i].ID == krID {
			return i
		}
	}
	return -1
}

func CreateProjectWithAccount(account string, project *Project) (*Project, error) {
	if account == "" {
		return nil, fmt.Errorf("account is required")
	}
	prepared, err := prepareProjectForSave(project, account, true)
	if err != nil {
		return nil, err
	}
	if err := saveProjectWithAccount(account, prepared, true); err != nil {
		return nil, err
	}
	log.DebugF(log.ModuleBlog, "created project %s for %s", prepared.ID, account)
	return GetProjectWithAccount(account, prepared.ID)
}

func GetProjectWithAccount(account, projectID string) (*Project, error) {
	if account == "" || strings.TrimSpace(projectID) == "" {
		return nil, fmt.Errorf("account and projectID are required")
	}
	b := blog.GetBlogWithAccount(account, projectTitle(projectID))
	if b == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}
	return parseProjectFromBlog(b.Content)
}

func ListProjectsWithAccount(account, status string) ([]Project, error) {
	projects := make([]Project, 0)
	for _, b := range blog.GetBlogsWithAccount(account) {
		if !strings.HasPrefix(b.Title, projectBlogPrefix) {
			continue
		}
		project, err := parseProjectFromBlog(b.Content)
		if err != nil {
			log.ErrorF(log.ModuleBlog, "parse project failed title=%s err=%v", b.Title, err)
			continue
		}
		if status != "" && project.Status != status {
			continue
		}
		projects = append(projects, *project)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].UpdatedAt > projects[j].UpdatedAt
	})
	return projects, nil
}

func UpdateProjectWithAccount(account string, project *Project) error {
	if account == "" {
		return fmt.Errorf("account is required")
	}
	if project == nil || strings.TrimSpace(project.ID) == "" {
		return fmt.Errorf("project id is required")
	}
	existing, err := GetProjectWithAccount(account, project.ID)
	if err != nil {
		return err
	}
	if project.CreatedAt == "" {
		project.CreatedAt = existing.CreatedAt
	}
	return saveProjectWithAccount(account, project, false)
}

func DeleteProjectWithAccount(account, projectID string) error {
	if account == "" || strings.TrimSpace(projectID) == "" {
		return fmt.Errorf("account and projectID are required")
	}
	target := blog.GetBlogWithAccount(account, projectTitle(projectID))
	if target == nil {
		return fmt.Errorf("project not found: %s", projectID)
	}
	if blog.DeleteBlogWithAccount(account, projectTitle(projectID)) != 0 {
		return fmt.Errorf("failed to delete project")
	}
	return nil
}

func AddGoalWithAccount(account, projectID string, goal Goal) (*Goal, error) {
	project, err := GetProjectWithAccount(account, projectID)
	if err != nil {
		return nil, err
	}
	now := nowString()
	if goal.ID == "" {
		goal.ID = generateID("goal_")
	}
	if goal.Status == "" {
		goal.Status = "pending"
	}
	if goal.Priority == "" {
		goal.Priority = "medium"
	}
	goal.CreatedAt = now
	goal.UpdatedAt = now
	if err := validateGoal(&goal); err != nil {
		return nil, err
	}
	project.Goals = append(project.Goals, goal)
	if err := UpdateProjectWithAccount(account, project); err != nil {
		return nil, err
	}
	return &goal, nil
}

func UpdateGoalWithAccount(account, projectID string, goal Goal) error {
	project, err := GetProjectWithAccount(account, projectID)
	if err != nil {
		return err
	}
	idx := findGoalIndex(project, goal.ID)
	if idx < 0 {
		return fmt.Errorf("goal not found: %s", goal.ID)
	}
	goal.CreatedAt = project.Goals[idx].CreatedAt
	goal.UpdatedAt = nowString()
	if err := validateGoal(&goal); err != nil {
		return err
	}
	project.Goals[idx] = goal
	return UpdateProjectWithAccount(account, project)
}

func DeleteGoalWithAccount(account, projectID, goalID string) error {
	project, err := GetProjectWithAccount(account, projectID)
	if err != nil {
		return err
	}
	idx := findGoalIndex(project, goalID)
	if idx < 0 {
		return fmt.Errorf("goal not found: %s", goalID)
	}
	project.Goals = append(project.Goals[:idx], project.Goals[idx+1:]...)
	return UpdateProjectWithAccount(account, project)
}

func AddOKRWithAccount(account, projectID string, okr OKR) (*OKR, error) {
	project, err := GetProjectWithAccount(account, projectID)
	if err != nil {
		return nil, err
	}
	now := nowString()
	if okr.ID == "" {
		okr.ID = generateID("okr_")
	}
	if okr.Status == "" {
		okr.Status = "draft"
	}
	if okr.KeyResults == nil {
		okr.KeyResults = []KeyResult{}
	}
	for i := range okr.KeyResults {
		if okr.KeyResults[i].ID == "" {
			okr.KeyResults[i].ID = generateID("kr_")
		}
		if okr.KeyResults[i].Status == "" {
			okr.KeyResults[i].Status = "pending"
		}
		okr.KeyResults[i].CreatedAt = now
		okr.KeyResults[i].UpdatedAt = now
	}
	okr.CreatedAt = now
	okr.UpdatedAt = now
	if err := validateOKR(&okr); err != nil {
		return nil, err
	}
	project.OKRs = append(project.OKRs, okr)
	if err := UpdateProjectWithAccount(account, project); err != nil {
		return nil, err
	}
	return &okr, nil
}

func UpdateOKRWithAccount(account, projectID string, okr OKR) error {
	project, err := GetProjectWithAccount(account, projectID)
	if err != nil {
		return err
	}
	idx := findOKRIndex(project, okr.ID)
	if idx < 0 {
		return fmt.Errorf("okr not found: %s", okr.ID)
	}
	okr.CreatedAt = project.OKRs[idx].CreatedAt
	okr.UpdatedAt = nowString()
	for i := range okr.KeyResults {
		if okr.KeyResults[i].CreatedAt == "" {
			existingIdx := findKRIndex(&project.OKRs[idx], okr.KeyResults[i].ID)
			if existingIdx >= 0 {
				okr.KeyResults[i].CreatedAt = project.OKRs[idx].KeyResults[existingIdx].CreatedAt
			} else {
				okr.KeyResults[i].CreatedAt = nowString()
			}
		}
		if okr.KeyResults[i].UpdatedAt == "" {
			okr.KeyResults[i].UpdatedAt = nowString()
		}
	}
	if err := validateOKR(&okr); err != nil {
		return err
	}
	project.OKRs[idx] = okr
	return UpdateProjectWithAccount(account, project)
}

func DeleteOKRWithAccount(account, projectID, okrID string) error {
	project, err := GetProjectWithAccount(account, projectID)
	if err != nil {
		return err
	}
	idx := findOKRIndex(project, okrID)
	if idx < 0 {
		return fmt.Errorf("okr not found: %s", okrID)
	}
	project.OKRs = append(project.OKRs[:idx], project.OKRs[idx+1:]...)
	return UpdateProjectWithAccount(account, project)
}

func UpdateKeyResultWithAccount(account, projectID, okrID string, kr KeyResult) error {
	project, err := GetProjectWithAccount(account, projectID)
	if err != nil {
		return err
	}
	okrIdx := findOKRIndex(project, okrID)
	if okrIdx < 0 {
		return fmt.Errorf("okr not found: %s", okrID)
	}
	krIdx := findKRIndex(&project.OKRs[okrIdx], kr.ID)
	now := nowString()
	if krIdx < 0 {
		if kr.ID == "" {
			kr.ID = generateID("kr_")
		}
		if kr.Status == "" {
			kr.Status = "pending"
		}
		kr.CreatedAt = now
		kr.UpdatedAt = now
		if err := validateKeyResult(&kr); err != nil {
			return err
		}
		project.OKRs[okrIdx].KeyResults = append(project.OKRs[okrIdx].KeyResults, kr)
	} else {
		kr.CreatedAt = project.OKRs[okrIdx].KeyResults[krIdx].CreatedAt
		kr.UpdatedAt = now
		if err := validateKeyResult(&kr); err != nil {
			return err
		}
		project.OKRs[okrIdx].KeyResults[krIdx] = kr
	}
	project.OKRs[okrIdx].UpdatedAt = now
	return UpdateProjectWithAccount(account, project)
}

func GetProjectSummaryWithAccount(account string) (*ProjectSummary, error) {
	projects, err := ListProjectsWithAccount(account, "")
	if err != nil {
		return nil, err
	}
	summary := &ProjectSummary{TotalProjects: len(projects)}
	now := time.Now()
	for _, project := range projects {
		switch project.Status {
		case "planning":
			summary.PlanningCount++
		case "active":
			summary.ActiveCount++
		case "on_hold":
			summary.OnHoldCount++
		case "completed":
			summary.CompletedCount++
		case "cancelled":
			summary.CancelledCount++
		}
		if project.EndDate != "" && project.Status != "completed" && project.Status != "cancelled" {
			if endDate, err := time.Parse(dateLayout, project.EndDate); err == nil && endDate.Before(now) {
				summary.OverdueCount++
			}
		}
		summary.TotalGoalCount += len(project.Goals)
		for _, okr := range project.OKRs {
			switch okr.Status {
			case "active":
				summary.ActiveOKRCount++
			case "completed":
				summary.CompletedOKRs++
			}
			summary.TotalKRCount += len(okr.KeyResults)
		}
	}
	return summary, nil
}
