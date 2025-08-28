package exercise

import (
	"account"
	"core"
)

// cmd definitions

// AddExerciseCmd adds a new exercise item
type AddExerciseCmd struct {
	core.ActorCommand
	Account   string
	Date      string
	Name      string
	Type      string
	Duration  int
	Intensity string
	Calories  int
	Notes     string
	Weight    float64
	BodyParts []string
}

func (cmd *AddExerciseCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	item, err := exerciseActor.addExercise(cmd.Account, cmd.Date, cmd.Name, cmd.Type, cmd.Duration, cmd.Intensity, cmd.Calories, cmd.Notes, cmd.Weight, cmd.BodyParts)
	if err != nil {
		cmd.Response() <- nil
		cmd.Response() <- err
	} else {
		cmd.Response() <- item
		cmd.Response() <- nil
	}
}

// DeleteExerciseCmd removes an exercise item by ID
type DeleteExerciseCmd struct {
	core.ActorCommand
	Account string
	Date    string
	ID      string
}

func (cmd *DeleteExerciseCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.deleteExercise(cmd.Account, cmd.Date, cmd.ID)
	cmd.Response() <- err
}

// UpdateExerciseCmd updates an existing exercise item
type UpdateExerciseCmd struct {
	core.ActorCommand
	Account   string
	Date      string
	ID        string
	Name      string
	Type      string
	Duration  int
	Intensity string
	Calories  int
	Notes     string
	Weight    float64
	BodyParts []string
}

func (cmd *UpdateExerciseCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.updateExercise(cmd.Account, cmd.Date, cmd.ID, cmd.Name, cmd.Type, cmd.Duration, cmd.Intensity, cmd.Calories, cmd.Notes, cmd.Weight, cmd.BodyParts)
	cmd.Response() <- err
}

// ToggleExerciseCmd toggles the completion status of an exercise item
type ToggleExerciseCmd struct {
	core.ActorCommand
	Account string
	Date    string
	ID      string
}

func (cmd *ToggleExerciseCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.toggleExercise(cmd.Account, cmd.Date, cmd.ID)
	cmd.Response() <- err
}

// GetExercisesByDateCmd retrieves the exercise list for a specific date
type GetExercisesByDateCmd struct {
	core.ActorCommand
	Account string
	Date    string
}

func (cmd *GetExercisesByDateCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	exercises, err := exerciseActor.getExercisesByDate(cmd.Account, cmd.Date)
	cmd.Response() <- exercises
	cmd.Response() <- err
}

// GetAllExercisesCmd retrieves all exercise lists
type GetAllExercisesCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetAllExercisesCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	exercises, err := exerciseActor.getAllExercises(cmd.Account)
	cmd.Response() <- exercises
	cmd.Response() <- err
}

// AddTemplateCmd adds a new exercise template
type AddTemplateCmd struct {
	core.ActorCommand
	Account   string
	Name      string
	Type      string
	Duration  int
	Intensity string
	Calories  int
	Notes     string
	Weight    float64
	BodyParts []string
}

func (cmd *AddTemplateCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	template, err := exerciseActor.addTemplate(cmd.Account, cmd.Name, cmd.Type, cmd.Duration, cmd.Intensity, cmd.Calories, cmd.Notes, cmd.Weight, cmd.BodyParts)
	if err != nil {
		cmd.Response() <- nil
		cmd.Response() <- err
	} else {
		cmd.Response() <- template
		cmd.Response() <- nil
	}
}

// DeleteTemplateCmd removes a template by ID
type DeleteTemplateCmd struct {
	core.ActorCommand
	Account string
	ID      string
}

func (cmd *DeleteTemplateCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.deleteTemplate(cmd.Account, cmd.ID)
	cmd.Response() <- err
}

// UpdateTemplateCmd updates an existing template
type UpdateTemplateCmd struct {
	core.ActorCommand
	Account   string
	ID        string
	Name      string
	Type      string
	Duration  int
	Intensity string
	Calories  int
	Notes     string
	Weight    float64
	BodyParts []string
}

