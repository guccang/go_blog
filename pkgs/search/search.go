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
			private := 1
			if tag == "public" {
				private = 0
			}
			return matchBlogsWithPublicPrivate(private,tokens[1:])
		} 
		if strings.ToLower(tag) == strings.ToLower("encrypt") {
			return matchEncrypt()
		}
		if strings.ToLower(tag) == strings.ToLower("reload") {
			if len(tokens) != 2 {
				return nil
			}
			reload(tokens[1])
		}
		if strings.ToLower(tag) == strings.ToLower("tag") {
			if len(tokens) < 2 {
				return nil
			}
			tagChange(tokens)
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
		log.MessageF("reload cfg %s",config_path)
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

	// 匹配title and content
	for _,match := range matches {
		// title match
		if strings.Contains(strings.ToLower(b.Title),strings.ToLower(match)) {
			return 1;
		}	
		// content match
		if strings.Contains(strings.ToLower(b.Content),strings.ToLower(match)){
			return 1;
		}
	}
	return 0
}

func matchBlogsWithPublicPrivate(private int,matches []string) []*module.Blog{
	auth_type := module.EAuthType_public

	if private == 1 {
		auth_type = module.EAuthType_private
	}

	s := make([]*module.Blog,0)
	for _,b := range blog.Blogs {
		// auth
		if  b.AuthType != auth_type {
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
