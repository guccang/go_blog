package todolist

import (
	"fmt"
)

// InitTodoList initializes the todo list functionality
func InitTodoList() error {
	// Create todo manager
	manager := NewTodoManager()

	// Create controller
	controller := NewController(manager)

	// Set controller for handlers
	SetController(controller)

	fmt.Println("Todolist initialized successfully using blog storage system")
	return nil
}
