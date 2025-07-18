package search
import (
	"fmt"
	"module"
	"strings"
	"sort"
	"time"
	"blog"
	log "mylog"
	"config"
)


func Info(){
	fmt.Println("info search v5.0");
}


/*
* @tag params for system
	@encrypt : show encryption blogs 
    @public : show public blogs
	@private : show private blogs
* $tag params for normal tags
   $linux     : search blogs with tag named linux
   $linux vim : search blogs with tag named linux, and find content or title has vim world
* others for search
*/
func Search(match string) []*module.Blog{
	// 空格分割
	tokens := strings.Split(match," ")

	log.DebugF("match=%s tokens =%d",match,len(tokens))	
	
	if(len(tokens) <= 0){
		empty := make([]*module.Blog,0)
		return empty
	}

	begin_token := tokens[0]

	if strings.HasPrefix(begin_token,"$") {
		// begin  with $
		tag := begin_token[1:]
		return matchTags(tag,tokens[1:])
	}else if strings.HasPrefix(begin_token,"@") {
		// begin with @
		tag := begin_token[1:]
		log.DebugF("tag=%s token=%s",tag,begin_token)
		if strings.ToLower(tag) == strings.ToLower("public") || strings.ToLower(tag) == strings.ToLower("private") {
			auth_type := module.EAuthType_private
			if tag == "public" {
				auth_type = module.EAuthType_public
			}
			return matchBlogsWithAuthType(auth_type,tokens[1:])
		} 
		if strings.ToLower(tag) == strings.ToLower("cooperation"){
			return matchBlogsWithAuthType(module.EAuthType_cooperation,tokens[1:])
		} 
		if strings.ToLower(tag) == strings.ToLower("encrypt") {
			return matchEncrypt()
		}
		if strings.ToLower(tag) == strings.ToLower("reload") {
			if len(tokens) != 2 {
				return nil
			}
			reload(tokens[1])
			// Return a special blog entry to indicate reload completion
			reloadBlog := &module.Blog{
				Title: "系统重新加载完成",
				Content: "配置文件已重新加载完成！",
				ModifyTime: time.Now().Format("2006-01-02 15:04:05"),
				Tags: "system",
				AuthType: module.EAuthType_public,
			}
			return []*module.Blog{reloadBlog}
		}
		if strings.ToLower(tag) == strings.ToLower("tag") {
			if len(tokens) < 2 {
				return nil
			}
			tagChange(tokens)
		}
		if strings.ToLower(tag) == strings.ToLower("timed") {
			return tagTimed(tokens)
		}
	}else{
			// begin with other
			return matchOther(tokens)	
	}

	return nil
}

func sortblogs(s []*module.Blog) {
	sort.Slice(s,func(i,j int) bool {
		ti,_ := time.Parse("2006-01-02 15:04:05",s[i].ModifyTime)
		tj,_ := time.Parse("2006-01-02 15:04:05",s[j].ModifyTime)
		return ti.Unix() > tj.Unix()
	})
}

func matchTags(tag string, matches []string) []*module.Blog{
	s := make([]*module.Blog,0)
	for _,b := range blog.Blogs {
		if false == strings.Contains(strings.ToLower(b.Tags),strings.ToLower(tag)) {
			continue
		}

		if ismatch(b,matches) == 0 {
			continue
		}

		s = append(s,b);	
	}

	sortblogs(s)

	return s
}

func reload(name string) {
	if name == "cfg" {
		config_path := config.GetConfigPath()
		config.ReloadConfig(config_path);
		log.InfoF("reload cfg %s",config_path)
	}
}

func matchOther(matches []string) []*module.Blog {
	s := make([]*module.Blog,0)
	for _,b := range blog.Blogs {
		if ismatch(b,matches) == 0 {
			continue
		}
		s = append(s,b);	
	}

	sortblogs(s)

	return s
}

func matchHelp() []*module.Blog {
	s := make([]*module.Blog,0)
	for _,b := range blog.Blogs {
		s = append(s,b);	
	}
	return s
}

func ismatch(b *module.Blog,matches []string) int{
	log.DebugF("ismatch len(matches)=%d matches=%v",len(matches),matches)

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
	if len(matches) >= 2  {
		if strings.ToLower(matches[0]) == strings.ToLower(tType) {
			// 只匹配标题
			onlyMatchTitle = 1
		}
	}

	// 匹配title and content
	for _,match := range matches {
		if strings.ToLower(match) == strings.ToLower(tType) {
			continue
		}
		// title match
		if strings.Contains(strings.ToLower(b.Title),strings.ToLower(match)) {
			return 1;
		}	
		if onlyMatchTitle == 1 {
			continue
		}

		// content match
		if strings.Contains(strings.ToLower(b.Content),strings.ToLower(match)){
			return 1;
		}
	}
	return 0
}

func matchBlogsWithAuthType(auth_type int,matches []string) []*module.Blog{
	s := make([]*module.Blog,0)
	for _,b := range blog.Blogs {
		// auth
		if  (b.AuthType & auth_type) == 0 {
			continue
		}

		if ismatch(b,matches) == 0 {
			continue;
		}

		s = append(s,b)

	}

	sortblogs(s)

	return s

}

func matchEncrypt() []*module.Blog{
	s := make([]*module.Blog,0)
	for _,b := range blog.Blogs {

		// not encrypt
		if b.Encrypt != 1 {
			continue
		}
		s = append(s,b)
	}

	sortblogs(s)

	return s
}

func tagChange(tokens []string){
	from := ""
	to   := ""

	if len(tokens) == 3 {
		from = tokens[1]
		to = tokens[2]
	}else if len(tokens) == 2{
		from = tokens[1]
	}

	blog.TagReplace(from,to)
}


func tagTimed(tokens []string) []*module.Blog{
	s := make([]*module.Blog,0)
	for _,b := range blog.Blogs {
		// not timed
		if config.IsTitleContainsDateSuffix(b.Title) != 1 {
			continue
		}
		if ismatch(b,tokens[1:]) == 0 {
			continue
		}
		s = append(s,b)
	}
	sortblogs(s)
	return s
}

