package search

import (
	"blog"
	"config"
	"fmt"
	"module"
	log "mylog"
	"sort"
	"strings"
	"time"
)

type optFunc func(string, []string) []*module.Blog

var optFuncMap = make(map[string]optFunc)

// SearchCommandMeta 搜索命令元数据
type SearchCommandMeta struct {
	Name        string // 命令名称，如 "@tag match"
	DisplayName string // 显示名称，如 "标签搜索"
	Description string // 命令描述
	Example     string // 使用示例
	HasParam    bool   // 是否需要额外参数
	ParamHint   string // 参数提示，如 "标签名"
}

// commandMetaMap 命令元数据映射
var commandMetaMap = make(map[string]SearchCommandMeta)

// registerCommand 注册搜索命令
func registerCommand(name string, fn optFunc, meta SearchCommandMeta) {
	optFuncMap[name] = fn
	commandMetaMap[name] = meta
}

// GetSearchCommands 获取所有搜索命令的元数据
func GetSearchCommands() []SearchCommandMeta {
	commands := make([]SearchCommandMeta, 0, len(commandMetaMap))
	for _, meta := range commandMetaMap {
		commands = append(commands, meta)
	}
	// 按显示名称排序，确保顺序一致
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].DisplayName < commands[j].DisplayName
	})
	return commands
}

func Info() {
	log.InfoF(log.ModuleSearch, "info search v5.0")

	// 注册搜索命令及其元数据
	registerCommand("@normal search", normalMatch, SearchCommandMeta{
		Name:        "@normal search",
		DisplayName: "普通搜索",
		Description: "搜索博客标题和内容",
		Example:     "关键词1 关键词2",
		HasParam:    true,
		ParamHint:   "搜索关键词",
	})

	// @matchauth public
	// @matchauth private
	// @matchauth encrypt
	registerCommand("@auth match", authMatch, SearchCommandMeta{
		Name:        "@auth match",
		DisplayName: "权限搜索",
		Description: "按权限类型搜索博客",
		Example:     "@auth match public",
		HasParam:    true,
		ParamHint:   "public/private/encrypt",
	})
	// @optauth public matchtitle
	// @optauth private matchtitle
	registerCommand("@auth optchange", authChange, SearchCommandMeta{
		Name:        "@auth optchange",
		DisplayName: "修改权限",
		Description: "修改博客的权限类型",
		Example:     "@auth optchange public 标题关键词",
		HasParam:    true,
		ParamHint:   "public/private 标题关键词",
	})
	// @matchtag "tag1 tag2 tag3"
	registerCommand("@tag match", tagMatch, SearchCommandMeta{
		Name:        "@tag match",
		DisplayName: "标签搜索",
		Description: "按标签搜索博客",
		Example:     "@tag match linux",
		HasParam:    true,
		ParamHint:   "标签名",
	})
	// @opttag add tag matchtitle
	registerCommand("@tag optadd", tagAdd, SearchCommandMeta{
		Name:        "@tag optadd",
		DisplayName: "添加标签",
		Description: "给博客添加标签",
		Example:     "@tag optadd linux 标题关键词",
		HasParam:    true,
		ParamHint:   "标签名 标题关键词",
	})
	// @opttag replace from-tag to-tag
	registerCommand("@tag optreplace", tagChange, SearchCommandMeta{
		Name:        "@tag optreplace",
		DisplayName: "替换标签",
		Description: "替换博客的标签",
		Example:     "@tag optreplace 旧标签 新标签",
		HasParam:    true,
		ParamHint:   "旧标签 新标签",
	})
	// @opttag clear tag
	registerCommand("@tag optclear", tagClear, SearchCommandMeta{
		Name:        "@tag optclear",
		DisplayName: "清除标签",
		Description: "清除博客的标签",
		Example:     "@tag optclear 标签名",
		HasParam:    true,
		ParamHint:   "标签名",
	})
	// @matchtimed machtitle 添加日期的标题的博客搜索
	registerCommand("@timed match", timedMatch, SearchCommandMeta{
		Name:        "@timed match",
		DisplayName: "定时博客",
		Description: "搜索包含日期的博客",
		Example:     "@timed match 关键词",
		HasParam:    true,
		ParamHint:   "搜索关键词",
	})
	// @reload cfg
	registerCommand("@reload cfg", reloadCfg, SearchCommandMeta{
		Name:        "@reload cfg",
		DisplayName: "重载配置",
		Description: "重新加载系统配置",
		Example:     "@reload cfg",
		HasParam:    false,
		ParamHint:   "",
	})
}

