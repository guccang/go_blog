package main

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// DeployMode 部署模式
type DeployMode string

const (
	DeployModeAuto DeployMode = ""          // 自动检测（默认）
	DeployModeFull DeployMode = "full"      // 完整部署（覆盖所有）
	DeployModeIncr DeployMode = "increment" // 增量部署（保护配置）
)

// Deployer 部署编排器
type Deployer struct {
	cfg          *DeployConfig  // 全局配置（SSH 等）
	proj         *ProjectConfig // 当前项目配置
	password     string
	packFile     string                      // 打包后的 zip 文件名（不含路径）
	SSHConnected bool                        // SSH 连接是否成功过（密码有效）
	OnProgress   func(level, message string) // daemon 模式进度回调（nil 则输出到 stdout）
	DeployMode   DeployMode                  // 部署模式: auto/full/increment
}

// NewDeployer 创建部署器
func NewDeployer(cfg *DeployConfig, proj *ProjectConfig, password string) *Deployer {
	return &Deployer{cfg: cfg, proj: proj, password: password}
}

// logf 输出进度信息（daemon 模式通过回调，CLI 模式输出到 stdout）
func (d *Deployer) logf(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if d.OnProgress != nil {
		d.OnProgress(level, msg)
	} else {
		fmt.Print(msg)
	}
}

