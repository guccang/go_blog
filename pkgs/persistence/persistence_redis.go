package persistence

import(
	"fmt"
	log "mylog"
	"module"
	"strconv"
	"github.com/go-redis/redis"
	"time"
	"config"
	"ioutils"
	"path/filepath"
	"sort"
	"strings"
	"encoding/json"
)

type DbRedis struct{
	client *redis.Client
}

var db =  DbRedis{}

func GetDb() *DbRedis{
	return &db
}

func strTime() string{
	return  time.Now().Format("2006-01-02 15:04:05")
}

func Init(){
	ip := config.GetConfig("redis_ip")
	port,_:= strconv.Atoi(config.GetConfig("redis_port"))
	pwd := config.GetConfig("redis_pwd")
	connect(ip,port,pwd)
}

func connect(ip string, port int,password string) int{
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d",ip,port),
		Password:password,
		DB:0,
	})

	pong,err:=client.Ping().Result()
	if err == nil {
		db.client = client
		log.DebugF("connect redis success ip=%s port=%d password=%s",ip,port,password)
		return 1
	}

	log.DebugF(pong,err)
	return 0
}

func DeleteBlog(title string) int{
	key := fmt.Sprintf("blog@%s",title)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("delete error key=%s err=%s",key,err.Error())
		return  1
	}

	log.DebugF("delete title=%s",key)

	deleteFile(title)

	return 0
}

func SaveBlog(blog *module.Blog){
	key := fmt.Sprintf("blog@%s",blog.Title)
	values := make(map[string]interface{})
	values["title"] = blog.Title
	values["content"] = blog.Content
	values["ct"] = blog.CreateTime
	values["mt"] = blog.ModifyTime
	values["at"] = blog.AccessTime
	values["modifynum"] = blog.ModifyNum
	values["accessnum"] = blog.AccessNum
	values["authtype"] = blog.AuthType
	values["tags"] = blog.Tags
	values["encrypt"] = blog.Encrypt
	err := db.client.HMSet(key,values).Err()
	if err != nil {
		log.ErrorF("saveblog error key=%s err=%s",key,err.Error())
	}
	log.DebugF("redis saveblog success key=%s mt=%s",key,blog.ModifyTime)

	saveToFile(blog)
}

func SaveBlogs(blogs map[string]*module.Blog){
	for _,b := range blogs {
		SaveBlog(b)
	}
}

func toBlog(m map[string]string)*module.Blog{
	now :=  strTime()
	ct,ok := m["ct"]
	if !ok { ct = now }
	mt,ok := m["mt"]
	if !ok { mt = now }
	at,ok := m["at"]
	if !ok { at = now }
	mn_s,ok := m["modifynum"]
	if !ok { mn_s = "0"}
	an_s,ok := m["accessnum"]
	if !ok { an_s = "0"}
	auth_s,ok := m["authtype"]
	if !ok { auth_s = "0" }
	tags,ok := m["tags"]
	if !ok { tags = "" }
	encrypt_s,ok := m["encrypt"]
	if !ok { encrypt_s = "0" }

	mn,_ := strconv.Atoi(mn_s)
	an,_ := strconv.Atoi(an_s)
	auth,_ := strconv.Atoi(auth_s) 	
	encrypt,_ := strconv.Atoi(encrypt_s)
	
	b := module.Blog{
		Title:m["title"],
		Content:m["content"],
		CreateTime:ct,
		ModifyTime:mt,
		AccessTime:at,
		ModifyNum:mn,
		AccessNum:an,
		AuthType:auth,
		Tags : tags,
		Encrypt : encrypt,
	}
	return &b
}

