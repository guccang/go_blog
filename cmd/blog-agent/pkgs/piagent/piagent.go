// Package piagent provides the small, retrieval-grounded Personal Intelligence agent.
package piagent

import (
	"blog"
	"bytes"
	"config"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	stdhttp "net/http"
	"persistence"
	"strings"
	"time"
)

const maxSourceChars = 6000

type Answer struct {
	Brief    string   `json:"brief"`
	Text     string   `json:"text"`
	Advice   string   `json:"advice"`
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Sources  []string `json:"sources"`
}

type providerConfig struct {
	Name   string `json:"name"`
	APIKey string `json:"api_key"`
	URL    string `json:"api_url"`
	Model  string `json:"model"`
}

type providersConfig struct {
	Default   string           `json:"default"`
	Providers []providerConfig `json:"providers"`
}

func Ask(account, question, provider string) (Answer, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return Answer{}, errors.New("question is required")
	}
	cfg, err := loadProvider(account, provider)
	if err != nil {
		return Answer{}, err
	}
	brief, err := chat(cfg, "请针对下面的问题给出不超过300字的初步回答。不要假装已检索个人博客，不要列来源。\n\n问题："+question)
	if err != nil {
		return Answer{}, err
	}
	brief = limitRunes(brief, 300)
	results, err := blog.SearchChunksWithAccount(account, question, 24)
	if err != nil {
		return Answer{}, fmt.Errorf("retrieve blogs: %w", err)
	}
	context, sources := buildContext(account, results)
	if context == "" {
		return Answer{Brief: brief, Text: "站内没有找到与该问题相关的博客内容。", Provider: cfg.Name, Model: cfg.Model}, nil
	}
	prompt := "你是个人博客助手。请基于站内召回资料，对用户问题给出总结回复。资料不足时明确说明；不要编造。使用中文，简洁回答。不要输出参考、来源或博客标题列表，页面会单独展示可点击来源。\n\n用户问题：" + question + "\n\n初步回答（仅供你校正或补充）：\n" + brief + "\n\n站内资料：\n" + context
	text, err := chat(cfg, prompt)
	if err != nil {
		return Answer{}, err
	}
	advice, err := chat(cfg, "请根据用户问题、站内资料总结和资料标题，探索用户可能的下一步意图，并给出1到3条具体、低风险且完整的建议。必须使用“可能”“可以考虑”等措辞，不要把推断说成事实。使用中文，保持简洁，不要列参考来源。\n\n问题："+question+"\n\n站内资料总结：\n"+text+"\n\n资料标题："+strings.Join(sources, "、"))
	if err != nil {
		return Answer{}, err
	}
	return Answer{Brief: brief, Text: text, Advice: advice, Provider: cfg.Name, Model: cfg.Model, Sources: sources}, nil
}

func limitRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

func loadProvider(account, requested string) (providerConfig, error) {
	var providers providersConfig
	raw := strings.TrimSpace(config.GetConfigWithAccount(account, "pi_providers"))
	if err := json.Unmarshal([]byte(raw), &providers); err != nil {
		return providerConfig{}, errors.New("pi_providers must be valid JSON")
	}
	name := strings.TrimSpace(requested)
	if name == "" {
		name = strings.TrimSpace(providers.Default)
	}
	if name == "" {
		return providerConfig{}, errors.New("PI Agent is not configured: set default and providers in pi_providers JSON")
	}
	for _, provider := range providers.Providers {
		if provider.Name != name {
			continue
		}
		if provider.APIKey == "" || provider.URL == "" || provider.Model == "" {
			return providerConfig{}, fmt.Errorf("provider %q requires api_key, api_url and model", name)
		}
		return provider, nil
	}
	return providerConfig{}, fmt.Errorf("provider %q is not enabled", name)
}

func buildContext(account string, results []persistence.BlogChunkSearchResult) (string, []string) {
	var context strings.Builder
	sources := make([]string, 0, len(results))
	remaining := maxSourceChars
	perBlog := map[string]int{}
	for _, result := range results {
		if perBlog[result.Title] >= 2 {
			continue
		}
		content := result.Content
		if len(content) > remaining {
			content = content[:remaining]
		}
		context.WriteString("## " + result.Title + "\n")
		if result.Heading != "" {
			context.WriteString("[章节：" + result.Heading + "]\n")
		}
		context.WriteString(content + "\n\n")
		perBlog[result.Title]++
		if perBlog[result.Title] == 1 {
			sources = append(sources, result.Title)
		}
		remaining -= len(content)
		if remaining <= 0 {
			break
		}
	}
	return context.String(), sources
}

func chat(cfg providerConfig, prompt string) (string, error) {
	body, err := json.Marshal(map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": "你是一个可靠、克制的个人知识库助手。"},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.2,
	})
	if err != nil {
		return "", err
	}
	req, err := stdhttp.NewRequest(stdhttp.MethodPost, cfg.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	client := &stdhttp.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call provider %q: %w", cfg.Name, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("provider %q returned %s", cfg.Name, resp.Status)
	}
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return "", fmt.Errorf("decode provider response: %w", err)
	}
	if len(response.Choices) == 0 || strings.TrimSpace(response.Choices[0].Message.Content) == "" {
		return "", errors.New("provider returned an empty answer")
	}
	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}
