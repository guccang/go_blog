package blog

import (
	"core"
	"module"
	log "mylog"
	"sync"

	"persistence"
)

// BlogManager manages multiple blog actors for different accounts
type BlogManager struct {
	actors     map[string]*BlogActor // account -> BlogActor
	defaultAct *BlogActor            // default actor for system operations
	mu         sync.RWMutex
}

var blogManager *BlogManager

// InitManager initializes the blog manager
type InitManagerCmd struct {
	core.ActorCommand
}

func (cmd *InitManagerCmd) Do(actor core.ActorInterface) {
	blogManager = &BlogManager{
		actors:     make(map[string]*BlogActor),
		defaultAct: nil,
	}

	// Initialize default actor for system operations
	blogManager.defaultAct = &BlogActor{
		Actor:   core.NewActor(),
		Account: getDefaultAccount(),
		blogs:   make(map[string]*module.Blog),
	}
	blogManager.defaultAct.Start(blogManager.defaultAct)

	// Load system blogs
	loadCmd := &loadBlogsCmd{ActorCommand: core.ActorCommand{Res: make(chan interface{})}}
	blogManager.defaultAct.Send(loadCmd)
	<-loadCmd.Response()

	cmd.Response() <- 0
}

// GetBlogActor returns the blog actor for a specific account
// If account is empty, returns the default actor
type GetBlogActorCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *GetBlogActorCmd) Do(actor core.ActorInterface) {
	blogManager.mu.RLock()
	defer blogManager.mu.RUnlock()

	if cmd.Account == "" {
		cmd.Response() <- blogManager.defaultAct
		return
	}

	if act, exists := blogManager.actors[cmd.Account]; exists {
		cmd.Response() <- act
		return
	}

	// Create new actor for this account
	blogManager.mu.RUnlock()
	blogManager.mu.Lock()

	newActor := &BlogActor{
		Actor:   core.NewActor(),
		Account: cmd.Account,
		blogs:   make(map[string]*module.Blog),
	}
	newActor.Start(newActor)

	// Load account-specific blogs
	loadCmd := &loadAccountBlogsCmd{
		ActorCommand: core.ActorCommand{Res: make(chan interface{})},
		Account:      cmd.Account,
	}
	newActor.Send(loadCmd)
	<-loadCmd.Response()

	blogManager.actors[cmd.Account] = newActor
	blogManager.mu.Unlock()
	blogManager.mu.RLock()

	cmd.Response() <- newActor
}

// loadAccountBlogsCmd loads blogs for a specific account
type loadAccountBlogsCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *loadAccountBlogsCmd) Do(actor core.ActorInterface) {
	blogActor := actor.(*BlogActor)
	blogs := persistence.GetBlogsByAccount(cmd.Account)
	if blogs != nil {
		for _, b := range blogs {
			if b.Encrypt == 1 {
				b.AuthType = module.EAuthType_encrypt
			}
			blogActor.blogs[b.Title] = b
		}
	}
	log.DebugF("getblogs for account %s number=%d", cmd.Account, len(blogs))
	cmd.Response() <- 0
}

// RemoveAccount removes an account's blog actor
type RemoveAccountCmd struct {
	core.ActorCommand
	Account string
}

func (cmd *RemoveAccountCmd) Do(actor core.ActorInterface) {
	blogManager.mu.Lock()
	defer blogManager.mu.Unlock()

	if act, exists := blogManager.actors[cmd.Account]; exists {
		act.Stop()
		delete(blogManager.actors, cmd.Account)
	}
	cmd.Response() <- 0
}