func GetBlog(name string)*module.Blog{
	key := fmt.Sprintf("blog@%s",name)
	m ,err := db.client.HGetAll(key).Result()
	if err !=nil {
		log.ErrorF("getblog error key=%s err=%s",key,err.Error())
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	log.DebugF("getblog success key=%s title=%s",key,m["title"])
	b := toBlog(m)
	return b
}

func GetBlogs()map[string]*module.Blog{
    keys,err := db.client.Keys("blog@*").Result()
	if err !=nil {
		log.ErrorF("getblogs error keys=blog@* err=%s",err.Error())
		return nil
	}

	blogs := make(map[string]*module.Blog)

	for _,key := range keys {
		log.DebugF("getblog key=%s",key)
		m ,err := db.client.HGetAll(key).Result()
		if err!=nil {
				log.ErrorF("getblog error key=%s err=%s",key,err.Error())
				continue
		}
		log.DebugF("getblog success key=%s",key)
		b := toBlog(m)
		blogs[b.Title] = b
		showBlog(b)
	}

	return blogs
}

func showBlog(b *module.Blog){
	log.DebugF("title=%s",b.Title)	
	log.DebugF("ct=%s",b.CreateTime)	
	log.DebugF("mt=%s",b.ModifyTime)	
	log.DebugF("at=%s",b.AccessTime)	
	log.DebugF("mn=%d",b.ModifyNum)	
	log.DebugF("an=%d",b.AccessNum)	
}

func deleteFile(title string) int {
	filename := title
	path := config.GetBlogsPath()
	full := filepath.Join(path,filename)
	full = fmt.Sprintf("%s.md",full)

	recycle_path:= config.GetRecyclePath()
	ioutils.Mkdir(recycle_path)
	new_filename := fmt.Sprintf("%s-%s.md",filename,time.Now().Format("2006-01-02"))
	ioutils.Mvfile(full,filepath.Join(recycle_path,new_filename))
	return 0
}

func saveToFile(blog *module.Blog){
	filename := blog.Title
	content := blog.Content

	path := config.GetBlogsPath()
	full := filepath.Join(path,filename)
	full = fmt.Sprintf("%s.md",full)

	fcontent,_ := ioutils.GetFileDatas(full)
	if(content == fcontent){
		log.DebugF("saveToFile Cancle content is same %s",full);
		return
	}
	ioutils.RmAndSaveFile(full,content)

}

func SaveBlogComments(bc *module.BlogComments){
	log.DebugF("SaveBlogComments title=%s comments_len=%d",bc.Title,len(bc.Comments))	

	key := fmt.Sprintf("comments@%s",bc.Title)
	values := make(map[string]interface{})
	s := "\x01"
	// save new keys
	for _,c := range bc.Comments {
		value := fmt.Sprintf("Idx=%d%sowner=%s%sct=%s%smt=%s%smsg=%s%smail=%s",
				c.Idx,s,
				c.Owner,s,
				c.CreateTime,s,
				c.ModifyTime,s,
				c.Msg,s,
				c.Mail)
		idx_str := fmt.Sprintf("%d",c.Idx)
		values[idx_str] = value
	}
	err := db.client.HMSet(key,values).Err()
	if err != nil {
		log.ErrorF("saveblogcomments error key=%s err=%s",key,err.Error())
	}else{
		log.DebugF("redis saveblogcomments success key=%s title=%s",key,bc.Title)
	}
}

func GetAllBlogComments() map[string]*module.BlogComments {
	// todo 
	keys,err := db.client.Keys("comments@*").Result()
	if err !=nil {
		log.ErrorF("getcomments error keys=comments@* err=%s",err.Error())
		return nil
	}

	bcs := make(map[string]*module.BlogComments)

	for _,key := range keys {
		m ,err := db.client.HGetAll(key).Result()
		if err!=nil {
				log.ErrorF("getComments error key=%s err=%s",key,err.Error())
				continue
		}
		log.DebugF("getComments success key=%s",key)
		title := key[strings.Index(key,"@")+1:]
		toBlogComments(title,m,bcs)
	}

	return bcs
}

// 新增阅读功能的数据存储

// 阅读计划存储
func SaveReadingPlan(plan *module.ReadingPlan) {
	key := fmt.Sprintf("reading_plan@%s", plan.ID)
	values := make(map[string]interface{})
	values["id"] = plan.ID
	values["title"] = plan.Title
	values["description"] = plan.Description
	values["start_date"] = plan.StartDate
	values["end_date"] = plan.EndDate
	values["target_books"] = strings.Join(plan.TargetBooks, ",")
	values["status"] = plan.Status
	values["progress"] = plan.Progress
	values["create_time"] = plan.CreateTime
	values["update_time"] = plan.UpdateTime
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveReadingPlan error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveReadingPlan success key=%s", key)
	}
}

func GetAllReadingPlans() []*module.ReadingPlan {
	keys, err := db.client.Keys("reading_plan@*").Result()
	if err != nil {
		log.ErrorF("GetAllReadingPlans error keys=reading_plan@* err=%s", err.Error())
		return nil
	}
	
	plans := make([]*module.ReadingPlan, 0)
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllReadingPlans error key=%s err=%s", key, err.Error())
			continue
		}
		
		plan := toReadingPlan(m)
		if plan != nil {
			plans = append(plans, plan)
		}
	}
	
	return plans
}

