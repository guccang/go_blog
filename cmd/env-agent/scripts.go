package main

// ScriptKey 脚本索引键
type ScriptKey struct {
	OS       string // "linux", "darwin", "windows"
	Distro   string // "ubuntu", "centos", "" (通用)
	Software string // "python", "go", "redis", ...
	Action   string // "check", "install"
}

// presetScripts 预置脚本表
var presetScripts = map[ScriptKey]string{
	// ========================= Ubuntu/Debian =========================
	{"linux", "ubuntu", "python", "check"}:   "python3 --version 2>&1",
	{"linux", "ubuntu", "python", "install"}: "apt-get update -qq && apt-get install -y python3",
	{"linux", "ubuntu", "go", "check"}:       "go version 2>&1",
	{"linux", "ubuntu", "go", "install"}:     "apt-get update -qq && apt-get install -y golang-go",
	{"linux", "ubuntu", "node", "check"}:     "node --version 2>&1",
	{"linux", "ubuntu", "node", "install"}:   "apt-get update -qq && apt-get install -y nodejs",
	{"linux", "ubuntu", "redis", "check"}:    "redis-server --version 2>&1",
	{"linux", "ubuntu", "redis", "install"}:  "apt-get update -qq && apt-get install -y redis-server",
	{"linux", "ubuntu", "mysql", "check"}:    "mysql --version 2>&1",
	{"linux", "ubuntu", "mysql", "install"}:  "apt-get update -qq && apt-get install -y mysql-server",
	{"linux", "ubuntu", "docker", "check"}:   "docker --version 2>&1",
	{"linux", "ubuntu", "docker", "install"}: "apt-get update -qq && apt-get install -y docker.io",
	{"linux", "ubuntu", "git", "check"}:      "git --version 2>&1",
	{"linux", "ubuntu", "git", "install"}:    "apt-get update -qq && apt-get install -y git",
	{"linux", "ubuntu", "nginx", "check"}:    "nginx -v 2>&1",
	{"linux", "ubuntu", "nginx", "install"}:  "apt-get update -qq && apt-get install -y nginx",
	{"linux", "ubuntu", "curl", "check"}:     "curl --version 2>&1",
	{"linux", "ubuntu", "curl", "install"}:   "apt-get update -qq && apt-get install -y curl",
	{"linux", "ubuntu", "java", "check"}:     "java -version 2>&1",
	{"linux", "ubuntu", "java", "install"}:   "apt-get update -qq && apt-get install -y default-jdk",

	// Debian 复用 Ubuntu 的脚本
	{"linux", "debian", "python", "install"}: "apt-get update -qq && apt-get install -y python3",
	{"linux", "debian", "go", "install"}:     "apt-get update -qq && apt-get install -y golang-go",
	{"linux", "debian", "node", "install"}:   "apt-get update -qq && apt-get install -y nodejs",
	{"linux", "debian", "redis", "install"}:  "apt-get update -qq && apt-get install -y redis-server",
	{"linux", "debian", "docker", "install"}: "apt-get update -qq && apt-get install -y docker.io",
	{"linux", "debian", "git", "install"}:    "apt-get update -qq && apt-get install -y git",
	{"linux", "debian", "curl", "install"}:   "apt-get update -qq && apt-get install -y curl",

	// ========================= CentOS/RHEL =========================
	{"linux", "centos", "python", "install"}: "yum install -y python3",
	{"linux", "centos", "go", "install"}:     "yum install -y golang",
	{"linux", "centos", "node", "install"}:   "yum install -y nodejs",
	{"linux", "centos", "redis", "install"}:  "yum install -y redis",
	{"linux", "centos", "mysql", "install"}:  "yum install -y mysql-server",
	{"linux", "centos", "docker", "install"}: "yum install -y docker",
	{"linux", "centos", "git", "install"}:    "yum install -y git",
	{"linux", "centos", "curl", "install"}:   "yum install -y curl",
	{"linux", "centos", "java", "install"}:   "yum install -y java-11-openjdk",

	// Fedora
	{"linux", "fedora", "python", "install"}: "dnf install -y python3",
	{"linux", "fedora", "go", "install"}:     "dnf install -y golang",
	{"linux", "fedora", "node", "install"}:   "dnf install -y nodejs",
	{"linux", "fedora", "redis", "install"}:  "dnf install -y redis",
	{"linux", "fedora", "docker", "install"}: "dnf install -y docker",
	{"linux", "fedora", "git", "install"}:    "dnf install -y git",

	// ========================= 通用 Linux（兜底） =========================
	{"linux", "", "python", "check"}:   "python3 --version 2>&1 || python --version 2>&1",
	{"linux", "", "go", "check"}:       "go version 2>&1",
	{"linux", "", "node", "check"}:     "node --version 2>&1",
	{"linux", "", "redis", "check"}:    "redis-server --version 2>&1",
	{"linux", "", "mysql", "check"}:    "mysql --version 2>&1",
	{"linux", "", "docker", "check"}:   "docker --version 2>&1",
	{"linux", "", "git", "check"}:      "git --version 2>&1",
	{"linux", "", "java", "check"}:     "java -version 2>&1",
	{"linux", "", "nginx", "check"}:    "nginx -v 2>&1",
	{"linux", "", "curl", "check"}:     "curl --version 2>&1",

	// ========================= macOS =========================
	{"darwin", "", "python", "check"}:   "python3 --version 2>&1",
	{"darwin", "", "python", "install"}: "brew install python3",
	{"darwin", "", "go", "check"}:       "go version 2>&1",
	{"darwin", "", "go", "install"}:     "brew install go",
	{"darwin", "", "node", "check"}:     "node --version 2>&1",
	{"darwin", "", "node", "install"}:   "brew install node",
	{"darwin", "", "redis", "check"}:    "redis-server --version 2>&1",
	{"darwin", "", "redis", "install"}:  "brew install redis",
	{"darwin", "", "docker", "check"}:   "docker --version 2>&1",
	{"darwin", "", "git", "check"}:      "git --version 2>&1",
	{"darwin", "", "git", "install"}:    "brew install git",
}

// findScript 查找预置脚本
// 优先精确匹配 distro，再回退到通用（distro=""）
func findScript(os, distro, software, action string) (string, bool) {
	// 精确匹配
	if script, ok := presetScripts[ScriptKey{os, distro, software, action}]; ok {
		return script, true
	}
	// 回退到通用
	if distro != "" {
		if script, ok := presetScripts[ScriptKey{os, "", software, action}]; ok {
			return script, true
		}
	}
	return "", false
}
