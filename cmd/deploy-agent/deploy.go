package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Deployer 部署编排器
type Deployer struct {
	cfg          *DeployConfig
	password     string
	packFile     string // 打包后的 zip 文件名（不含路径）
	SSHConnected bool   // SSH 连接是否成功过（密码有效）
}

// NewDeployer 创建部署器
func NewDeployer(cfg *DeployConfig, password string) *Deployer {
	return &Deployer{cfg: cfg, password: password}
}

// Run 执行部署 pipeline
func (d *Deployer) Run(packOnly bool, targetFilter string) error {
	start := time.Now()

	targets := d.cfg.Targets
	if targetFilter != "" {
		targets = nil
		for _, t := range d.cfg.Targets {
			if t.Name == targetFilter || t.Host == targetFilter {
				targets = []*Target{t}
				break
			}
		}
		if len(targets) == 0 {
			return fmt.Errorf("target %q not found", targetFilter)
		}
	}

	totalSteps := 4
	if packOnly {
		totalSteps = 1
	}

	// Step 1: 打包
	fmt.Printf("[STEP 1/%d] 打包项目...\n", totalSteps)
	if err := d.pack(); err != nil {
		return fmt.Errorf("打包失败: %v", err)
	}
	packPath := filepath.Join(d.cfg.ProjectDir, d.packFile)
	info, err := os.Stat(packPath)
	if err != nil {
		return fmt.Errorf("找不到打包文件: %v", err)
	}
	fmt.Printf("[STEP 1/%d] 打包完成: %s (%s)\n", totalSteps, d.packFile, formatSize(info.Size()))

	if packOnly {
		fmt.Printf("[DONE] 打包完成，耗时 %s\n", formatDuration(time.Since(start)))
		return nil
	}

	// Step 2-4: 逐个目标部署
	for _, t := range targets {
		label := t.Host
		if t.Name != "" && !strings.HasPrefix(t.Name, "default-") {
			label = fmt.Sprintf("%s (%s)", t.Name, t.Host)
		}

		// 建立 SSH 连接
		user, host := parseHost(t.Host)
		fmt.Printf("[STEP 2/%d] 连接 %s...\n", totalSteps, label)
		client, err := d.connectSSH(user, host, t.Port)
		if err != nil {
			fmt.Printf("[ERROR] 连接 %s 失败: %v\n", label, err)
			continue
		}

		// Step 2: 上传
		fmt.Printf("[STEP 2/%d] 上传到 %s:%s...\n", totalSteps, t.Host, t.RemoteDir)
		if err := d.upload(client, t); err != nil {
			client.Close()
			fmt.Printf("[ERROR] 上传到 %s 失败: %v\n", label, err)
			continue
		}

		// Step 3: 解压
		fmt.Printf("[STEP 3/%d] 解压到 %s:%s...\n", totalSteps, t.Host, t.RemoteDir)
		cmd := fmt.Sprintf("cd %s && unzip -o %s", t.RemoteDir, d.packFile)
		if err := d.runRemoteCmd(client, cmd); err != nil {
			client.Close()
			fmt.Printf("[ERROR] 解压到 %s 失败: %v\n", label, err)
			continue
		}

		// Step 4: 发布
		if t.RemoteScript != "" {
			fmt.Printf("[STEP 4/%d] 执行 %s on %s...\n", totalSteps, t.RemoteScript, label)
			if err := d.runPublishCmd(client, t); err != nil {
				client.Close()
				fmt.Printf("[ERROR] 执行 %s on %s 失败: %v\n", t.RemoteScript, label, err)
				continue
			}
		} else {
			fmt.Printf("[STEP 4/%d] 无发布脚本，跳过\n", totalSteps)
		}

		client.Close()
		fmt.Printf("[OK] %s 部署成功\n", label)
	}

	fmt.Printf("[DONE] 部署完成，耗时 %s\n", formatDuration(time.Since(start)))
	return nil
}

// pack 执行本地打包脚本
func (d *Deployer) pack() error {
	var name string
	var args []string

	if strings.HasSuffix(d.cfg.PackScript, ".bat") || strings.HasSuffix(d.cfg.PackScript, ".cmd") {
		name = "cmd"
		args = []string{"/c", d.cfg.PackScript}
	} else {
		name = "bash"
		args = []string{d.cfg.PackScript}
	}

	if err := d.runLocalCmd(name, args, d.cfg.ProjectDir); err != nil {
		return err
	}

	// 查找最新 zip 文件
	globPattern := strings.ReplaceAll(d.cfg.PackPattern, "{date}", "*")
	matches, err := filepath.Glob(filepath.Join(d.cfg.ProjectDir, globPattern))
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

	localPath := filepath.Join(d.cfg.ProjectDir, d.packFile)
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
	fmt.Printf("  > 已上传 %s (%s, %.1fs)\n", remotePath, formatSize(localInfo.Size()), elapsed.Seconds())
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

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	fmt.Printf("  > %s\n", cmd)
	start := time.Now()
	err = session.Run(cmd)
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("命令执行失败 (%.1fs): %v", elapsed.Seconds(), err)
	}
	fmt.Printf("  > 完成 (%.1fs)\n", elapsed.Seconds())
	return nil
}

// runPublishCmd 执行远程发布脚本
// 发布脚本可能启动后台服务，其 stdout/stderr 会导致 SSH 会话一直不关闭
// 因此将脚本输出重定向到临时文件，执行后读取输出，确保 SSH 会话能正常结束
func (d *Deployer) runPublishCmd(client *ssh.Client, t *Target) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("创建 SSH 会话失败: %v", err)
	}
	defer session.Close()

	// 脚本输出写入临时文件；成功时不显示，失败时 cat 输出帮助排查
	tmpLog := "/tmp/deploy_publish_$$.log"
	cmd := fmt.Sprintf(
		"cd %s && setsid bash %s > %s 2>&1 < /dev/null; ec=$?; if [ $ec -ne 0 ]; then cat %s; fi; rm -f %s; exit $ec",
		t.RemoteDir, t.RemoteScript, tmpLog, tmpLog, tmpLog,
	)

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	fmt.Printf("  > %s\n", t.RemoteScript)
	start := time.Now()
	err = session.Run(cmd)
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("命令执行失败 (%.1fs): %v", elapsed.Seconds(), err)
	}
	fmt.Printf("  > 完成 (%.1fs)\n", elapsed.Seconds())
	return nil
}

// runLocalCmd 执行本地命令
func (d *Deployer) runLocalCmd(name string, args []string, dir string) error {
	start := time.Now()
	fmt.Printf("  > %s %s\n", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if dir != "" {
		cmd.Dir = dir
	}

	err := cmd.Run()
	elapsed := time.Since(start)
	if err != nil {
		return fmt.Errorf("%s failed (%.1fs): %v", name, elapsed.Seconds(), err)
	}
	fmt.Printf("  > 完成 (%.1fs)\n", elapsed.Seconds())
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