/*
  - @tag params for system
    @encrypt : show encryption blogs
    @public : show public blogs
    @private : show private blogs
  - $tag params for normal tags
    $linux     : search blogs with tag named linux
    $linux vim : search blogs with tag named linux, and find content or title has vim world
  - others for search
*/
func Search(account, match string) []*module.Blog {
	// 空格分割
	tokens := strings.Split(match, " ")

	log.DebugF(log.ModuleSearch, "match=%s tokens =%d", match, len(tokens))

	if len(tokens) <= 0 {
		empty := make([]*module.Blog, 0)
		return empty
	}

	opt := tokens[0]
	if strings.HasPrefix(opt, "@") {
		if len(tokens) > 1 {
			opt = fmt.Sprintf("%s %s", tokens[0], tokens[1])
			if f, ok := optFuncMap[opt]; ok {
				// tokens[1:] pass params
				return f(account, tokens[2:])
			}

		}
	} else {
		opt = "@normal search"
		return optFuncMap[opt](account, tokens)
	}

	return []*module.Blog{}
}

func sortblogs(s []*module.Blog) {
	sort.Slice(s, func(i, j int) bool {
		ti, _ := time.Parse("2006-01-02 15:04:05", s[i].ModifyTime)
		tj, _ := time.Parse("2006-01-02 15:04:05", s[j].ModifyTime)
		return ti.Unix() > tj.Unix()
	})
}

// @tag match tags
func tagMatch(account string, tokens []string) []*module.Blog {
	s := make([]*module.Blog, 0)
	tag := ""
	if len(tokens) > 0 {
		tag = tokens[0]
	}

	matches := tokens[1:]
	for _, b := range blog.GetBlogsWithAccount(account) {
		if false == strings.Contains(strings.ToLower(b.Tags), strings.ToLower(tag)) {
			continue
		}

		if ismatch(b, matches) == 0 {
			continue
		}

		s = append(s, b)
	}

	sortblogs(s)

	return s
}

func reloadCfg(account string, tokens []string) []*module.Blog {
	config_path := config.GetConfigPathWithAccount(account)
	config.ReloadConfig(account, config_path)
	log.InfoF(log.ModuleSearch, "reload cfg %s", config_path)
	return []*module.Blog{}
}

func normalMatch(account string, matches []string) []*module.Blog {
	s := make([]*module.Blog, 0)
	for _, b := range blog.GetBlogsWithAccount(account) {
		if ismatch(b, matches) == 0 {
			continue
		}
		s = append(s, b)
	}

	sortblogs(s)

	return s
}

func isMatchTitle(b *module.Blog, matches []string) int {
	log.DebugF(log.ModuleSearch, "isMatchTitle len(matches)=%d matches=%v", len(matches), matches)

	// 加密不显示
	if b.Encrypt == 1 {
		return 0
	}

	// 没有matches
	if len(matches) == 0 {
		return 0
	}

	// 匹配title
	for _, match := range matches {
		// title match
		if strings.Contains(strings.ToLower(b.Title), strings.ToLower(match)) {
			return 1
		}
	}
	return 0
}

