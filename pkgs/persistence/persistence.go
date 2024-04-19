package persistence
import (
	"fmt"
	"module"
)

func Info(){
	fmt.Println("info persistence v1.0")
}
	

type persistence interface{
	SaveBlog(b module.Blog)
	SaveBlogs(blogs map[string]*module.Blog)
	GetBlog(name string)*module.Blog 
	GetBlogs()map[string]*module.Blog
}