func toReadingPlan(m map[string]string) *module.ReadingPlan {
	id, ok := m["id"]
	if !ok {
		return nil
	}
	
	title := m["title"]
	description := m["description"]
	startDate := m["start_date"]
	endDate := m["end_date"]
	targetBooksStr := m["target_books"]
	status := m["status"]
	progressStr := m["progress"]
	createTime := m["create_time"]
	updateTime := m["update_time"]
	
	var targetBooks []string
	if targetBooksStr != "" {
		targetBooks = strings.Split(targetBooksStr, ",")
	}
	
	progress := 0.0
	if progressStr != "" {
		if p, err := strconv.ParseFloat(progressStr, 64); err == nil {
			progress = p
		}
	}
	
	return &module.ReadingPlan{
		ID:          id,
		Title:       title,
		Description: description,
		StartDate:   startDate,
		EndDate:     endDate,
		TargetBooks: targetBooks,
		Status:      status,
		Progress:    progress,
		CreateTime:  createTime,
		UpdateTime:  updateTime,
	}
}

// 阅读目标存储
func SaveReadingGoal(goal *module.ReadingGoal) {
	key := fmt.Sprintf("reading_goal@%s", goal.ID)
	values := make(map[string]interface{})
	values["id"] = goal.ID
	values["year"] = goal.Year
	values["month"] = goal.Month
	values["target_type"] = goal.TargetType
	values["target_value"] = goal.TargetValue
	values["current_value"] = goal.CurrentValue
	values["status"] = goal.Status
	values["create_time"] = goal.CreateTime
	values["update_time"] = goal.UpdateTime
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveReadingGoal error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveReadingGoal success key=%s", key)
	}
}

func GetAllReadingGoals() []*module.ReadingGoal {
	keys, err := db.client.Keys("reading_goal@*").Result()
	if err != nil {
		log.ErrorF("GetAllReadingGoals error keys=reading_goal@* err=%s", err.Error())
		return nil
	}
	
	goals := make([]*module.ReadingGoal, 0)
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllReadingGoals error key=%s err=%s", key, err.Error())
			continue
		}
		
		goal := toReadingGoal(m)
		if goal != nil {
			goals = append(goals, goal)
		}
	}
	
	return goals
}

func toReadingGoal(m map[string]string) *module.ReadingGoal {
	id, ok := m["id"]
	if !ok {
		return nil
	}
	
	yearStr := m["year"]
	monthStr := m["month"]
	targetType := m["target_type"]
	targetValueStr := m["target_value"]
	currentValueStr := m["current_value"]
	status := m["status"]
	createTime := m["create_time"]
	updateTime := m["update_time"]
	
	year, _ := strconv.Atoi(yearStr)
	month, _ := strconv.Atoi(monthStr)
	targetValue, _ := strconv.Atoi(targetValueStr)
	currentValue, _ := strconv.Atoi(currentValueStr)
	
	return &module.ReadingGoal{
		ID:           id,
		Year:         year,
		Month:        month,
		TargetType:   targetType,
		TargetValue:  targetValue,
		CurrentValue: currentValue,
		Status:       status,
		CreateTime:   createTime,
		UpdateTime:   updateTime,
	}
}

// 书籍收藏夹存储
func SaveBookCollection(collection *module.BookCollection) {
	key := fmt.Sprintf("book_collection@%s", collection.ID)
	values := make(map[string]interface{})
	values["id"] = collection.ID
	values["name"] = collection.Name
	values["description"] = collection.Description
	values["book_ids"] = strings.Join(collection.BookIDs, ",")
	values["is_public"] = collection.IsPublic
	values["tags"] = strings.Join(collection.Tags, ",")
	values["create_time"] = collection.CreateTime
	values["update_time"] = collection.UpdateTime
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveBookCollection error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveBookCollection success key=%s", key)
	}
}

func GetAllBookCollections() []*module.BookCollection {
	keys, err := db.client.Keys("book_collection@*").Result()
	if err != nil {
		log.ErrorF("GetAllBookCollections error keys=book_collection@* err=%s", err.Error())
		return nil
	}
	
	collections := make([]*module.BookCollection, 0)
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllBookCollections error key=%s err=%s", key, err.Error())
			continue
		}
		
		collection := toBookCollection(m)
		if collection != nil {
			collections = append(collections, collection)
		}
	}
	
	return collections
}