func (cmd *UpdateTemplateCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.updateTemplate(cmd.Account, cmd.ID, cmd.Name, cmd.Type, cmd.Duration, cmd.Intensity, cmd.Calories, cmd.Notes, cmd.Weight, cmd.BodyParts)
	cmd.Response() <- err
}

// GetTemplatesCmd retrieves all exercise templates
type GetTemplatesCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetTemplatesCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	templates, err := exerciseActor.getTemplates(cmd.Account)
	cmd.Response() <- templates
	cmd.Response() <- err
}

// GetWeeklyStatsCmd calculates weekly exercise statistics
type GetWeeklyStatsCmd struct {
	core.ActorCommand
	Account   string
	StartDate string
}

func (cmd *GetWeeklyStatsCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	stats, err := exerciseActor.getWeeklyStats(cmd.Account, cmd.StartDate)
	cmd.Response() <- stats
	cmd.Response() <- err
}

// GetMonthlyStatsCmd calculates monthly exercise statistics
type GetMonthlyStatsCmd struct {
	core.ActorCommand
	Account string
	Year    int
	Month   int
}

func (cmd *GetMonthlyStatsCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	stats, err := exerciseActor.getMonthlyStats(cmd.Account, cmd.Year, cmd.Month)
	cmd.Response() <- stats
	cmd.Response() <- err
}

// GetYearlyStatsCmd calculates yearly exercise statistics
type GetYearlyStatsCmd struct {
	core.ActorCommand
	Account string
	Year    int
}

func (cmd *GetYearlyStatsCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	stats, err := exerciseActor.getYearlyStats(cmd.Account, cmd.Year)
	cmd.Response() <- stats
	cmd.Response() <- err
}

// AddCollectionCmd adds a new template collection
type AddCollectionCmd struct {
	core.ActorCommand
	Account     string
	Name        string
	Description string
	TemplateIDs []string
}

func (cmd *AddCollectionCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	collection, err := exerciseActor.addCollection(cmd.Account, cmd.Name, cmd.Description, cmd.TemplateIDs)
	if err != nil {
		cmd.Response() <- nil
		cmd.Response() <- err
	} else {
		cmd.Response() <- collection
		cmd.Response() <- nil
	}
}

// DeleteCollectionCmd removes a collection by ID
type DeleteCollectionCmd struct {
	core.ActorCommand
	Account string
	ID      string
}

func (cmd *DeleteCollectionCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.deleteCollection(cmd.Account, cmd.ID)
	cmd.Response() <- err
}

// UpdateCollectionCmd updates an existing collection
type UpdateCollectionCmd struct {
	core.ActorCommand
	Account     string
	ID          string
	Name        string
	Description string
	TemplateIDs []string
}

func (cmd *UpdateCollectionCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.updateCollection(cmd.Account, cmd.ID, cmd.Name, cmd.Description, cmd.TemplateIDs)
	cmd.Response() <- err
}

// GetCollectionsCmd retrieves all template collections
type GetCollectionsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetCollectionsCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	collections, err := exerciseActor.getCollections(cmd.Account)
	cmd.Response() <- collections
	cmd.Response() <- err
}

// GetCollectionWithTemplatesCmd retrieves a collection with its associated templates
type GetCollectionWithTemplatesCmd struct {
	core.ActorCommand
	Account      string
	CollectionID string
}

func (cmd *GetCollectionWithTemplatesCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	collection, templates, err := exerciseActor.getCollectionWithTemplates(cmd.Account, cmd.CollectionID)
	cmd.Response() <- collection
	cmd.Response() <- templates
	cmd.Response() <- err
}

// AddFromCollectionCmd adds all exercises from a collection to a specific date
type AddFromCollectionCmd struct {
	core.ActorCommand
	Account      string
	Date         string
	CollectionID string
}

