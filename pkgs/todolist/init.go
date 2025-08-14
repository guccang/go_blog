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

	// We don't need hooks since we're directly using blog's functions
	// instead of: persistence.AddHook(func() {
	//    persistence.SaveBlogs(blog.GetBlogs())
	// })

	fmt.Println("Todolist initialized successfully using blog storage system")
	return nil
}
