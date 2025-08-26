package reading

import (
	"core"
	"module"
	log "mylog"
	"sync"

	"blog"
)

// ReadingManager manages multiple reading actors for different accounts
type ReadingManager struct {
	actors     map[string]*ReadingActor // account -> ReadingActor
	defaultAct *ReadingActor            // default actor for system operations
	mu         sync.RWMutex
}

var readingManager *ReadingManager

// InitManager initializes the reading manager
type InitManagerCmd struct {
	core.ActorCommand
}

func (cmd *InitManagerCmd) Do(actor core.ActorInterface) {
	readingManager = &ReadingManager{
		actors:     make(map[string]*ReadingActor),
		defaultAct: nil,
	}

	// Initialize default actor for system operations
	readingManager.defaultAct = &ReadingActor{
		Actor:               core.NewActor(),
		Account:             blog.GetDefaultAccount(),
		books:               make(map[string]*module.Book),
		readingRecords:      make(map[string]*module.ReadingRecord),
		bookNotes:           make(map[string][]*module.BookNote),
		bookInsights:        make(map[string]*module.BookInsight),
		readingPlans:        make(map[string]*module.ReadingPlan),
		readingGoals:        make(map[string]*module.ReadingGoal),
		bookRecommendations: make(map[string]*module.BookRecommendation),
		bookCollections:     make(map[string]*module.BookCollection),
		readingTimeRecords:  make(map[string][]*module.ReadingTimeRecord),
	}
	readingManager.defaultAct.Start(readingManager.defaultAct)

	// Load system reading data
	loadCmd := &loadReadingDataCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}
	readingManager.defaultAct.Send(loadCmd)
	<-loadCmd.Response()

	cmd.Response() <- 0
}

// GetReadingActor returns the reading actor for a specific account
// If account is empty, returns the default actor
type GetReadingActorCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetReadingActorCmd) Do(actor core.ActorInterface) {
	readingManager.mu.RLock()
	defer readingManager.mu.RUnlock()

	if cmd.Account == "" {
		cmd.Response() <- readingManager.defaultAct
		return
	}

	if act, exists := readingManager.actors[cmd.Account]; exists {
		cmd.Response() <- act
		return
	}

	// Create new actor for this account
	readingManager.mu.RUnlock()
	readingManager.mu.Lock()

	newActor := &ReadingActor{
		Actor:               core.NewActor(),
		Account:             cmd.Account,
		books:               make(map[string]*module.Book),
		readingRecords:      make(map[string]*module.ReadingRecord),
		bookNotes:           make(map[string][]*module.BookNote),
		bookInsights:        make(map[string]*module.BookInsight),
		readingPlans:        make(map[string]*module.ReadingPlan),
		readingGoals:        make(map[string]*module.ReadingGoal),
		bookRecommendations: make(map[string]*module.BookRecommendation),
		bookCollections:     make(map[string]*module.BookCollection),
		readingTimeRecords:  make(map[string][]*module.ReadingTimeRecord),
	}
	newActor.Start(newActor)

	// Load account-specific reading data
	loadCmd := &loadAccountReadingDataCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Account:      cmd.Account,
	}
	newActor.Send(loadCmd)
	<-loadCmd.Response()

	readingManager.actors[cmd.Account] = newActor
	readingManager.mu.Unlock()
	readingManager.mu.RLock()

	cmd.Response() <- newActor
}

// loadAccountReadingDataCmd loads reading data for a specific account
type loadAccountReadingDataCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *loadAccountReadingDataCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	
	// Load account-specific reading data from blog system
	readingActor.loadBooksForAccount(cmd.Account)
	readingActor.loadReadingRecordsForAccount(cmd.Account)
	readingActor.loadBookNotesForAccount(cmd.Account)
	readingActor.loadBookInsightsForAccount(cmd.Account)
	readingActor.loadReadingPlansForAccount(cmd.Account)
	readingActor.loadReadingGoalsForAccount(cmd.Account)
	readingActor.loadBookCollectionsForAccount(cmd.Account)
	readingActor.loadReadingTimeRecordsForAccount(cmd.Account)

	log.DebugF("Loaded reading data for account %s - Books: %d, Records: %d, Notes: %d, Insights: %d",
		cmd.Account, len(readingActor.books), len(readingActor.readingRecords), 
		readingActor.getTotalNotesCount(), len(readingActor.bookInsights))
	
	cmd.Response() <- 0
}