func (cmd *AddFromCollectionCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	err := exerciseActor.addFromCollection(cmd.Account, cmd.Date, cmd.CollectionID)
	cmd.Response() <- err
}

// CalculateCaloriesCmd calculates calories burned
type CalculateCaloriesCmd struct {
	core.ActorCommand
	Account      string
	ExerciseType string
	Intensity    string
	Duration     int
	Weight       float64
}

func (cmd *CalculateCaloriesCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	
	// If weight is not provided, try to get it from account info
	weight := cmd.Weight
	if weight <= 0 && cmd.Account != "" {
		if accountInfo, err := account.GetAccountInfo(cmd.Account); err == nil && accountInfo != nil {
			weight = accountInfo.Weight
		}
	}
	
	calories := exerciseActor.calculateCalories(cmd.ExerciseType, cmd.Intensity, cmd.Duration, weight)
	cmd.Response() <- calories
}

// GetMETValueWithDescriptionCmd returns the MET value and description
type GetMETValueWithDescriptionCmd struct {
	core.ActorCommand
	Account      string
	ExerciseType string
	Intensity    string
}

func (cmd *GetMETValueWithDescriptionCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	met, description := exerciseActor.getMETValueWithDescription(cmd.ExerciseType, cmd.Intensity)
	cmd.Response() <- met
	cmd.Response() <- description
}

// SaveUserProfileCmd saves or updates user profile
type SaveUserProfileCmd struct {
	core.ActorCommand
	Account string
	Name    string
	Gender  string
	Weight  float64
	Height  float64
	Age     int
}

func (cmd *SaveUserProfileCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	profile, err := exerciseActor.saveUserProfile(cmd.Account, cmd.Name, cmd.Gender, cmd.Weight, cmd.Height, cmd.Age)
	if err != nil {
		cmd.Response() <- nil
		cmd.Response() <- err
	} else {
		cmd.Response() <- profile
		cmd.Response() <- nil
	}
}

// GetUserProfileCmd retrieves user profile
type GetUserProfileCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetUserProfileCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	profile, err := exerciseActor.getUserProfile(cmd.Account)
	cmd.Response() <- profile
	cmd.Response() <- err
}

// GetMETValuesCmd retrieves MET values
type GetMETValuesCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetMETValuesCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	metValues, err := exerciseActor.getMETValues(cmd.Account)
	cmd.Response() <- metValues
	cmd.Response() <- err
}

// UpdateAllTemplateCaloriesCmd updates calories for all existing templates
type UpdateAllTemplateCaloriesCmd struct {
	core.ActorCommand
	Account string
	Weight  float64
}

func (cmd *UpdateAllTemplateCaloriesCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	
	// If weight is not provided, try to get it from account info
	weight := cmd.Weight
	if weight <= 0 && cmd.Account != "" {
		if accountInfo, err := account.GetAccountInfo(cmd.Account); err == nil && accountInfo != nil {
			weight = accountInfo.Weight
		}
	}
	
	err := exerciseActor.updateAllTemplateCalories(cmd.Account, weight)
	cmd.Response() <- err
}

// UpdateAllExerciseCaloriesCmd updates calories for all existing exercise records
type UpdateAllExerciseCaloriesCmd struct {
	core.ActorCommand
	Account string
	Weight  float64
}

func (cmd *UpdateAllExerciseCaloriesCmd) Do(actor core.ActorInterface) {
	exerciseActor := actor.(*ExerciseActor)
	
	// If weight is not provided, try to get it from account info
	weight := cmd.Weight
	if weight <= 0 && cmd.Account != "" {
		if accountInfo, err := account.GetAccountInfo(cmd.Account); err == nil && accountInfo != nil {
			weight = accountInfo.Weight
		}
	}
	
	updatedCount, err := exerciseActor.updateAllExerciseCalories(cmd.Account, weight)
	cmd.Response() <- updatedCount
	cmd.Response() <- err
}