func ismatch(b *module.Blog, matches []string) int {
	log.DebugF(log.ModuleSearch, "ismatch len(matches)=%d matches=%v", len(matches), matches)

	// 加密不显示
	if b.Encrypt == 1 {
		return 0
	}

	// 没有matches,所有的都可以
	if len(matches) == 0 {
		return 1
	}

	tType := "-tTitle"

	onlyMatchTitle := 0
	if len(matches) >= 2 {
		if strings.ToLower(matches[0]) == strings.ToLower(tType) {
			// 只匹配标题
			onlyMatchTitle = 1
		}
	}

	// 匹配title and content
	for _, match := range matches {
		if strings.ToLower(match) == strings.ToLower(tType) {
			continue
		}
		// title match
		if strings.Contains(strings.ToLower(b.Title), strings.ToLower(match)) {
			return 1
		}
		if onlyMatchTitle == 1 {
			continue
		}

		// content match
		if strings.Contains(strings.ToLower(b.Content), strings.ToLower(match)) {
			return 1
		}
	}
	return 0
}

func authChange(account string, tokens []string) []*module.Blog {
	s := make([]*module.Blog, 0)
	if len(tokens) <= 0 {
		return s
	}

	tag := tokens[0]
	auth_type := module.EAuthType_private
	if strings.ToLower(tag) == strings.ToLower("private") {
		auth_type = module.EAuthType_private
	} else if strings.ToLower(tag) == strings.ToLower("public") {
		auth_type = module.EAuthType_public
	} else {
		return s
	}

	matches := tokens[1:]
	for _, b := range blog.GetBlogsWithAccount(account) {

		if isMatchTitle(b, matches) == 0 {
			continue
		}

		b.AuthType = auth_type

		s = append(s, b)

		blog.AddAuthTypeWithAccount(account, b.Title, auth_type)
	}

	sortblogs(s)

	return s

}

func authMatch(account string, tokens []string) []*module.Blog {
	s := make([]*module.Blog, 0)
	if len(tokens) <= 0 {
		return s
	}

	t := tokens[0]
	auth_type := module.EAuthType_private
	if strings.ToLower(t) == "public" {
		auth_type = module.EAuthType_public
	} else if strings.ToLower(t) == "private" {
		auth_type = module.EAuthType_private
	} else if strings.ToLower(t) == "encrypt" {
		return encryptMatch(account, tokens)
	} else {
		return s
	}

	matches := tokens[1:]
	for _, b := range blog.GetBlogsWithAccount(account) {
		// auth
		if (b.AuthType & auth_type) == 0 {
			continue
		}

		if ismatch(b, matches) == 0 {
			continue
		}

		s = append(s, b)

	}

	sortblogs(s)

	return s

}

func encryptMatch(account string, tokens []string) []*module.Blog {
	s := make([]*module.Blog, 0)
	for _, b := range blog.GetBlogsWithAccount(account) {

		// not encrypt
		if b.Encrypt != 1 {
			continue
		}
		s = append(s, b)
	}

	sortblogs(s)

	return s
}

func tagAdd(account string, tokens []string) []*module.Blog {
	if len(tokens) != 2 {
		return []*module.Blog{}
	}
	tag := tokens[0]
	title := tokens[1]

	return blog.TagAddWithAccount(account, title, tag)
}

func tagClear(account string, tokens []string) []*module.Blog {
	from := ""
	to := ""

	if len(tokens) == 1 {
		from = tokens[0]
	}

	return blog.TagReplaceWithAccount(account, from, to)
}

func tagChange(account string, tokens []string) []*module.Blog {
	from := ""
	to := ""

	if len(tokens) == 2 {
		from = tokens[0]
		to = tokens[1]
	}

	return blog.TagReplaceWithAccount(account, from, to)
}

func timedMatch(account string, tokens []string) []*module.Blog {
	s := make([]*module.Blog, 0)
	for _, b := range blog.GetBlogsWithAccount(account) {
		// not timed
		if config.IsTitleContainsDateSuffix(b.Title) != 1 {
			continue
		}
		if ismatch(b, tokens) == 0 {
			continue
		}
		s = append(s, b)
	}
	sortblogs(s)
	return s
}
