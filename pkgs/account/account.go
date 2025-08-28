package account

import (
	"blog"
	"encoding/json"
	"fmt"
	"module"
	log "mylog"
	"strconv"
	"strings"
	"time"
)

// AccountInfo 账户信息结构体
type AccountInfo struct {
	Name      string   `json:"name"`       // 姓名
	Phone     string   `json:"phone"`      // 电话
	Email     string   `json:"email"`      // 邮箱
	Age       int      `json:"age"`        // 年龄
	Height    float64  `json:"height"`     // 身高(cm)
	Weight    float64  `json:"weight"`     // 体重(kg)
	Hobbies   []string `json:"hobbies"`    // 爱好
	Avatar    string   `json:"avatar"`     // 头像(单字符)
	Bio       string   `json:"bio"`        // 个人简介
	Location  string   `json:"location"`   // 所在地
	Website   string   `json:"website"`    // 个人网站
	Birthday  string   `json:"birthday"`   // 生日(YYYY-MM-DD)
	UpdatedAt string   `json:"updated_at"` // 更新时间
}

// GetAccountInfo 获取账户信息
func GetAccountInfo(account string) (*AccountInfo, error) {
	if account == "" {
		return nil, fmt.Errorf("account is empty")
	}

	// 从博客中获取账户信息
	blogPost := blog.GetBlogWithAccount(account, "sys_account_info")
	if blogPost == nil {
		// 返回默认账户信息
		return &AccountInfo{
			Name:      account,
			Avatar:    strings.ToUpper(string(account[0])),
			UpdatedAt: time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	// 解析博客内容为JSON
	var info AccountInfo
	if err := json.Unmarshal([]byte(blogPost.Content), &info); err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to parse account info JSON: %v", err)
		return nil, err
	}

	return &info, nil
}

// SaveAccountInfo 保存账户信息
func SaveAccountInfo(account string, info *AccountInfo) error {
	if account == "" {
		log.ErrorF(log.ModuleAccount, "account is empty")
		return fmt.Errorf("account is empty")
	}

	if info == nil {
		log.ErrorF(log.ModuleAccount, "account info is nil")
		return fmt.Errorf("account info is nil")
	}

	// 设置更新时间
	info.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")

	// 如果没有设置头像，使用用户名首字母
	if info.Avatar == "" {
		if info.Name != "" {
			info.Avatar = strings.ToUpper(string(info.Name[0]))
		} else {
			info.Avatar = strings.ToUpper(string(account[0]))
		}
	}

	log.DebugF(log.ModuleAccount, "account info: %v", info)

	// 序列化为JSON
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		log.ErrorF(log.ModuleAccount, "Failed to marshal account info to JSON: %v", err)
		return err
	}

	// 创建博客数据
	udb := &module.UploadedBlogData{
		Title:    "sys_account_info",
		Content:  string(data),
		AuthType: module.EAuthType_private, // 私有权限
		Tags:     "account,info,personal",
		Account:  account,
	}

	// 检查是否已存在账户信息博客
	existingBlog := blog.GetBlogWithAccount(account, "sys_account_info")
	if existingBlog != nil {
		// 修改现有博客
		if result := blog.ModifyBlogWithAccount(account, udb); result != 0 {
			log.ErrorF(log.ModuleAccount, "Failed to modify account info blog")
			return fmt.Errorf("failed to modify account info")
		}
	} else {
		// 创建新博客
		if result := blog.AddBlogWithAccount(account, udb); result != 0 {
			log.ErrorF(log.ModuleAccount, "Failed to add account info blog")
			return fmt.Errorf("failed to add account info")
		}
	}

	log.InfoF(log.ModuleAccount, "Account info saved for user: %s", account)
	return nil
}

// ValidateAccountInfo 验证账户信息
func ValidateAccountInfo(info *AccountInfo) []string {
	var errors []string

	// 验证年龄
	if info.Age < 0 || info.Age > 150 {
		errors = append(errors, "年龄必须在0-150之间")
	}

	// 验证身高
	if info.Height < 0 || info.Height > 300 {
		errors = append(errors, "身高必须在0-300cm之间")
	}

	// 验证体重
	if info.Weight < 0 || info.Weight > 1000 {
		errors = append(errors, "体重必须在0-1000kg之间")
	}

	// 验证电话号码格式
	if info.Phone != "" && !isValidPhone(info.Phone) {
		errors = append(errors, "电话号码格式不正确")
	}

	// 验证邮箱格式
	if info.Email != "" && !isValidEmail(info.Email) {
		errors = append(errors, "邮箱格式不正确")
	}

	// 验证生日格式
	if info.Birthday != "" && !isValidDate(info.Birthday) {
		errors = append(errors, "生日格式不正确，应为YYYY-MM-DD")
	}

	return errors
}

// GetBMI 计算BMI
func (info *AccountInfo) GetBMI() float64 {
	if info.Height <= 0 || info.Weight <= 0 {
		return 0
	}
	heightInMeters := info.Height / 100
	return info.Weight / (heightInMeters * heightInMeters)
}

// GetBMIStatus 获取BMI状态
func (info *AccountInfo) GetBMIStatus() string {
	bmi := info.GetBMI()
	if bmi == 0 {
		return "数据不足"
	}

	if bmi < 18.5 {
		return "偏瘦"
	} else if bmi < 24 {
		return "正常"
	} else if bmi < 28 {
		return "偏胖"
	} else {
		return "肥胖"
	}
}

// GetAge 根据生日计算年龄
func (info *AccountInfo) GetAge() int {
	if info.Birthday == "" {
		return info.Age
	}

	birthday, err := time.Parse("2006-01-02", info.Birthday)
	if err != nil {
		return info.Age
	}

	now := time.Now()
	age := now.Year() - birthday.Year()

	// 如果今年的生日还没过，年龄减1
	if now.Month() < birthday.Month() ||
		(now.Month() == birthday.Month() && now.Day() < birthday.Day()) {
		age--
	}

	return age
}

// 验证电话号码格式的简单实现
func isValidPhone(phone string) bool {
	// 移除所有非数字字符
	digits := ""
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits += string(r)
		}
	}

	// 检查长度（中国手机号11位，固话可能更短）
	return len(digits) >= 7 && len(digits) <= 15
}

// 验证邮箱格式的简单实现
func isValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	localPart := parts[0]
	domainPart := parts[1]

	// 基本检查
	if len(localPart) == 0 || len(domainPart) == 0 {
		return false
	}

	// 域名部分必须包含点
	if !strings.Contains(domainPart, ".") {
		return false
	}

	return true
}

// 验证日期格式
func isValidDate(dateStr string) bool {
	_, err := time.Parse("2006-01-02", dateStr)
	return err == nil
}

// ParseHobbies 解析爱好字符串（逗号分隔）
func ParseHobbies(hobbiesStr string) []string {
	if hobbiesStr == "" {
		return []string{}
	}

	hobbies := strings.Split(hobbiesStr, ",")
	result := make([]string, 0, len(hobbies))

	for _, hobby := range hobbies {
		trimmed := strings.TrimSpace(hobby)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// HobbiesToString 将爱好数组转换为字符串
func HobbiesToString(hobbies []string) string {
	return strings.Join(hobbies, ", ")
}

// FormatFloat 格式化浮点数
func FormatFloat(value float64) string {
	if value == 0 {
		return ""
	}
	return strconv.FormatFloat(value, 'f', 1, 64)
}

// ParseFloat 解析浮点数
func ParseFloat(str string) float64 {
	if str == "" {
		return 0
	}
	value, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0
	}
	return value
}
