package ioutils
import (
	log "mylog"
	"io/ioutil"
	"config"
	"path/filepath"
	"os"
)

func Info(){
	log.Debug("info ioutils v1.0")
	dir := config.GetBlogsPath()
	GetFiles(dir)
}

func GetFiles(dir string)[]string{
	files := make([]string,0)

	entries, err := ioutil.ReadDir(dir)
    if err != nil {
		log.DebugF("GetFiles error=%s",err.Error())
        return nil
    }

    for _, entry := range entries {
        if !entry.IsDir() {
			full:=filepath.Join(dir,entry.Name())
            //log.DebugF("GetFiles %s",full)
			files = append(files,full)
        }
    }
	
	return files
}

func GetBaseAndExt(full string)(string,string){
	filenameWithExt :=  filepath.Base(full)
	ext := filepath.Ext(filenameWithExt)
	nameWithoutExt := filenameWithExt[:len(filenameWithExt)-len(ext)]
	return nameWithoutExt,ext
}

func GetFileDatas(full string)(string,int){
	data, err := ioutil.ReadFile(full)
	if err != nil {
		log.ErrorF("Error reading file:%s %s",full, err.Error())
		return "",0
	}
	return string(data),len(data)
}

func RmAndSaveFile(full string,content string){
	// 判断文件是否存在
	if _, err := os.Stat(full); os.IsNotExist(err) {
		// todo 
	} else {
			// 删除已存在的文件
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			log.DebugF("Error removing existing file:%s %s",full,err.Error())
			return
		}
		log.DebugF("File already exists and remove success %s",full)
	}

	// 文件不存在，写入内容
	err := ioutil.WriteFile(full, []byte(content), 0644)
	if err != nil {
		log.ErrorF("Error writing to file: %s %s", err.Error(),full)
	} else {
		log.DebugF("File created and content written successfully. %s",full)
	}
}
