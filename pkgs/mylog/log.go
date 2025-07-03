package mylog
import (
	"fmt"
	"time"
)

func Info(){
	fmt.Println("info log v1.0")
}

func strTime() string{
	return  time.Now().Format("2006-01-02 15:04:05")
}

func Debug(str string){
	fmt.Printf("[DEBUG] %s %s\n",strTime(),str)
}

func DebugF(f string,a ...any){
	str := fmt.Sprintf(f,a...)
	fmt.Printf("[DEBUG] %s %s\n",strTime(),str)
}

func InfoF(f string,a ...any){
	str := fmt.Sprintf(f,a...)
	fmt.Printf("[MESSAGE] %s %s\n",strTime(),str)
}

func ErrorF(f string, a ...any){
	str := fmt.Sprintf(f,a...)
	fmt.Printf("[ERROR] %s %s\n",strTime(),str)
}