func toBookCollection(m map[string]string) *module.BookCollection {
	id, ok := m["id"]
	if !ok {
		return nil
	}
	
	name := m["name"]
	description := m["description"]
	bookIDsStr := m["book_ids"]
	isPublicStr := m["is_public"]
	tagsStr := m["tags"]
	createTime := m["create_time"]
	updateTime := m["update_time"]
	
	var bookIDs []string
	if bookIDsStr != "" {
		bookIDs = strings.Split(bookIDsStr, ",")
	}
	
	var tags []string
	if tagsStr != "" {
		tags = strings.Split(tagsStr, ",")
	}
	
	isPublic, _ := strconv.ParseBool(isPublicStr)
	
	return &module.BookCollection{
		ID:          id,
		Name:        name,
		Description: description,
		BookIDs:     bookIDs,
		IsPublic:    isPublic,
		Tags:        tags,
		CreateTime:  createTime,
		UpdateTime:  updateTime,
	}
}

// 阅读时间记录存储
func SaveReadingTimeRecord(record *module.ReadingTimeRecord) {
	key := fmt.Sprintf("reading_time_record@%s", record.ID)
	values := make(map[string]interface{})
	values["id"] = record.ID
	values["book_id"] = record.BookID
	values["start_time"] = record.StartTime
	values["end_time"] = record.EndTime
	values["duration"] = record.Duration
	values["pages"] = record.Pages
	values["notes"] = record.Notes
	values["create_time"] = record.CreateTime
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveReadingTimeRecord error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveReadingTimeRecord success key=%s", key)
	}
}

func GetAllReadingTimeRecords() map[string][]*module.ReadingTimeRecord {
	keys, err := db.client.Keys("reading_time_record@*").Result()
	if err != nil {
		log.ErrorF("GetAllReadingTimeRecords error keys=reading_time_record@* err=%s", err.Error())
		return nil
	}
	
	records := make(map[string][]*module.ReadingTimeRecord)
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllReadingTimeRecords error key=%s err=%s", key, err.Error())
			continue
		}
		
		record := toReadingTimeRecord(m)
		if record != nil {
			bookID := record.BookID
			if records[bookID] == nil {
				records[bookID] = []*module.ReadingTimeRecord{}
			}
			records[bookID] = append(records[bookID], record)
		}
	}
	
	return records
}

func toReadingTimeRecord(m map[string]string) *module.ReadingTimeRecord {
	id, ok := m["id"]
	if !ok {
		return nil
	}
	
	bookID := m["book_id"]
	startTime := m["start_time"]
	endTime := m["end_time"]
	durationStr := m["duration"]
	pagesStr := m["pages"]
	notes := m["notes"]
	createTime := m["create_time"]
	
	duration, _ := strconv.Atoi(durationStr)
	pages, _ := strconv.Atoi(pagesStr)
	
	return &module.ReadingTimeRecord{
		ID:         id,
		BookID:     bookID,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		Pages:      pages,
		Notes:      notes,
		CreateTime: createTime,
	}
}

// 删除相关函数

// DeleteBook 删除指定key的数据
func DeleteBook(key string) {
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteBook error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteBook success key=%s", key)
	}
}

// DeleteReadingPlan 删除阅读计划
func DeleteReadingPlan(planID string) {
	key := fmt.Sprintf("reading_plan@%s", planID)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteReadingPlan error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteReadingPlan success key=%s", key)
	}
}

// DeleteReadingGoal 删除阅读目标
func DeleteReadingGoal(goalID string) {
	key := fmt.Sprintf("reading_goal@%s", goalID)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteReadingGoal error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteReadingGoal success key=%s", key)
	}
}

// DeleteBookCollection 删除书籍收藏夹
func DeleteBookCollection(collectionID string) {
	key := fmt.Sprintf("book_collection@%s", collectionID)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteBookCollection error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteBookCollection success key=%s", key)
	}
}

// DeleteReadingTimeRecord 删除阅读时间记录
func DeleteReadingTimeRecord(recordID string) {
	key := fmt.Sprintf("reading_time_record@%s", recordID)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteReadingTimeRecord error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteReadingTimeRecord success key=%s", key)
	}
}

