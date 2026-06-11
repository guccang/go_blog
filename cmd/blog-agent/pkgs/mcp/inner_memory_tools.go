package mcp

import (
	"memory"
)

// ============================================================================
// Memory 模块工具函数 - 私人管家共享记忆库（MEMORY/checkpoint/goals/journal）
// 设计参考 MiMo Code：Markdown 可审阅记忆 + 关键词检索
// ============================================================================

func Inner_blog_RawMemoryRead(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	file, err := getStringParam(arguments, "file")
	if err != nil {
		return errorJSON(err.Error())
	}
	content, err := memory.ReadFile(account, file)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(map[string]string{"file": file, "content": content}))
}

func Inner_blog_RawMemoryWrite(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	file, err := getStringParam(arguments, "file")
	if err != nil {
		return errorJSON(err.Error())
	}
	content, err := getStringParam(arguments, "content")
	if err != nil {
		return errorJSON(err.Error())
	}
	if err := memory.WriteFile(account, file, content); err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(`{"success":true}`)
}

func Inner_blog_RawMemoryAppend(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	file, err := getStringParam(arguments, "file")
	if err != nil {
		return errorJSON(err.Error())
	}
	content, err := getStringParam(arguments, "content")
	if err != nil {
		return errorJSON(err.Error())
	}
	if err := memory.Append(account, file, content); err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(`{"success":true}`)
}

func Inner_blog_RawMemoryJournal(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	content, err := getStringParam(arguments, "content")
	if err != nil {
		return errorJSON(err.Error())
	}
	file, err := memory.AppendJournal(account, content)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(map[string]interface{}{"success": true, "file": file}))
}

func Inner_blog_RawMemorySearch(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	query, err := getStringParam(arguments, "query")
	if err != nil {
		return errorJSON(err.Error())
	}
	limit := getOptionalIntParam(arguments, "limit", 5)
	days := getOptionalIntParam(arguments, "recent_journal_days", 14)
	return wrapResult(memory.SearchJSON(account, query, limit, days))
}

func Inner_blog_RawMemoryRewriteSection(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	file, err := getStringParam(arguments, "file")
	if err != nil {
		return errorJSON(err.Error())
	}
	section, err := getStringParam(arguments, "section")
	if err != nil {
		return errorJSON(err.Error())
	}
	content, err := getStringParam(arguments, "content")
	if err != nil {
		return errorJSON(err.Error())
	}
	if err := memory.RewriteSection(account, file, section, content); err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(`{"success":true}`)
}

func Inner_blog_RawMemoryListFiles(arguments map[string]interface{}) string {
	requestedAccount, err := getStringParam(arguments, "account")
	if err != nil {
		return errorJSON(err.Error())
	}
	account, err := ValidateAccountParam(requestedAccount)
	if err != nil {
		return errorJSON(err.Error())
	}
	return wrapResult(toJSONString(memory.ListFiles(account)))
}