// loadReadingDataCmd loads system reading data
type loadReadingDataCmd struct {
	core.ActorCommand
}

func (cmd *loadReadingDataCmd) Do(actor core.ActorInterface) {
	readingActor := actor.(*ReadingActor)
	
	// Load default reading data
	readingActor.loadBooks()
	readingActor.loadReadingRecords()
	readingActor.loadBookNotes()
	readingActor.loadBookInsights()
	readingActor.loadReadingPlans()
	readingActor.loadReadingGoals()
	readingActor.loadBookCollections()
	readingActor.loadReadingTimeRecords()

	cmd.Response() <- 0
}

// RemoveAccount removes an account's reading actor
type RemoveAccountCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *RemoveAccountCmd) Do(actor core.ActorInterface) {
	readingManager.mu.Lock()
	defer readingManager.mu.Unlock()

	if act, exists := readingManager.actors[cmd.Account]; exists {
		act.Stop()
		delete(readingManager.actors, cmd.Account)
	}
	cmd.Response() <- 0
}

// getReadingActor returns the reading actor for the given account
func getReadingActor(account string) *ReadingActor {
	// If account is empty, use default account
	if account == "" {
		account = blog.GetDefaultAccount()
	}

	if readingManager == nil {
		// Initialize manager if not already done
		readingManager = &ReadingManager{
			actors: make(map[string]*ReadingActor),
			defaultAct: &ReadingActor{
				Actor:               core.NewActor(),
				Account:             blog.GetDefaultAccount(),
				books:               make(map[string]*module.Book),
				readingRecords:      make(map[string]*module.ReadingRecord),
				bookNotes:           make(map[string][]*module.BookNote),
				bookInsights:        make(map[string]*module.BookInsight),
				readingPlans:        make(map[string]*module.ReadingPlan),
				readingGoals:        make(map[string]*module.ReadingGoal),
				bookRecommendations: make(map[string]*module.BookRecommendation),
				bookCollections:     make(map[string]*module.BookCollection),
				readingTimeRecords:  make(map[string][]*module.ReadingTimeRecord),
			},
		}
		readingManager.defaultAct.Start(readingManager.defaultAct)

		// Load system reading data
		loadCmd := &loadReadingDataCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}
		readingManager.defaultAct.Send(loadCmd)
		<-loadCmd.Response()
	}

	if account == blog.GetDefaultAccount() {
		return readingManager.defaultAct
	}

	readingManager.mu.RLock()
	if act, exists := readingManager.actors[account]; exists {
		readingManager.mu.RUnlock()
		return act
	}
	readingManager.mu.RUnlock()

	// Create new actor for this account
	readingManager.mu.Lock()
	defer readingManager.mu.Unlock()

	newActor := &ReadingActor{
		Actor:               core.NewActor(),
		Account:             account,
		books:               make(map[string]*module.Book),
		readingRecords:      make(map[string]*module.ReadingRecord),
		bookNotes:           make(map[string][]*module.BookNote),
		bookInsights:        make(map[string]*module.BookInsight),
		readingPlans:        make(map[string]*module.ReadingPlan),
		readingGoals:        make(map[string]*module.ReadingGoal),
		bookRecommendations: make(map[string]*module.BookRecommendation),
		bookCollections:     make(map[string]*module.BookCollection),
		readingTimeRecords:  make(map[string][]*module.ReadingTimeRecord),
	}
	newActor.Start(newActor)

	// Load account-specific reading data
	loadCmd := &loadAccountReadingDataCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Account:      account,
	}
	newActor.Send(loadCmd)
	<-loadCmd.Response()

	readingManager.actors[account] = newActor
	return newActor
}