func toBlogComments(title string,m map[string]string,bcs map[string]*module.BlogComments){

	bc,ok := bcs[title]
	if !ok {
		bc = &module.BlogComments {
			Title : title,
		}
		bcs[title] = bc
	}

	for _,v := range m {
		owner := ""
		msg  := ""
		ct   := ""
		mt   := ""
		mail := ""
		idx  := -1

		// analy the hash value, split by ASCII 0x01 which is can not print
		tokens := strings.Split(v,"\x01")
		log.DebugF("toBlogComments v=%s tokens_len=%d",v,len(tokens))
		for _,t := range tokens {
			kv := strings.Split(t,"=")
			if len(kv) >= 2 {
				k := kv[0]
				v := t[strings.Index(t,"=")+1:]
				log.DebugF("k=%s v=%s",k,v)

				if strings.ToLower(k) == "owner" {
					owner = v
				}else if strings.ToLower(k) == "msg" {
				msg  = v
				}else if strings.ToLower(k) == "ct"  {
					ct   = v
				}else if strings.ToLower(k) == "mt"  {
					mt   = v
				}else if strings.ToLower(k) == "mail"  {
					mail   = v
				}else if strings.ToLower(k) == "idx" {
					the_idx,err := strconv.Atoi(v)
					if err != nil {
						log.ErrorF("split idx conv to int error %s the_idx=%d",err.Error(),the_idx)
					}else{
						idx = the_idx
					}
				} 

			}else{
				log.ErrorF("split tokens %s error kv <= 2",t)
			}

		}

		if idx < 0 {
			log.ErrorF("toBlogComments idx<0 idx=%d",idx)
			continue
		}

		c := module.Comment {
			Owner: owner,
			Msg : msg,
			CreateTime : ct,
			ModifyTime : mt,
			Mail : mail,
			Idx : idx,
		}
		bc.Comments = append(bc.Comments,&c)
	}


	// sort by c.Idx
	sort.SliceStable(bc.Comments,func(i,j int) bool {
		return bc.Comments[i].Idx < bc.Comments[j].Idx
	})

	showBlogComments(bc)
}



func showBlogComments(cs *module.BlogComments){
	log.DebugF("title=%s",cs.Title)	
	for _,c := range cs.Comments {
		log.DebugF("Idx=%d",c.Idx)	
		log.DebugF("owner=%s",c.Owner)	
		log.DebugF("msg=%s",c.Msg)	
		log.DebugF("ct=%s",c.CreateTime)	
		log.DebugF("mt=%s",c.ModifyTime)	
	}
}

func toCooperation(m map[string]string) *module.Cooperation{
	now :=  strTime()
	ct,ok := m["ct"]
	if !ok { ct = now }
	account,ok := m["account"]
	if !ok { return nil }
	pwd,ok :=m["pwd"]
	if !ok { return nil }
	tags, ok := m["tags"]
	if !ok { tags = "" }
	blogs, ok := m["blogs"]
	if !ok { blogs = "" }


	c := &module.Cooperation{
		Account:account,
		Password:pwd,
		CreateTime:ct,
		Tags : tags,
		Blogs : blogs,
	}
	return c
}

func DelCooperation(account string)int{
	key := fmt.Sprintf("cooperation@%s",account)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("delete cooperation error key=%s err=%s",key,err.Error())
		return  1
	}

	log.DebugF("delete account=%s",key)

	return 0
}

func SaveCooperation(account string,pwd string,blogs string,tags string) int {
	key := fmt.Sprintf("cooperation@%s",account)
	values := make(map[string]interface{})
	values["account"] = account
	values["pwd"] = pwd
	values["ct"] = strTime()
	values["blogs"] = blogs
	values["tags"] = tags
	err := db.client.HMSet(key,values).Err()
	if err != nil {
		log.ErrorF("savecooperation error key=%s err=%s",key,err.Error())
		return 1
	}
	log.DebugF("redis savecooperation success account=%s pwd=%s",key,pwd)
	return 0
}

// 评论用户管理相关函数
func SaveCommentUser(user *module.CommentUser) {
	key := fmt.Sprintf("comment_user@%s", user.UserID)
	values := make(map[string]interface{})
	values["user_id"] = user.UserID
	values["username"] = user.Username
	values["email"] = user.Email
	values["avatar"] = user.Avatar
	values["register_time"] = user.RegisterTime
	values["last_active"] = user.LastActive
	values["comment_count"] = user.CommentCount
	values["reputation"] = user.Reputation
	values["status"] = user.Status
	values["is_verified"] = user.IsVerified
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveCommentUser error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveCommentUser success key=%s", key)
	}
}

func SaveCommentSession(session *module.CommentSession) {
	key := fmt.Sprintf("comment_session@%s", session.SessionID)
	values := make(map[string]interface{})
	values["session_id"] = session.SessionID
	values["user_id"] = session.UserID
	values["ip"] = session.IP
	values["user_agent"] = session.UserAgent
	values["create_time"] = session.CreateTime
	values["expire_time"] = session.ExpireTime
	values["is_active"] = session.IsActive
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveCommentSession error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveCommentSession success key=%s", key)
	}
}

