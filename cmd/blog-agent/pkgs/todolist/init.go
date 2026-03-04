package todolist

import (
	log "mylog"
)

// InitTodoList initializes the todo list functionality
func InitTodoList() error {
	// Create todo manager
	manager := NewTodoManager()

	// Create controller
	controller := NewController(manager)

	// Set controller for handlers
	SetController(controller)

	log.InfoF(log.ModuleTodolist, "Todolist initialized successfully using blog storage system")
	return nil
}
