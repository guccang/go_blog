package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PipelineStep 编排中的单个步骤
type PipelineStep struct {
	Project       string `json:"project"`                  // 必填：项目名
	Target        string `json:"target,omitempty"`         // 部署目标覆盖
	BuildPlatform string `json:"build_platform,omitempty"` // 目标平台覆盖
	PackOnly      bool   `json:"pack_only,omitempty"`      // 仅打包
}

// Pipeline 命名编排
type Pipeline struct {
	Name        string         `json:"name"`        // 编排名称（未设置时从文件名推导）
	Description string         `json:"description"` // 描述
	Steps       []PipelineStep `json:"steps"`       // 有序步骤
}

// PipelinesConfig 编排配置（从 pipelines/ 目录加载的全部 pipeline）
type PipelinesConfig struct {
	Pipelines []Pipeline
}

// LoadPipelines 扫描 pipelines 目录，每个 .json 文件加载为一个 Pipeline
// 文件名即为默认 pipeline 名称（如 prod-all.json → "prod-all"），
// 若文件内设置了 name 字段则以文件内为准
func LoadPipelines(dir string) (*PipelinesConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取 pipelines 目录失败: %v", err)
	}

	cfg := &PipelinesConfig{}

	// 按文件名排序，保证顺序稳定
	var jsonFiles []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			jsonFiles = append(jsonFiles, e)
		}
	}
	sort.Slice(jsonFiles, func(i, j int) bool {
		return jsonFiles[i].Name() < jsonFiles[j].Name()
	})

	for _, entry := range jsonFiles {
		filePath := filepath.Join(dir, entry.Name())
		baseName := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))

		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("读取 %s 失败: %v", entry.Name(), err)
		}

		var p Pipeline
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("解析 %s 失败: %v", entry.Name(), err)
		}

		// name 未设置时用文件名
		if p.Name == "" {
			p.Name = baseName
		}

		// 校验
		if len(p.Steps) == 0 {
			return nil, fmt.Errorf("pipeline %q (%s): steps 不能为空", p.Name, entry.Name())
		}
		for j, s := range p.Steps {
			if s.Project == "" {
				return nil, fmt.Errorf("pipeline %q (%s) step[%d]: project 不能为空", p.Name, entry.Name(), j)
			}
		}

		cfg.Pipelines = append(cfg.Pipelines, p)
	}

	return cfg, nil
}

// Get 按名称查找 pipeline
func (cfg *PipelinesConfig) Get(name string) *Pipeline {
	for i := range cfg.Pipelines {
		if cfg.Pipelines[i].Name == name {
			return &cfg.Pipelines[i]
		}
	}
	return nil
}

// Names 列出所有 pipeline 名称
func (cfg *PipelinesConfig) Names() []string {
	names := make([]string, len(cfg.Pipelines))
	for i, p := range cfg.Pipelines {
		names[i] = p.Name
	}
	return names
}

// ValidatePipeline 校验所有 step 的 project 是否存在于 DeployConfig
func ValidatePipeline(p *Pipeline, deployCfg *DeployConfig) error {
	var missing []string
	for idx, step := range p.Steps {
		proj := deployCfg.GetProject(step.Project)
		if proj == nil {
			missing = append(missing, step.Project)
			continue
		}
		if proj.BuildOnly && !step.PackOnly {
			return fmt.Errorf("pipeline %q step[%d] project %q 仅支持 pack_only", p.Name, idx, step.Project)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("pipeline %q 引用了不存在的项目: %s (可用: %v)",
			p.Name, strings.Join(missing, ", "), deployCfg.ProjectNames())
	}
	return nil
}