func SaveUsernameReservation(reservation *module.UsernameReservation) {
	key := fmt.Sprintf("username_reservation@%s", reservation.Username)
	values := make(map[string]interface{})
	values["username"] = reservation.Username
	values["user_id"] = reservation.UserID
	values["reserve_time"] = reservation.ReserveTime
	values["is_temporary"] = reservation.IsTemporary
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveUsernameReservation error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveUsernameReservation success key=%s", key)
	}
}

func GetAllCommentUsers() map[string]*module.CommentUser {
	keys, err := db.client.Keys("comment_user@*").Result()
	if err != nil {
		log.ErrorF("GetAllCommentUsers error keys=comment_user@* err=%s", err.Error())
		return nil
	}
	
	users := make(map[string]*module.CommentUser)
	
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllCommentUsers error key=%s err=%s", key, err.Error())
			continue
		}
		
		user := toCommentUser(m)
		if user != nil {
			users[user.UserID] = user
			log.DebugF("GetAllCommentUsers success key=%s", key)
		}
	}
	
	return users
}

func GetAllUsernameReservations() map[string]*module.UsernameReservation {
	keys, err := db.client.Keys("username_reservation@*").Result()
	if err != nil {
		log.ErrorF("GetAllUsernameReservations error keys=username_reservation@* err=%s", err.Error())
		return nil
	}
	
	reservations := make(map[string]*module.UsernameReservation)
	
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllUsernameReservations error key=%s err=%s", key, err.Error())
			continue
		}
		
		reservation := toUsernameReservation(m)
		if reservation != nil {
			reservations[reservation.Username] = reservation
		}
	}
	
	return reservations
}

func GetAllCommentSessions() map[string]*module.CommentSession {
	keys, err := db.client.Keys("comment_session@*").Result()
	if err != nil {
		log.ErrorF("GetAllCommentSessions error keys=comment_session@* err=%s", err.Error())
		return nil
	}
	
	sessions := make(map[string]*module.CommentSession)
	
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllCommentSessions error key=%s err=%s", key, err.Error())
			continue
		}
		
		session := toCommentSession(m)
		if session != nil {
			sessions[session.SessionID] = session
		}
	}
	
	return sessions
}

func DeleteCommentSession(sessionID string) {
	key := fmt.Sprintf("comment_session@%s", sessionID)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteCommentSession error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteCommentSession success key=%s", key)
	}
}

func DeleteUsernameReservation(username string) {
	key := fmt.Sprintf("username_reservation@%s", username)
	err := db.client.Del(key).Err()
	if err != nil {
		log.ErrorF("DeleteUsernameReservation error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("DeleteUsernameReservation success key=%s", key)
	}
}

func toCommentUser(m map[string]string) *module.CommentUser {
	userID, ok := m["user_id"]
	if !ok {
		return nil
	}
	
	commentCount, _ := strconv.Atoi(m["comment_count"])
	reputation, _ := strconv.Atoi(m["reputation"])
	status, _ := strconv.Atoi(m["status"])
	isVerified, _ := strconv.ParseBool(m["is_verified"])
	
	return &module.CommentUser{
		UserID:       userID,
		Username:     m["username"],
		Email:        m["email"],
		Avatar:       m["avatar"],
		RegisterTime: m["register_time"],
		LastActive:   m["last_active"],
		CommentCount: commentCount,
		Reputation:   reputation,
		Status:       status,
		IsVerified:   isVerified,
	}
}

func toUsernameReservation(m map[string]string) *module.UsernameReservation {
	username, ok := m["username"]
	if !ok {
		return nil
	}
	
	isTemporary, _ := strconv.ParseBool(m["is_temporary"])
	
	return &module.UsernameReservation{
		Username:    username,
		UserID:      m["user_id"],
		ReserveTime: m["reserve_time"],
		IsTemporary: isTemporary,
	}
}

func toCommentSession(m map[string]string) *module.CommentSession {
	sessionID, ok := m["session_id"]
	if !ok {
		return nil
	}
	
	isActive, _ := strconv.ParseBool(m["is_active"])
	
	return &module.CommentSession{
		SessionID:  sessionID,
		UserID:     m["user_id"],
		IP:         m["ip"],
		UserAgent:  m["user_agent"],
		CreateTime: m["create_time"],
		ExpireTime: m["expire_time"],
		IsActive:   isActive,
	}
}

// 读书功能相关的数据持久化函数
func SaveBook(book *module.Book) {
	key := fmt.Sprintf("book@%s", book.ID)
	values := make(map[string]interface{})
	values["id"] = book.ID
	values["title"] = book.Title
	values["author"] = book.Author
	values["isbn"] = book.ISBN
	values["publisher"] = book.Publisher
	values["publish_date"] = book.PublishDate
	values["cover_url"] = book.CoverUrl
	values["description"] = book.Description
	values["total_pages"] = book.TotalPages
	values["current_page"] = book.CurrentPage
	values["category"] = strings.Join(book.Category, ",")
	values["tags"] = strings.Join(book.Tags, ",")
	values["source_url"] = book.SourceUrl
	values["add_time"] = book.AddTime
	values["rating"] = book.Rating
	values["status"] = book.Status
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveBook error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveBook success key=%s", key)
	}
}

