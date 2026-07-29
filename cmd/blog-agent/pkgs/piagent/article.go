package piagent

import (
	"blog"
	"errors"
	"fmt"
	"module"
	"persistence"
	"strings"
	"time"
)

const (
	maxArticleContextRunes = 9000
	maxRelatedContextRunes = 4500
)

var articleActionInstructions = map[string]string{
	"summary":    "用不超过500字总结文章的核心主题、主要结论和必要限制。先给一句总览，再分点说明。",
	"key_points": "提取5到8条最值得保留的关键结论。每条必须来自文章，不要补写文章没有表达的事实。",
	"related":    "说明当前文章与站内关联资料之间的联系、补充和差异。没有足够关联资料时直接说明。",
	"next_steps": "根据文章已有内容给出1到5条具体、低风险的下一步建议。使用“可以考虑”等措辞，区分原文结论与模型建议。",
	"question":   "直接回答用户针对当前文章提出的问题。优先依据当前文章，资料不足时明确说明；关联资料只能作为补充。",
}

// AskArticle answers against the article being read and only retrieves other blogs as supplements.
func AskArticle(account, title, action, question, provider string) (Answer, error) {
	startedAt := time.Now()
	instruction, ok := articleActionInstructions[strings.TrimSpace(action)]
	if !ok {
		return Answer{}, errors.New("unsupported article action")
	}
	question = strings.TrimSpace(question)
	if action == "question" && question == "" {
		return Answer{}, errors.New("question is required")
	}

	article := blog.GetBlogWithAccount(account, title)
	if article == nil {
		return Answer{}, errors.New("article not found")
	}
	if article.Encrypt != 0 || article.AuthType&(module.EAuthType_encrypt|module.EAuthType_diary) != 0 {
		return Answer{}, errors.New("PI reading assistant is disabled for encrypted articles and diaries")
	}
	cfg, err := loadProvider(account, provider)
	if err != nil {
		return Answer{}, err
	}

	articleContext := selectArticleContext(article.Content)
	relatedContext := ""
	sources := []string{}
	if action == "related" || action == "question" {
		results, searchErr := blog.SearchChunksWithAccount(account, strings.TrimSpace(article.Title+" "+question), 18)
		if searchErr != nil {
			return Answer{}, fmt.Errorf("retrieve related blogs: %w", searchErr)
		}
		relatedContext, sources = buildRelatedArticleContext(article.Title, results)
	}

	prompt := "你是当前文章的阅读助手。回答必须以当前文章为主要依据，清楚区分原文内容、关联资料和你的建议。不要输出参考标题列表，页面会单独展示可点击来源。\n\n"
	prompt += "任务：" + instruction + "\n\n当前文章标题：" + article.Title + "\n\n当前文章内容：\n" + articleContext
	if question != "" {
		prompt += "\n\n用户问题：" + question
	}
	if relatedContext != "" {
		prompt += "\n\n站内关联资料：\n" + relatedContext
	}

	result, err := chat(cfg, prompt)
	if err != nil {
		return Answer{}, err
	}
	return Answer{Text: result.Content, Provider: cfg.Name, Model: cfg.Model, Sources: sources, Usage: result.Usage, DurationMs: time.Since(startedAt).Milliseconds()}, nil
}

func selectArticleContext(content string) string {
	runes := []rune(strings.TrimSpace(content))
	if len(runes) <= maxArticleContextRunes {
		return string(runes)
	}
	middleStart := len(runes)/2 - 1250
	lastStart := len(runes) - 3000
	return string(runes[:3500]) + "\n\n[文章中段节选]\n" + string(runes[middleStart:middleStart+2500]) + "\n\n[文章末段节选]\n" + string(runes[lastStart:])
}

func buildRelatedArticleContext(currentTitle string, results []persistence.BlogChunkSearchResult) (string, []string) {
	var context strings.Builder
	sources := make([]string, 0, 6)
	seen := map[string]struct{}{currentTitle: {}}
	remaining := maxRelatedContextRunes
	for _, result := range results {
		if _, exists := seen[result.Title]; exists {
			continue
		}
		contentRunes := []rune(strings.TrimSpace(result.Content))
		if len(contentRunes) > 900 {
			contentRunes = contentRunes[:900]
		}
		if len(contentRunes) > remaining {
			contentRunes = contentRunes[:remaining]
		}
		context.WriteString("## " + result.Title + "\n")
		if result.Heading != "" {
			context.WriteString("[章节：" + result.Heading + "]\n")
		}
		context.WriteString(string(contentRunes) + "\n\n")
		sources = append(sources, result.Title)
		seen[result.Title] = struct{}{}
		remaining -= len(contentRunes)
		if remaining <= 0 || len(sources) >= 6 {
			break
		}
	}
	return context.String(), sources
}