// isLocalTarget 判断是否为本机部署目标
func isLocalTarget(host string) bool {
	_, h := parseHost(host)
	switch strings.ToLower(h) {
	case "local", "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

// Run 执行部署 pipeline
func (d *Deployer) Run(packOnly bool, targetFilter string) error {
	start := time.Now()

	// 自动合并目标 agent 配置文件中的 protected_files
	d.mergeAgentProtectedFiles()

	targets := d.proj.Targets
	if targetFilter != "" {
		targets = nil
		for _, t := range d.proj.Targets {
			if t.Name == targetFilter || t.Host == targetFilter || strings.HasPrefix(t.Name, targetFilter+".") {
				targets = append(targets, t)
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("target %q not found", targetFilter)
		}
	}

	// local target 按 HostPlatform 过滤（配置加载了所有平台，部署时只用当前平台）
	var filteredTargets []*Target
	for _, t := range targets {
		if isLocalTarget(t.Host) && t.Platform != "" && t.Platform != d.cfg.HostPlatform {
			continue
		}
		filteredTargets = append(filteredTargets, t)
	}
	targets = filteredTargets
	if len(targets) == 0 {
		return fmt.Errorf("no matching targets (host_platform=%s)", d.cfg.HostPlatform)
	}

	totalSteps := 4
	if packOnly {
		totalSteps = 1
	}

	// Step 1: 打包（根据 target 平台决定是否交叉编译）
	d.logf("info", "[STEP 1/%d] 打包项目 [%s]...\n", totalSteps, d.proj.Name)
	targetPlatform := d.inferTargetPlatform(targets)
	if err := d.pack(targetPlatform); err != nil {
		return fmt.Errorf("打包失败: %v", err)
	}
	packPath := filepath.Join(d.proj.ProjectDir, d.packFile)
	info, err := os.Stat(packPath)
	if err != nil {
		return fmt.Errorf("找不到打包文件: %v", err)
	}
	d.logf("info", "[STEP 1/%d] 打包完成: %s (%s)\n", totalSteps, d.packFile, formatSize(info.Size()))

	if packOnly {
		d.logf("info", "[DONE] 打包完成，耗时 %s\n", formatDuration(time.Since(start)))
		return nil
	}

	// Step 2-4: 逐个目标部署
	var errs []string
	for _, t := range targets {
		var err error
		if isLocalTarget(t.Host) {
			err = d.deployLocal(t, totalSteps)
		} else if t.Type == "bridge" {
			err = d.deployBridge(t, totalSteps)
		} else {
			err = d.deployRemote(t, totalSteps)
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("[%s] %v", t.Name, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("部署失败: %s", strings.Join(errs, "; "))
	}

	d.logf("info", "[DONE] 项目 [%s] 部署完成，耗时 %s\n", d.proj.Name, formatDuration(time.Since(start)))
	return nil
}

// ========================= 二进制 MD5 对比 =========================

// binaryName 根据 target 平台返回二进制文件名
func (d *Deployer) binaryName(t *Target) string {
	name := d.proj.Name
	if t.Platform == "win" {
		name += ".exe"
	}
	return name
}

// md5File 计算本地文件 MD5（返回十六进制字符串，失败返回空串）
func md5File(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// md5Remote 通过 SSH 计算远程文件 MD5（失败返回空串）
func md5Remote(client *ssh.Client, remotePath string) string {
	session, err := client.NewSession()
	if err != nil {
		return ""
	}
	defer session.Close()
	var buf bytes.Buffer
	session.Stdout = &buf
	// md5sum 输出格式: "hash  filename\n"
	if session.Run(fmt.Sprintf("md5sum %s 2>/dev/null", remotePath)) != nil {
		return ""
	}
	parts := strings.Fields(buf.String())
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// shortMD5 截取 MD5 前16字符用于显示
func shortMD5(hash string) string {
	if len(hash) > 16 {
		return hash[:16]
	}
	return hash
}

// logMD5Diff 对比并输出二进制变更信息
func (d *Deployer) logMD5Diff(before, after string) {
	if before == "" && after == "" {
		return
	}
	if before == "" && after != "" {
		d.logf("info", "🔖 二进制已部署: %s\n", shortMD5(after))
		return
	}
	if after == "" {
		d.logf("info", "⚠ 无法获取部署后二进制信息\n")
		return
	}
	if before != after {
		d.logf("info", "🔖 二进制已更新: %s → %s\n", shortMD5(before), shortMD5(after))
	} else {
		d.logf("info", "⚠ 二进制未变化: %s — 解压或覆盖可能失败\n", shortMD5(after))
	}
}

// deployRemote 远程 SSH 部署（原有逻辑）
func (d *Deployer) deployRemote(t *Target, totalSteps int) error {
	label := t.Host
	if t.Name != "" && !strings.HasPrefix(t.Name, "default-") {
		label = fmt.Sprintf("%s (%s)", t.Name, t.Host)
	}

	user, host := parseHost(t.Host)
	d.logf("info", "[STEP 2/%d] 连接 %s...\n", totalSteps, label)
	client, err := d.connectSSH(user, host, t.Port)
	if err != nil {
		d.logf("error", "[ERROR] 连接 %s 失败: %v\n", label, err)
		return fmt.Errorf("连接 %s 失败: %v", label, err)
	}
	defer client.Close()

	// 读取部署前二进制 MD5
	binaryPath := t.RemoteDir + "/" + d.binaryName(t)
	beforeMD5 := md5Remote(client, binaryPath)

	d.logf("info", "[STEP 2/%d] 上传到 %s:%s...\n", totalSteps, t.Host, t.RemoteDir)
	if err := d.upload(client, t); err != nil {
		d.logf("error", "[ERROR] 上传到 %s 失败: %v\n", label, err)
		return fmt.Errorf("上传到 %s 失败: %v", label, err)
	}

	d.logf("info", "[STEP 3/%d] 解压到 %s:%s...\n", totalSteps, t.Host, t.RemoteDir)

	// 判断部署模式：首次 vs 增量
	isFirstDeploy := beforeMD5 == ""
	effectiveMode := d.resolveDeployMode(isFirstDeploy)
	d.logDeployMode(isFirstDeploy, effectiveMode)

	if isFirstDeploy && len(d.proj.SetupDirs) > 0 {
		// 首次部署：创建 setup_dirs
		mkdirCmd := "mkdir -p"
		for _, dir := range d.proj.SetupDirs {
			mkdirCmd += " " + t.RemoteDir + "/" + strings.TrimRight(dir, "/")
		}
		d.logf("info", "  > 创建数据目录: %s\n", strings.Join(d.proj.SetupDirs, ", "))
		if err := d.runRemoteCmd(client, mkdirCmd); err != nil {
			d.logf("info", "  > 创建目录警告: %v\n", err)
		}
	}

	// 构建 unzip 命令
	var cmd string
	if effectiveMode == DeployModeIncr && len(d.proj.ProtectFiles) > 0 {
		// 增量部署：检查远程已存在的受保护文件，构建排除列表
		d.logf("info", "  > 检查受保护文件: %s\n", strings.Join(d.proj.ProtectFiles, ", "))
		excludes := d.findExistingProtectedFilesRemote(client, t.RemoteDir, d.proj.ProtectFiles)
		if len(excludes) > 0 {
			d.logf("info", "  > 跳过已有文件: %s\n", strings.Join(excludes, ", "))
			cmd = fmt.Sprintf("cd %s && unzip -o %s -x %s", t.RemoteDir, d.packFile, strings.Join(excludes, " "))
		} else {
			d.logf("info", "  > 受保护文件均不存在，全量解压\n")
			cmd = fmt.Sprintf("cd %s && unzip -o %s", t.RemoteDir, d.packFile)
		}
	} else if effectiveMode == DeployModeIncr {
		d.logf("info", "  > 未配置 protect_files，全量解压\n")
		cmd = fmt.Sprintf("cd %s && unzip -o %s", t.RemoteDir, d.packFile)
	} else {
		// full 模式
		cmd = fmt.Sprintf("cd %s && unzip -o %s", t.RemoteDir, d.packFile)
	}

	if err := d.runRemoteCmd(client, cmd); err != nil {
		d.logf("error", "[ERROR] 解压到 %s 失败: %v\n", label, err)
		return fmt.Errorf("解压到 %s 失败: %v", label, err)
	}

	if t.RemoteScript != "" {
		d.logf("info", "[STEP 4/%d] 执行 %s on %s...\n", totalSteps, t.RemoteScript, label)
		if err := d.runPublishCmd(client, t); err != nil {
			d.logf("error", "[ERROR] 执行 %s on %s 失败: %v\n", t.RemoteScript, label, err)
			return fmt.Errorf("执行 %s on %s 失败: %v", t.RemoteScript, label, err)
		}
	} else {
		d.logf("info", "[STEP 4/%d] 无发布脚本，跳过\n", totalSteps)
	}

	// 读取部署后二进制 MD5 并对比
	afterMD5 := md5Remote(client, binaryPath)
	d.logMD5Diff(beforeMD5, afterMD5)

	d.logf("info", "[OK] %s 部署成功\n", label)
	return nil
}

// deployLocal 本机部署：复制 → 解压 → 执行发布脚本
func (d *Deployer) deployLocal(t *Target, totalSteps int) error {
	label := "local"
	if t.Name != "" && !strings.HasPrefix(t.Name, "default-") {
		label = t.Name + " (local)"
	}

	targetDir := t.RemoteDir
	d.logf("info", "[STEP 2/%d] 本机部署到 %s...\n", totalSteps, targetDir)

	// 读取部署前二进制 MD5
	binaryPath := filepath.Join(targetDir, d.binaryName(t))
	beforeMD5 := md5File(binaryPath)

	// 确保目标目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		d.logf("error", "[ERROR] 创建目录 %s 失败: %v\n", targetDir, err)
		return fmt.Errorf("创建目录 %s 失败: %v", targetDir, err)
	}

	// 复制 zip 到目标目录（如果源和目标相同则跳过）
	srcPath := filepath.Join(d.proj.ProjectDir, d.packFile)
	dstPath := filepath.Join(targetDir, d.packFile)
	absSrc, _ := filepath.Abs(srcPath)
	absDst, _ := filepath.Abs(dstPath)
	if absSrc != absDst {
		if err := copyFile(srcPath, dstPath); err != nil {
			d.logf("error", "[ERROR] 复制到 %s 失败: %v\n", dstPath, err)
			return fmt.Errorf("复制到 %s 失败: %v", dstPath, err)
		}
		d.logf("info", "  > 已复制 %s\n", dstPath)
	} else {
		d.logf("info", "  > 源与目标相同，跳过复制\n")
	}

	// 解压（Windows 使用 7z，Linux/Mac 使用 unzip）
	d.logf("info", "[STEP 3/%d] 解压到 %s...\n", totalSteps, targetDir)

	// 判断部署模式：首次 vs 增量
	isFirstDeploy := beforeMD5 == ""
	effectiveMode := d.resolveDeployMode(isFirstDeploy)
	d.logDeployMode(isFirstDeploy, effectiveMode)

	if isFirstDeploy && len(d.proj.SetupDirs) > 0 {
		// 首次部署：创建 setup_dirs
		for _, dir := range d.proj.SetupDirs {
			setupPath := filepath.Join(targetDir, strings.TrimRight(dir, "/"))
			if err := os.MkdirAll(setupPath, 0755); err != nil {
				d.logf("info", "  > 创建目录警告: %v\n", err)
			}
		}
		d.logf("info", "  > 创建数据目录: %s\n", strings.Join(d.proj.SetupDirs, ", "))
	}

	if effectiveMode == DeployModeIncr && len(d.proj.ProtectFiles) > 0 {
		// 增量部署：检查本地已存在的受保护文件，构建排除列表
		d.logf("info", "  > 检查受保护文件: %s\n", strings.Join(d.proj.ProtectFiles, ", "))
		excludes := d.findExistingProtectedFilesLocal(targetDir, d.proj.ProtectFiles)
		if len(excludes) > 0 {
			d.logf("info", "  > 跳过已有文件: %s\n", strings.Join(excludes, ", "))
			if err := d.localUnzipWithExcludes(dstPath, targetDir, excludes); err != nil {
				d.logf("error", "[ERROR] 解压失败: %v\n", err)
				return fmt.Errorf("解压失败: %v", err)
			}
		} else {
			d.logf("info", "  > 受保护文件均不存在，全量解压\n")
			if err := d.localUnzip(dstPath, targetDir); err != nil {
				d.logf("error", "[ERROR] 解压失败: %v\n", err)
				return fmt.Errorf("解压失败: %v", err)
			}
		}
	} else if effectiveMode == DeployModeIncr {
		d.logf("info", "  > 未配置 protect_files，全量解压\n")
		if err := d.localUnzip(dstPath, targetDir); err != nil {
			d.logf("error", "[ERROR] 解压失败: %v\n", err)
			return fmt.Errorf("解压失败: %v", err)
		}
	} else {
		// full 模式
		if err := d.localUnzip(dstPath, targetDir); err != nil {
			d.logf("error", "[ERROR] 解压失败: %v\n", err)
			return fmt.Errorf("解压失败: %v", err)
		}
	}

	// 执行发布脚本
	if t.RemoteScript != "" {
		d.logf("info", "[STEP 4/%d] 执行 %s (local)...\n", totalSteps, t.RemoteScript)
		scriptPath := filepath.Join(targetDir, t.RemoteScript)
		if err := d.runLocalScript(scriptPath, targetDir); err != nil {
			d.logf("error", "[ERROR] 执行 %s 失败: %v\n", t.RemoteScript, err)
			return fmt.Errorf("执行 %s 失败: %v", t.RemoteScript, err)
		}
	} else {
		d.logf("info", "[STEP 4/%d] 无发布脚本，跳过\n", totalSteps)
	}

	// 读取部署后二进制 MD5 并对比
	afterMD5 := md5File(binaryPath)
	d.logMD5Diff(beforeMD5, afterMD5)

	d.logf("info", "[OK] %s 部署成功\n", label)
	return nil
}

// deployBridge 通过 Bridge HTTP API 部署
func (d *Deployer) deployBridge(t *Target, totalSteps int) error {
	label := t.Name
	if label == "" {
		label = t.BridgeURL
	}

	zipPath := filepath.Join(d.proj.ProjectDir, d.packFile)

	// Step 2: 上传
	d.logf("info", "[STEP 2/%d] 上传到 Bridge %s...\n", totalSteps, label)
	filename, err := d.bridgeUpload(t.BridgeURL, t.AuthToken, zipPath)
	if err != nil {
		d.logf("error", "[ERROR] 上传到 Bridge 失败: %v\n", err)
		return fmt.Errorf("上传到 Bridge 失败: %v", err)
	}
	d.logf("info", "[STEP 2/%d] 上传完成: %s\n", totalSteps, filename)

	// Step 3: 触发部署
	script := t.RemoteScript
	if script == "" {
		script = "publish.sh"
	}
	d.logf("info", "[STEP 3/%d] 触发 Bridge 部署 %s → %s...\n", totalSteps, filename, t.RemoteDir)
	deployID, err := d.bridgeDeploy(t.BridgeURL, t.AuthToken, filename, t.RemoteDir, script)
	if err != nil {
		d.logf("error", "[ERROR] 触发 Bridge 部署失败: %v\n", err)
		return fmt.Errorf("触发 Bridge 部署失败: %v", err)
	}
	d.logf("info", "[STEP 3/%d] 部署任务已创建: %s\n", totalSteps, deployID)

	// Step 4: 等待完成
	d.logf("info", "[STEP 4/%d] 等待 Bridge 部署完成...\n", totalSteps)
	if err := d.bridgeWaitDone(t.BridgeURL, t.AuthToken, deployID); err != nil {
		d.logf("error", "[ERROR] Bridge 部署失败: %v\n", err)
		return fmt.Errorf("Bridge 部署失败: %v", err)
	}

	d.logf("info", "[OK] %s Bridge 部署成功\n", label)
	return nil
}

// bridgeUpload 上传 zip 到 bridge server (POST /api/upload multipart)
// 会先计算本地 MD5 并查询服务端，相同 MD5 的文件不重复上传
func (d *Deployer) bridgeUpload(bridgeURL, token, zipPath string) (string, error) {
	// 计算本地文件 MD5
	localMD5, err := d.localFileMD5(zipPath)
	if err != nil {
		return "", fmt.Errorf("计算文件MD5失败: %v", err)
	}

	// 查询服务端已有包列表，检查 MD5 是否重复
	if pkgs, err := d.bridgePackages(bridgeURL, token); err == nil {
		for _, pkg := range pkgs {
			if pkg.MD5 == localMD5 {
				d.logf("info", "文件已存在于服务器（MD5相同）: %s，跳过上传\n", pkg.Name)
				return pkg.Name, nil
			}
		}
	}

	file, err := os.Open(zipPath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer file.Close()

	// 构建 multipart body
	var buf bytes.Buffer
	boundary := fmt.Sprintf("----deploy-%d", time.Now().UnixNano())
	writer := fmt.Sprintf("--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"%s\"\r\nContent-Type: application/zip\r\n\r\n",
		boundary, filepath.Base(zipPath))
	buf.WriteString(writer)

	// 读取文件内容
	fileData, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("读取文件失败: %v", err)
	}
	buf.Write(fileData)
	buf.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	req, err := http.NewRequest("POST", strings.TrimRight(bridgeURL, "/")+"/api/upload", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Filename string `json:"filename"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("上传失败: %s", result.Error)
	}
	return result.Filename, nil
}

// bridgePackages 获取服务端已上传包列表 (GET /api/packages)
func (d *Deployer) bridgePackages(bridgeURL, token string) ([]bridgePackageInfo, error) {
	req, err := http.NewRequest("GET", strings.TrimRight(bridgeURL, "/")+"/api/packages", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var pkgs []bridgePackageInfo
	if err := json.NewDecoder(resp.Body).Decode(&pkgs); err != nil {
		return nil, err
	}
	return pkgs, nil
}

type bridgePackageInfo struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	MD5  string `json:"md5"`
}

// localFileMD5 计算本地文件的 MD5 哈希
func (d *Deployer) localFileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// bridgeDeploy 触发 bridge 部署 (POST /api/deploy)
func (d *Deployer) bridgeDeploy(bridgeURL, token, filename, targetDir, script string) (string, error) {
	reqBody, _ := json.Marshal(map[string]interface{}{
		"filename":      filename,
		"target_dir":    targetDir,
		"script":        script,
		"protect_files": d.proj.ProtectFiles,
		"setup_dirs":    d.proj.SetupDirs,
		"deploy_mode":   string(d.DeployMode),
	})

	req, err := http.NewRequest("POST", strings.TrimRight(bridgeURL, "/")+"/api/deploy",
		bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		DeployID string `json:"deploy_id"`
		Error    string `json:"error"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %v", err)
	}
	if result.Error != "" {
		return "", fmt.Errorf("部署失败: %s", result.Error)
	}
	return result.DeployID, nil
}

// bridgeWaitDone 轮询等待 bridge 部署完成 (GET /api/deploy/{id}/logs?mode=full)
func (d *Deployer) bridgeWaitDone(bridgeURL, token, deployID string) error {
	client := &http.Client{Timeout: 30 * time.Second}
	url := fmt.Sprintf("%s/api/deploy/%s/logs?mode=full",
		strings.TrimRight(bridgeURL, "/"), deployID)

	lastLogCount := 0

	for i := 0; i < 120; i++ { // 最多轮询 120 次（约 4 分钟）
		time.Sleep(2 * time.Second)

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		if err != nil {
			d.logf("info", "  > 轮询失败，重试: %v\n", err)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		var result struct {
			DeployID string `json:"deploy_id"`
			Status   string `json:"status"`
			Error    string `json:"error"`
			Logs     []struct {
				Time  string `json:"time"`
				Level string `json:"level"`
				Text  string `json:"text"`
			} `json:"logs"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		// 只输出新增日志
		for idx := lastLogCount; idx < len(result.Logs); idx++ {
			log := result.Logs[idx]
			d.logf("info", "  [%s] %s\n", log.Time, log.Text)
		}
		lastLogCount = len(result.Logs)

		switch result.Status {
		case "done":
			return nil
		case "error":
			return fmt.Errorf("%s", result.Error)
		}
		// pending / running → 继续轮询
	}

	return fmt.Errorf("部署超时（轮询超过 4 分钟）")
}

// localUnzip 本地解压 zip 文件
func (d *Deployer) localUnzip(zipPath, targetDir string) error {
	if runtime.GOOS == "windows" {
		// Windows: 优先 7z，回退到 PowerShell Expand-Archive
		if sevenZip, err := exec.LookPath("7z"); err == nil {
			return d.runLocalCmd(sevenZip, []string{"x", "-y", "-o" + targetDir, zipPath}, "")
		}
		psCmd := fmt.Sprintf("Expand-Archive -Force -Path '%s' -DestinationPath '%s'", zipPath, targetDir)
		return d.runLocalCmd("powershell", []string{"-Command", psCmd}, "")
	}
	return d.runLocalCmd("unzip", []string{"-o", zipPath, "-d", targetDir}, "")
}

// runLocalScript 执行本地脚本（自动适配 .bat/.sh）
// 对于 Unix 系统，优先使用 setsid 创建新会话，使脚本中启动的后台进程与 deploy-agent 完全分离
// macOS 没有 setsid，改用 Setpgid 将脚本放入新进程组，避免 deploy-agent 退出时信号传播到子进程
//
// 注意：脚本执行使用 fire-and-forget 模式，stdout/stderr 设为 nil（丢弃），
// 避免 cmd.Run() 因等待 pipe 关闭而卡住（子进程继承 pipe 句柄导致）
func (d *Deployer) runLocalScript(scriptPath, workDir string) error {
	start := time.Now()
	var name string
	var args []string

	if strings.HasSuffix(scriptPath, ".bat") || strings.HasSuffix(scriptPath, ".cmd") {
		name = "cmd"
		args = []string{"/c", "start", "cmd", "/c", scriptPath}
	} else {
		// bash 需要正斜杠路径（Windows 反斜杠会被解释为转义字符）
		bashPath := filepath.ToSlash(scriptPath)
		// 优先使用 setsid（Linux 可用），使发布脚本在新会话中运行
		if setsid, err := exec.LookPath("setsid"); err == nil {
			name = setsid
			args = []string{"bash", bashPath}
		} else {
			// macOS fallback: 用 Setpgid 创建新进程组，防止信号传播
			return d.runLocalCmdDetached("bash", []string{bashPath}, workDir)
		}
	}

	d.logf("info", "  > %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	// stdout/stderr 设为 nil（丢弃）：
	// start/setsid 启动的子进程会继承 pipe 句柄，如果用 buffer 捕获，
	// cmd.Run() 会等待 pipe 关闭，但子进程持有句柄不释放 → 永久卡住
	cmd.Stdout = nil
	cmd.Stderr = nil
	if workDir != "" {
		cmd.Dir = workDir
	}

	err := cmd.Run()
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("%s failed (%.1fs): %v", name, elapsed.Seconds(), err)
	}
	d.logf("info", "  > 完成 (%.1fs)\n", elapsed.Seconds())
	return nil
}

// copyFile 本地文件复制
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// platformToGoEnv 将平台子目录名转换为 Go 交叉编译环境变量
func platformToGoEnv(platform string) (goos, goarch string) {
	switch platform {
	case "linux":
		return "linux", "amd64"
	case "macos":
		return "darwin", "amd64"
	case "win":
		return "windows", "amd64"
	default:
		return platform, "amd64"
	}
}

// inferTargetPlatform 从 target 列表推断目标平台
// 如果所有 target 平台相同，返回该平台；否则返回空串（不交叉编译）
func (d *Deployer) inferTargetPlatform(targets []*Target) string {
	if len(targets) == 0 {
		return d.cfg.HostPlatform
	}
	platform := targets[0].Platform
	for _, t := range targets[1:] {
		if t.Platform != platform {
			return "" // 多平台混合，不设置交叉编译
		}
	}
	if platform == "" {
		return d.cfg.HostPlatform
	}
	return platform
}

// pack 执行本地打包脚本
// targetPlatform: 目标平台，若与 HostPlatform 不同则交叉编译
func (d *Deployer) pack(targetPlatform string) error {
	var name string
	var args []string

	if strings.HasSuffix(d.proj.PackScript, ".bat") || strings.HasSuffix(d.proj.PackScript, ".cmd") {
		name = "cmd"
		args = []string{"/c", d.proj.PackScript}
	} else {
		name = "bash"
		args = []string{d.proj.PackScript}
	}

	// 交叉编译：目标平台 ≠ 主机平台时设置环境变量
	var extraEnv []string
	if targetPlatform != "" && d.cfg.HostPlatform != targetPlatform {
		goos, goarch := platformToGoEnv(targetPlatform)
		extraEnv = []string{"GOOS=" + goos, "GOARCH=" + goarch, "CGO_ENABLED=0"}
		d.logf("info", "  > 交叉编译: GOOS=%s GOARCH=%s\n", goos, goarch)
	}

	if err := d.runLocalCmdWithEnv(name, args, d.proj.ProjectDir, extraEnv); err != nil {
		return err
	}

	// 查找最新 zip 文件
	globPattern := strings.ReplaceAll(d.proj.PackPattern, "{date}", "*")
	matches, err := filepath.Glob(filepath.Join(d.proj.ProjectDir, globPattern))
	if err != nil {
		return fmt.Errorf("glob zip files: %v", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("打包完成但未找到匹配 %q 的文件", globPattern)
	}

	// 按修改时间排序，取最新
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})

	d.packFile = filepath.Base(matches[0])
	return nil
}

// connectSSH 建立 SSH 连接
func (d *Deployer) connectSSH(user, host string, port int) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(d.password),
			ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = d.password
				}
				return answers, nil
			}),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}

	// 如果配置了 SSH Key，优先使用
	if d.cfg.SSHKey != "" {
		keyBytes, err := os.ReadFile(d.cfg.SSHKey)
		if err == nil {
			if signer, err := ssh.ParsePrivateKey(keyBytes); err == nil {
				config.Auth = append([]ssh.AuthMethod{ssh.PublicKeys(signer)}, config.Auth...)
			}
		}
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, err
	}
	d.SSHConnected = true
	return client, nil
}

// upload 通过 SFTP 上传文件
func (d *Deployer) upload(client *ssh.Client, t *Target) error {
	start := time.Now()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		return fmt.Errorf("创建 SFTP 连接失败: %v", err)
	}
	defer sftpClient.Close()

	localPath := filepath.Join(d.proj.ProjectDir, d.packFile)
	remotePath := t.RemoteDir + "/" + d.packFile

	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("打开本地文件失败: %v", err)
	}
	defer localFile.Close()

	localInfo, err := localFile.Stat()
	if err != nil {
		return err
	}

	// 远程目录不存在则自动创建
	if err := sftpClient.MkdirAll(t.RemoteDir); err != nil {
		return fmt.Errorf("创建远程目录失败: %v", err)
	}

	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("创建远程文件失败: %v", err)
	}
	defer remoteFile.Close()

	written, err := io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("上传失败: %v", err)
	}

	elapsed := time.Since(start)
	d.logf("info", "  > 已上传 %s (%s, %.1fs)\n", remotePath, formatSize(localInfo.Size()), elapsed.Seconds())
	_ = written
	return nil
}

// runRemoteCmd 通过 SSH 执行远程命令
func (d *Deployer) runRemoteCmd(client *ssh.Client, cmd string) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH 会话失败: %v", err)
	}
	defer session.Close()

	// 捕获 stdout/stderr 到 buffer，失败时可以返回具体原因
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	d.logf("info", "  > %s\n", cmd)
	start := time.Now()
	err = session.Run(cmd)
	elapsed := time.Since(start)

	// 输出 stdout（如 unzip 文件列表）
	if out := strings.TrimSpace(stdoutBuf.String()); out != "" {
		d.logf("info", "%s\n", out)
	}

	if err != nil {
		errDetail := strings.TrimSpace(stderrBuf.String())
		if errDetail != "" {
			return fmt.Errorf("命令执行失败 (%.1fs): %s", elapsed.Seconds(), errDetail)
		}
		return fmt.Errorf("命令执行失败 (%.1fs): %v", elapsed.Seconds(), err)
	}
	d.logf("info", "  > 完成 (%.1fs)\n", elapsed.Seconds())
	return nil
}

// runPublishCmd 执行远程发布脚本
// 发布脚本可能启动后台服务，其 stdout/stderr 会导致 SSH 会话一直不关闭
// 因此将脚本输出重定向到临时文件，执行后读取输出，确保 SSH 会话能正常结束
func (d *Deployer) runPublishCmd(client *ssh.Client, t *Target) error {
	// 部署前清理旧进程：先按进程名找到 PID 及其监听端口，全部 kill
	// 确保不会出现 "address already in use"
	binaryName := d.proj.Name
	d.logf("info", "  > 清理旧进程 %s...\n", binaryName)
	killSession, err := client.NewSession()
	if err == nil {
		// 1. 按进程名查找 PID 及其监听端口并 kill
		// 2. 如果有显式 ServicePort，额外确保该端口释放
		var portKillExtra string
		if t.ServicePort > 0 {
			portKillExtra = fmt.Sprintf(
				"EXTRA_PID=$(lsof -ti:%d 2>/dev/null || ss -tlnp 2>/dev/null | grep ':%d ' | grep -oP 'pid=\\K\\d+'); "+
					"if [ -n \"$EXTRA_PID\" ]; then echo \"Kill port %d PID: $EXTRA_PID\"; echo \"$EXTRA_PID\" | xargs kill -9 2>/dev/null; fi; ",
				t.ServicePort, t.ServicePort, t.ServicePort,
			)
		}
		killCmd := fmt.Sprintf(
			"OLD_PID=$(pgrep -f '%s' 2>/dev/null | head -5); "+
				"if [ -n \"$OLD_PID\" ]; then "+
				"echo \"Kill old process: $OLD_PID\"; echo \"$OLD_PID\" | xargs kill -9 2>/dev/null; sleep 1; "+
				"else echo \"No old process found\"; fi; %s"+
				"echo \"Cleanup done\"",
			binaryName, portKillExtra,
		)
		var killOut bytes.Buffer
		killSession.Stdout = &killOut
		killSession.Stderr = &killOut
		if runErr := killSession.Run(killCmd); runErr != nil {
			d.logf("info", "  > 清理警告: %v\n", runErr)
		} else {
			d.logf("info", "  > %s\n", strings.TrimSpace(killOut.String()))
		}
		killSession.Close()
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH 会话失败: %v", err)
	}
	defer session.Close()

	// 脚本输出写入临时文件；成功时不显示，失败时 cat 输出帮助排查
	// sed 去除 Windows CRLF 行尾（\r），防止 Windows 打包的 .sh 在 Linux 上乱码无法执行
	// 注：端口占用清理已在 publish.sh 模板中通过 lsof/ss 处理
	tmpLog := "/tmp/deploy_publish_$$.log"
	cmd := fmt.Sprintf(
		"cd %s && sed -i 's/\\r$//' %s && setsid bash %s > %s 2>&1 < /dev/null; ec=$?; if [ $ec -ne 0 ]; then cat %s; fi; rm -f %s; exit $ec",
		t.RemoteDir, t.RemoteScript, t.RemoteScript, tmpLog, tmpLog, tmpLog,
	)

	// 捕获输出（失败时 cat 的内容会出现在 stdout）
	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	d.logf("info", "  > %s\n", t.RemoteScript)
	start := time.Now()
	err = session.Run(cmd)
	elapsed := time.Since(start)

	if err != nil {
		// 优先用 stdout（cat 的脚本输出），其次 stderr
		errDetail := strings.TrimSpace(stdoutBuf.String())
		if errDetail == "" {
			errDetail = strings.TrimSpace(stderrBuf.String())
		}
		if errDetail != "" {
			d.logf("error", "%s\n", errDetail)
			return fmt.Errorf("命令执行失败 (%.1fs): %s", elapsed.Seconds(), errDetail)
		}
		return fmt.Errorf("命令执行失败 (%.1fs): %v", elapsed.Seconds(), err)
	}
	d.logf("info", "  > 完成 (%.1fs)\n", elapsed.Seconds())
	return nil
}

// runLocalCmd 执行本地命令
func (d *Deployer) runLocalCmd(name string, args []string, dir string) error {
	return d.runLocalCmdWithEnv(name, args, dir, nil)
}

// runLocalCmdWithEnv 执行本地命令（支持额外环境变量）
func (d *Deployer) runLocalCmdWithEnv(name string, args []string, dir string, extraEnv []string) error {
	start := time.Now()
	d.logf("info", "  > %s %s\n", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if dir != "" {
		cmd.Dir = dir
	}
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}

	err := cmd.Run()
	elapsed := time.Since(start)

	// 输出 stdout
	if out := strings.TrimSpace(stdoutBuf.String()); out != "" {
		d.logf("info", "%s\n", out)
	}

	if err != nil {
		errDetail := strings.TrimSpace(stderrBuf.String())
		if errDetail != "" {
			d.logf("error", "%s\n", errDetail)
			return fmt.Errorf("%s failed (%.1fs): %s", name, elapsed.Seconds(), errDetail)
		}
		return fmt.Errorf("%s failed (%.1fs): %v", name, elapsed.Seconds(), err)
	}
	d.logf("info", "  > 完成 (%.1fs)\n", elapsed.Seconds())
	return nil
}

// runLocalCmdDetached 在新进程组中执行本地命令
// Unix 系统设置 Setpgid=true 使子进程脱离当前进程组，deploy-agent 退出时不会向其发送信号
// stdout/stderr 设为 nil，避免子进程继承 pipe 句柄导致 cmd.Run() 卡住
func (d *Deployer) runLocalCmdDetached(name string, args []string, dir string) error {
	start := time.Now()
	d.logf("info", "  > %s %s (detached)\n", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	setSysProcAttr(cmd)
	if dir != "" {
		cmd.Dir = dir
	}

	err := cmd.Run()
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("%s failed (%.1fs): %v", name, elapsed.Seconds(), err)
	}
	d.logf("info", "  > 完成 (%.1fs)\n", elapsed.Seconds())
	return nil
}

// parseHost 解析 user@host → (user, host)
func parseHost(hostStr string) (string, string) {
	if i := strings.Index(hostStr, "@"); i >= 0 {
		return hostStr[:i], hostStr[i+1:]
	}
	return "root", hostStr
}

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fGB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fMB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fKB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// formatDuration 格式化耗时
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.0fs", d.Seconds())
}

// ========================= 部署模式与文件保护 =========================

// resolveDeployMode 根据 DeployMode 和首次部署标志确定实际部署模式
func (d *Deployer) resolveDeployMode(isFirstDeploy bool) DeployMode {
	switch d.DeployMode {
	case DeployModeFull:
		return DeployModeFull
	case DeployModeIncr:
		return DeployModeIncr
	default: // auto
		if isFirstDeploy {
			return DeployModeFull
		}
		return DeployModeIncr
	}
}

// logDeployMode 输出部署模式判定日志
func (d *Deployer) logDeployMode(isFirstDeploy bool, effectiveMode DeployMode) {
	modeLabel := map[DeployMode]string{
		DeployModeFull: "完整部署 (full)",
		DeployModeIncr: "增量部署 (increment)",
	}
	label := modeLabel[effectiveMode]

	if d.DeployMode == DeployModeAuto {
		if isFirstDeploy {
			d.logf("info", "  > 部署模式: %s（自动检测: 首次部署，远程无二进制）\n", label)
		} else {
			d.logf("info", "  > 部署模式: %s（自动检测: 远程已有二进制）\n", label)
		}
	} else {
		d.logf("info", "  > 部署模式: %s（手动指定）\n", label)
	}
}

// findExistingProtectedFilesRemote 通过 SSH 检查哪些受保护文件在远程已存在
func (d *Deployer) findExistingProtectedFilesRemote(client *ssh.Client, remoteDir string, protectFiles []string) []string {
	var existing []string
	for _, f := range protectFiles {
		remotePath := remoteDir + "/" + f
		// 目录以 / 结尾，使用 test -d；文件使用 test -f
		var testCmd string
		if strings.HasSuffix(f, "/") {
			testCmd = fmt.Sprintf("test -d %s", strings.TrimRight(remotePath, "/"))
		} else {
			testCmd = fmt.Sprintf("test -f %s", remotePath)
		}
		session, err := client.NewSession()
		if err != nil {
			continue
		}
		err = session.Run(testCmd)
		session.Close()
		if err == nil {
			// 文件/目录存在，需要排除
			if strings.HasSuffix(f, "/") {
				// 目录：排除目录下所有文件
				existing = append(existing, strings.TrimRight(f, "/")+"/*")
			} else {
				existing = append(existing, f)
			}
		}
	}
	return existing
}

// findExistingProtectedFilesLocal 检查哪些受保护文件在本地目标目录已存在
func (d *Deployer) findExistingProtectedFilesLocal(targetDir string, protectFiles []string) []string {
	var existing []string
	for _, f := range protectFiles {
		localPath := filepath.Join(targetDir, f)
		if strings.HasSuffix(f, "/") {
			// 目录：检查是否存在
			if info, err := os.Stat(strings.TrimRight(localPath, "/")); err == nil && info.IsDir() {
				existing = append(existing, strings.TrimRight(f, "/")+"/*")
			}
		} else {
			if _, err := os.Stat(localPath); err == nil {
				existing = append(existing, f)
			}
		}
	}
	return existing
}

// localUnzipWithExcludes 本地解压 zip 文件（带排除列表）
func (d *Deployer) localUnzipWithExcludes(zipPath, targetDir string, excludes []string) error {
	if runtime.GOOS == "windows" {
		// Windows: 优先 7z，回退到 backup-restore 方式
		if sevenZip, err := exec.LookPath("7z"); err == nil {
			args := []string{"x", "-y", "-o" + targetDir, zipPath}
			for _, ex := range excludes {
				args = append(args, "-xr!"+ex)
			}
			return d.runLocalCmd(sevenZip, args, "")
		}
		// PowerShell 回退：备份受保护文件，全量解压，恢复
		return d.localUnzipWindowsBackupRestore(zipPath, targetDir, excludes)
	}
	// Unix: unzip -o archive.zip -d targetDir -x file1 file2
	args := []string{"-o", zipPath, "-d", targetDir}
	args = append(args, "-x")
	args = append(args, excludes...)
	return d.runLocalCmd("unzip", args, "")
}

// localUnzipWindowsBackupRestore Windows PowerShell 回退：备份-解压-恢复
func (d *Deployer) localUnzipWindowsBackupRestore(zipPath, targetDir string, excludes []string) error {
	// 1. 备份受保护文件
	type backup struct {
		src string
		tmp string
	}
	var backups []backup
	for _, ex := range excludes {
		// 跳过通配符模式（目录/*）
		if strings.Contains(ex, "*") {
			continue
		}
		src := filepath.Join(targetDir, ex)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		tmp := src + ".deploy-backup"
		if err := os.Rename(src, tmp); err != nil {
			d.logf("info", "  > 备份文件警告: %v\n", err)
			continue
		}
		backups = append(backups, backup{src: src, tmp: tmp})
	}

	// 2. 全量解压
	err := d.localUnzip(zipPath, targetDir)

	// 3. 恢复备份
	for _, b := range backups {
		if restoreErr := os.Rename(b.tmp, b.src); restoreErr != nil {
			d.logf("info", "  > 恢复文件警告: %v\n", restoreErr)
		}
	}

	return err
}

// ========================= Adhoc 一次性部署 =========================

// AdhocConfig 一次性部署参数（无需预配置 .conf 文件）
type AdhocConfig struct {
	ProjectDir string // Go 项目目录（必填）
	SSHHost    string // SSH 目标（如 root@114.115.214.86）（必填）
	SSHPort    int    // SSH 端口（默认 22）
	RemoteDir  string // 远程部署目录（默认 /data/program/<项目名>）
	StartArgs  string // 启动参数（如 config.json）
	VerifyURL  string // 部署后健康检查 URL（可选）
	ServicePort int   // 服务监听端口（部署前 kill 占用该端口的进程，0 表示不处理）
}

// adhocDeploy 一次性部署：在内存中构建临时配置，复用 Deployer.Run() 完成部署
// onProgress 为 daemon 模式回调（nil 则输出到 stdout）
func adhocDeploy(cfg *DeployConfig, adhoc *AdhocConfig, password string,
	onProgress func(string, string)) error {

	logf := func(level, format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		if onProgress != nil {
			onProgress(level, msg)
		} else {
			fmt.Print(msg)
		}
	}

	// 1. 检测 Go 项目
	absDir, err := filepath.Abs(adhoc.ProjectDir)
	if err != nil {
		return fmt.Errorf("resolve project dir: %v", err)
	}
	_, binName, err := detectGoProject(absDir)
	if err != nil {
		return err
	}
	logf("info", "检测到 Go 项目: %s\n", binName)

	// 2. 扫描额外文件
	extras := scanExtraFiles(absDir, binName)
	if len(extras) > 0 {
		logf("info", "额外文件: %s\n", strings.Join(extras, ", "))
	}

	// 3. 确保打包/发布脚本存在（不覆盖已有文件）
	initCfg := &InitConfig{
		ProjectDir:  absDir,
		ProjectName: binName,
		ExtraFiles:  extras,
		StartArgs:   adhoc.StartArgs,
	}
	if err := ensurePackScripts(initCfg); err != nil {
		return fmt.Errorf("ensure pack scripts: %v", err)
	}

	// 4. 构建内存中的 ProjectConfig + Target
	sshPort := adhoc.SSHPort
	if sshPort == 0 {
		sshPort = 22
	}
	remoteDir := adhoc.RemoteDir
	if remoteDir == "" {
		remoteDir = "/data/program/" + binName
	}

	// 确定服务端口：优先使用显式指定的 ServicePort，其次从 StartArgs 中提取
	servicePort := adhoc.ServicePort
	if servicePort == 0 && adhoc.StartArgs != "" {
		if extracted := extractPortFromArgs(adhoc.StartArgs); extracted != "" {
			fmt.Sscanf(extracted, "%d", &servicePort)
		}
	}

	var packScript string
	if runtime.GOOS == "windows" {
		packScript = filepath.Join(absDir, "zip-files.bat")
	} else {
		packScript = filepath.Join(absDir, "zip-files.sh")
	}

	proj := &ProjectConfig{
		Name:        binName,
		ProjectDir:  absDir,
		PackScript:  packScript,
		PackPattern: binName + "_{date}.zip",
		Targets: []*Target{
			{
				Name:          "adhoc-ssh",
				Host:          adhoc.SSHHost,
				Port:          sshPort,
				RemoteDir:     remoteDir,
				RemoteScript:  "publish.sh",
				Platform:      "linux",
				VerifyURL:     adhoc.VerifyURL,
				VerifyTimeout: 10,
				ServicePort:   servicePort,
			},
		},
	}

	// 5. 使用 Deployer.Run() 执行部署
	deployer := NewDeployer(cfg, proj, password)
	deployer.OnProgress = onProgress
	if err := deployer.Run(false, ""); err != nil {
		return err
	}

	// 6. 部署验证（可选）
	if adhoc.VerifyURL != "" {
		logf("info", "⏳ 等待服务启动 (5s)...\n")
		time.Sleep(5 * time.Second)
		httpClient := &http.Client{Timeout: 10 * time.Second}
		resp, err := httpClient.Get(adhoc.VerifyURL)
		if err != nil {
			return fmt.Errorf("部署验证失败: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("部署验证失败: HTTP %d", resp.StatusCode)
		}
		logf("info", "✅ 部署验证通过（HTTP 200）\n")
	}

	return nil
}

// agentConfigProtectedFiles 用于从目标 agent 配置文件提取 protected_files
type agentConfigProtectedFiles struct {
	ProtectedFiles []string `json:"protected_files"`
}

// mergeAgentProtectedFiles 从目标 agent 的配置文件中读取 protected_files 并合并
// 扫描项目目录中的 JSON 配置文件（{project-name}.json 或其他 JSON 文件），
// 提取 protected_files 字段，合并到 d.proj.ProtectFiles（去重）
func (d *Deployer) mergeAgentProtectedFiles() {
	if d.proj.ProjectDir == "" {
		return
	}

	// 收集所有 JSON 文件中的 protected_files
	var agentProtected []string

	// 优先查找 {project-name}.json
	candidates := []string{d.proj.Name + ".json"}

	// 也扫描项目目录下所有顶层 JSON 文件（去重）
	entries, err := os.ReadDir(d.proj.ProjectDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
				continue
			}
			name := e.Name()
			if name == candidates[0] {
				continue // 已在候选列表中
			}
			candidates = append(candidates, name)
		}
	}

	for _, name := range candidates {
		configPath := filepath.Join(d.proj.ProjectDir, name)
		data, err := os.ReadFile(configPath)
		if err != nil {
			continue
		}
		var acpf agentConfigProtectedFiles
		if err := json.Unmarshal(data, &acpf); err != nil {
			continue
		}
		if len(acpf.ProtectedFiles) > 0 {
			agentProtected = append(agentProtected, acpf.ProtectedFiles...)
			break // 找到第一个含 protected_files 的配置即可
		}
	}

	if len(agentProtected) == 0 {
		return
	}

	// 合并去重
	existing := make(map[string]bool)
	for _, f := range d.proj.ProtectFiles {
		existing[f] = true
	}
	var added []string
	for _, f := range agentProtected {
		if !existing[f] {
			d.proj.ProtectFiles = append(d.proj.ProtectFiles, f)
			existing[f] = true
			added = append(added, f)
		}
	}
	if len(added) > 0 {
		d.logf("info", "  > 从 agent 配置合并 protected_files: %s\n", strings.Join(added, ", "))
	}
}