func SaveReadingRecord(record *module.ReadingRecord) {
	key := fmt.Sprintf("reading_record@%s", record.BookID)
	values := make(map[string]interface{})
	values["book_id"] = record.BookID
	values["status"] = record.Status
	values["start_date"] = record.StartDate
	values["end_date"] = record.EndDate
	values["current_page"] = record.CurrentPage
	values["total_reading_time"] = record.TotalReadingTime
	values["last_update_time"] = record.LastUpdateTime
	
	// 序列化阅读会话
	sessionsData, _ := json.Marshal(record.ReadingSessions)
	values["reading_sessions"] = string(sessionsData)
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveReadingRecord error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveReadingRecord success key=%s", key)
	}
}

func SaveBookNotes(bookID string, notes []*module.BookNote) {
	key := fmt.Sprintf("book_notes@%s", bookID)
	
	// 先删除旧数据
	db.client.Del(key)
	
	if len(notes) > 0 {
		values := make(map[string]interface{})
		for i, note := range notes {
			noteData, _ := json.Marshal(note)
			values[fmt.Sprintf("note_%d", i)] = string(noteData)
		}
		
		err := db.client.HMSet(key, values).Err()
		if err != nil {
			log.ErrorF("SaveBookNotes error key=%s err=%s", key, err.Error())
		} else {
			log.DebugF("SaveBookNotes success key=%s count=%d", key, len(notes))
		}
	}
}

func SaveBookInsight(insight *module.BookInsight) {
	key := fmt.Sprintf("book_insight@%s", insight.ID)
	values := make(map[string]interface{})
	values["id"] = insight.ID
	values["book_id"] = insight.BookID
	values["title"] = insight.Title
	values["content"] = insight.Content
	values["rating"] = insight.Rating
	values["create_time"] = insight.CreateTime
	values["update_time"] = insight.UpdateTime
	
	// 序列化数组字段
	keyTakeawaysData, _ := json.Marshal(insight.KeyTakeaways)
	applicationsData, _ := json.Marshal(insight.Applications)
	tagsData, _ := json.Marshal(insight.Tags)
	
	values["key_takeaways"] = string(keyTakeawaysData)
	values["applications"] = string(applicationsData)
	values["tags"] = string(tagsData)
	
	err := db.client.HMSet(key, values).Err()
	if err != nil {
		log.ErrorF("SaveBookInsight error key=%s err=%s", key, err.Error())
	} else {
		log.DebugF("SaveBookInsight success key=%s", key)
	}
}

func GetAllBooks() []*module.Book {
	keys, err := db.client.Keys("book@*").Result()
	if err != nil {
		log.ErrorF("GetAllBooks error keys=book@* err=%s", err.Error())
		return nil
	}
	
	var books []*module.Book
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllBooks error key=%s err=%s", key, err.Error())
			continue
		}
		
		book := toBook(m)
		if book != nil {
			books = append(books, book)
		}
	}
	
	return books
}

func GetAllReadingRecords() []*module.ReadingRecord {
	keys, err := db.client.Keys("reading_record@*").Result()
	if err != nil {
		log.ErrorF("GetAllReadingRecords error keys=reading_record@* err=%s", err.Error())
		return nil
	}
	
	var records []*module.ReadingRecord
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllReadingRecords error key=%s err=%s", key, err.Error())
			continue
		}
		
		record := toReadingRecord(m)
		if record != nil {
			records = append(records, record)
		}
	}
	
	return records
}

func GetAllBookNotes() map[string][]*module.BookNote {
	keys, err := db.client.Keys("book_notes@*").Result()
	if err != nil {
		log.ErrorF("GetAllBookNotes error keys=book_notes@* err=%s", err.Error())
		return nil
	}
	
	allNotes := make(map[string][]*module.BookNote)
	for _, key := range keys {
		bookID := key[strings.Index(key, "@")+1:]
		
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllBookNotes error key=%s err=%s", key, err.Error())
			continue
		}
		
		var notes []*module.BookNote
		for _, noteData := range m {
			var note module.BookNote
			if err := json.Unmarshal([]byte(noteData), &note); err == nil {
				notes = append(notes, &note)
			}
		}
		
		if len(notes) > 0 {
			allNotes[bookID] = notes
		}
	}
	
	return allNotes
}

