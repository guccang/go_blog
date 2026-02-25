package ioutils

import (
	"io/ioutil"
	log "mylog"
	"os"
	"path/filepath"
)

func Info() {
	log.Debug(log.ModuleCommon, "info ioutils v1.0")
}

func GetFiles(dir string) []string {
	files := make([]string, 0)

	entries, err := ioutil.ReadDir(dir)
	if err != nil {
		log.DebugF(log.ModuleCommon, "GetFiles error=%s", err.Error())
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			full := filepath.Join(dir, entry.Name())
			//log.DebugF("GetFiles %s",full)
			files = append(files, full)
		}
	}

	return files
}

// GetFilesRecursive 递归获取目录下所有文件（包括子目录）
func GetFilesRecursive(dir string) []string {
	files := make([]string, 0)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files
}

func GetBaseAndExt(full string) (string, string) {
	filenameWithExt := filepath.Base(full)
	ext := filepath.Ext(filenameWithExt)
	nameWithoutExt := filenameWithExt[:len(filenameWithExt)-len(ext)]
	return nameWithoutExt, ext
}

func GetFileDatas(full string) (string, int) {
	data, err := ioutil.ReadFile(full)
	if err != nil {
		log.ErrorF(log.ModuleCommon, "Error reading file:%s %s", full, err.Error())
		return "", 0
	}
	return string(data), len(data)
}

func DeleteFile(full string) {
	// 判断文件是否存在
	if _, err := os.Stat(full); os.IsNotExist(err) {
		// todo
	} else {
		// 删除已存在的文件
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			log.DebugF(log.ModuleCommon, "Error removing existing file:%s %s", full, err.Error())
			return
		}
		log.DebugF(log.ModuleCommon, "File already exists and remove success %s", full)
	}
}

func Mkdir(path string) int {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			log.ErrorF(log.ModuleCommon, "Mkdir path=%s error=%s", path, err.Error())
			return 1
		}
	}
	return 0
}

func Mvfile(full string, to string) int {
	err := os.Rename(full, to)
	if err != nil {
		log.ErrorF(log.ModuleCommon, "movefile form=%s to=%s error=%s", full, to, err.Error())
		return 1
	}
	return 0
}

func RmAndSaveFile(full string, content string) {
	// 判断文件是否存在
	if _, err := os.Stat(full); os.IsNotExist(err) {
		// todo
	} else {
		// 删除已存在的文件
		if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
			log.DebugF(log.ModuleCommon, "Error removing existing file:%s %s", full, err.Error())
			return
		}
		log.DebugF(log.ModuleCommon, "File already exists and remove success %s", full)
	}

	// 文件不存在，写入内容
	err := ioutil.WriteFile(full, []byte(content), 0644)
	if err != nil {
		log.ErrorF(log.ModuleCommon, "Error writing to file: %s %s", err.Error(), full)
	} else {
		log.DebugF(log.ModuleCommon, "File created and content written successfully. %s", full)
	}
}