func GetAllBookInsights() []*module.BookInsight {
	keys, err := db.client.Keys("book_insight@*").Result()
	if err != nil {
		log.ErrorF("GetAllBookInsights error keys=book_insight@* err=%s", err.Error())
		return nil
	}
	
	var insights []*module.BookInsight
	for _, key := range keys {
		m, err := db.client.HGetAll(key).Result()
		if err != nil {
			log.ErrorF("GetAllBookInsights error key=%s err=%s", key, err.Error())
			continue
		}
		
		insight := toBookInsight(m)
		if insight != nil {
			insights = append(insights, insight)
		}
	}
	
	return insights
}

// 辅助转换函数
func toBook(m map[string]string) *module.Book {
	id, ok := m["id"]
	if !ok {
		return nil
	}
	
	totalPages, _ := strconv.Atoi(m["total_pages"])
	currentPage, _ := strconv.Atoi(m["current_page"])
	rating, _ := strconv.ParseFloat(m["rating"], 64)
	
	var category, tags []string
	if m["category"] != "" {
		category = strings.Split(m["category"], ",")
	}
	if m["tags"] != "" {
		tags = strings.Split(m["tags"], ",")
	}
	
	return &module.Book{
		ID:          id,
		Title:       m["title"],
		Author:      m["author"],
		ISBN:        m["isbn"],
		Publisher:   m["publisher"],
		PublishDate: m["publish_date"],
		CoverUrl:    m["cover_url"],
		Description: m["description"],
		TotalPages:  totalPages,
		CurrentPage: currentPage,
		Category:    category,
		Tags:        tags,
		SourceUrl:   m["source_url"],
		AddTime:     m["add_time"],
		Rating:      rating,
		Status:      m["status"],
	}
}

func toReadingRecord(m map[string]string) *module.ReadingRecord {
	bookID, ok := m["book_id"]
	if !ok {
		return nil
	}
	
	currentPage, _ := strconv.Atoi(m["current_page"])
	totalReadingTime, _ := strconv.Atoi(m["total_reading_time"])
	
	var sessions []module.ReadingSession
	if m["reading_sessions"] != "" {
		json.Unmarshal([]byte(m["reading_sessions"]), &sessions)
	}
	
	return &module.ReadingRecord{
		BookID:           bookID,
		Status:           m["status"],
		StartDate:        m["start_date"],
		EndDate:          m["end_date"],
		CurrentPage:      currentPage,
		TotalReadingTime: totalReadingTime,
		ReadingSessions:  sessions,
		LastUpdateTime:   m["last_update_time"],
	}
}

func toBookInsight(m map[string]string) *module.BookInsight {
	id, ok := m["id"]
	if !ok {
		return nil
	}
	
	rating, _ := strconv.Atoi(m["rating"])
	
	var keyTakeaways, applications, tags []string
	if m["key_takeaways"] != "" {
		json.Unmarshal([]byte(m["key_takeaways"]), &keyTakeaways)
	}
	if m["applications"] != "" {
		json.Unmarshal([]byte(m["applications"]), &applications)
	}
	if m["tags"] != "" {
		json.Unmarshal([]byte(m["tags"]), &tags)
	}
	
	return &module.BookInsight{
		ID:           id,
		BookID:       m["book_id"],
		Title:        m["title"],
		Content:      m["content"],
		KeyTakeaways: keyTakeaways,
		Applications: applications,
		Rating:       rating,
		Tags:         tags,
		CreateTime:   m["create_time"],
		UpdateTime:   m["update_time"],
	}
}

func GetCooperations()map[string]*module.Cooperation{
    keys,err := db.client.Keys("cooperation@*").Result()
	if err !=nil {
		log.ErrorF("getblogs error keys=cooperation@* err=%s",err.Error())
		return nil
	}

	cooperations := make(map[string]*module.Cooperation)

	for _,key := range keys {
		log.DebugF("getcooperation key=%s",key)
		m ,err := db.client.HGetAll(key).Result()
		if err!=nil {
				log.ErrorF("getcooperation error key=%s err=%s",key,err.Error())
				continue
		}
		log.DebugF("getcooperation success key=%s",key)
		c := toCooperation(m)
		cooperations[c.Account] = c
	}

	return cooperations
}


