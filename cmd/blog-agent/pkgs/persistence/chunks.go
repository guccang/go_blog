package persistence

import (
	"database/sql"
	"module"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

const (
	chunkMaxRunes     = 600
	chunkOverlapRunes = 80
)

type BlogChunkSearchResult struct {
	Title      string
	ChunkIndex int
	Heading    string
	Content    string
}

func rebuildBlogChunksTx(tx *sql.Tx, account, title, content string) error {
	if _, err := tx.Exec("DELETE FROM blog_chunks_fts WHERE account=? AND blog_title=?", account, title); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM blog_chunks WHERE account=? AND blog_title=?", account, title); err != nil {
		return err
	}
	for index, chunk := range splitMarkdownChunks(title, content) {
		if _, err := tx.Exec("INSERT INTO blog_chunks(account,blog_title,chunk_index,heading,content) VALUES(?,?,?,?,?)", account, title, index, chunk.heading, chunk.content); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO blog_chunks_fts(account,blog_title,chunk_index,heading,content) VALUES(?,?,?,?,?)", account, title, index, chunk.heading, chunk.content); err != nil {
			return err
		}
	}
	return nil
}

type markdownChunk struct{ heading, content string }

func splitMarkdownChunks(title, content string) []markdownChunk {
	sections := []markdownChunk{}
	heading := title
	var body strings.Builder
	flush := func() {
		text := strings.TrimSpace(body.String())
		if text == "" {
			return
		}
		sections = append(sections, markdownChunk{heading: heading, content: text})
		body.Reset()
	}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			name := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			if name != "" {
				flush()
				heading = name
				continue
			}
		}
		body.WriteString(line)
		body.WriteByte('\n')
	}
	flush()
	if len(sections) == 0 {
		return []markdownChunk{{heading: title, content: title}}
	}
	chunks := []markdownChunk{}
	for _, section := range sections {
		chunks = append(chunks, splitSection(section)...)
	}
	return chunks
}

func splitSection(section markdownChunk) []markdownChunk {
	runes := []rune(section.content)
	if len(runes) <= chunkMaxRunes {
		return []markdownChunk{section}
	}
	chunks := []markdownChunk{}
	for start := 0; start < len(runes); {
		end := start + chunkMaxRunes
		if end >= len(runes) {
			end = len(runes)
		} else {
			for index := end; index > start+chunkMaxRunes/2; index-- {
				if runes[index-1] == '\n' {
					end = index
					break
				}
			}
		}
		chunks = append(chunks, markdownChunk{heading: section.heading, content: string(runes[start:end])})
		if end == len(runes) {
			break
		}
		start = end - chunkOverlapRunes
		if start < 0 {
			start = 0
		}
	}
	return chunks
}

// EnsureBlogChunks builds missing derived chunks without blocking startup.
func EnsureBlogChunks() error {
	rows, err := requireSQLite().Query(`SELECT b.account,b.title,b.content FROM blogs b
		WHERE NOT EXISTS (SELECT 1 FROM blog_chunks c WHERE c.account=b.account AND c.blog_title=b.title)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []struct{ account, title, content string }{}
	for rows.Next() {
		item := struct{ account, title, content string }{}
		if err := rows.Scan(&item.account, &item.title, &item.content); err != nil {
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range items {
		tx, err := requireSQLite().Begin()
		if err != nil {
			return err
		}
		if err := rebuildBlogChunksTx(tx, item.account, item.title, item.content); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func SearchBlogChunks(account, query string, limit int) ([]BlogChunkSearchResult, error) {
	terms := chunkSearchTerms(query)
	if account == "" || len(terms) == 0 {
		return []BlogChunkSearchResult{}, nil
	}
	if limit <= 0 {
		limit = 24
	}
	if limit > 100 {
		limit = 100
	}
	candidateLimit := limit * 8
	if candidateLimit > 400 {
		candidateLimit = 400
	}

	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	rows, err := requireSQLite().Query(`SELECT f.blog_title,f.chunk_index,f.heading,f.content
		FROM blog_chunks_fts f JOIN blogs b ON b.account=f.account AND b.title=f.blog_title
		WHERE f.account=? AND blog_chunks_fts MATCH ? AND b.encrypt=0 AND (b.auth_type & ?) = 0
		ORDER BY bm25(blog_chunks_fts) LIMIT ?`, account, strings.Join(quoted, " OR "), module.EAuthType_diary, candidateLimit)
	if err != nil {
		return nil, err
	}
	candidates := make([]chunkSearchCandidate, 0, candidateLimit)
	seen := map[string]struct{}{}
	for rows.Next() {
		var item BlogChunkSearchResult
		if err := rows.Scan(&item.Title, &item.ChunkIndex, &item.Heading, &item.Content); err != nil {
			rows.Close()
			return nil, err
		}
		key := chunkSearchKey(item)
		seen[key] = struct{}{}
		candidates = append(candidates, chunkSearchCandidate{item: item, fts: true, order: len(candidates)})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	conditions := make([]string, 0, len(terms))
	args := make([]any, 0, 3+len(terms)*3)
	args = append(args, account, module.EAuthType_diary)
	for _, term := range terms {
		conditions = append(conditions, `(instr(lower(c.blog_title),lower(?))>0 OR instr(lower(c.heading),lower(?))>0 OR instr(lower(c.content),lower(?))>0)`)
		args = append(args, term, term, term)
	}
	args = append(args, candidateLimit)
	fallbackRows, err := requireSQLite().Query(`SELECT c.blog_title,c.chunk_index,c.heading,c.content
		FROM blog_chunks c JOIN blogs b ON b.account=c.account AND b.title=c.blog_title
		WHERE c.account=? AND b.encrypt=0 AND (b.auth_type & ?) = 0
		  AND (`+strings.Join(conditions, " OR ")+`)
		ORDER BY b.modify_time DESC,c.blog_title,c.chunk_index LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	for fallbackRows.Next() {
		var item BlogChunkSearchResult
		if err := fallbackRows.Scan(&item.Title, &item.ChunkIndex, &item.Heading, &item.Content); err != nil {
			fallbackRows.Close()
			return nil, err
		}
		key := chunkSearchKey(item)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		candidates = append(candidates, chunkSearchCandidate{item: item, order: len(candidates)})
	}
	if err := fallbackRows.Err(); err != nil {
		fallbackRows.Close()
		return nil, err
	}
	if err := fallbackRows.Close(); err != nil {
		return nil, err
	}
	return rankChunkSearchCandidates(query, terms, candidates, limit), nil
}

type chunkSearchCandidate struct {
	item  BlogChunkSearchResult
	fts   bool
	order int
	score int
}

func chunkSearchKey(item BlogChunkSearchResult) string {
	return item.Title + "\x00" + strconv.Itoa(item.ChunkIndex)
}

func rankChunkSearchCandidates(query string, terms []string, candidates []chunkSearchCandidate, limit int) []BlogChunkSearchResult {
	for index := range candidates {
		candidates[index].score = chunkSearchScore(query, terms, candidates[index].item)
		if candidates[index].fts {
			candidates[index].score += 8
		}
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].score != candidates[right].score {
			return candidates[left].score > candidates[right].score
		}
		return candidates[left].order < candidates[right].order
	})

	result := make([]BlogChunkSearchResult, 0, limit)
	perBlog := map[string]int{}
	for _, candidate := range candidates {
		if perBlog[candidate.item.Title] >= 2 {
			continue
		}
		result = append(result, candidate.item)
		perBlog[candidate.item.Title]++
		if len(result) >= limit {
			break
		}
	}
	return result
}

func chunkSearchScore(query string, terms []string, item BlogChunkSearchResult) int {
	title := strings.ToLower(item.Title)
	heading := strings.ToLower(item.Heading)
	content := strings.ToLower(item.Content)
	fullQuery := strings.ToLower(strings.TrimSpace(query))
	score := 0
	if fullQuery != "" {
		if strings.Contains(title, fullQuery) {
			score += 40
		}
		if strings.Contains(heading, fullQuery) {
			score += 30
		}
		if strings.Contains(content, fullQuery) {
			score += 20
		}
	}
	for _, term := range terms {
		matched := false
		if strings.Contains(title, term) {
			score += 16
			matched = true
		}
		if strings.Contains(heading, term) {
			score += 10
			matched = true
		}
		if count := strings.Count(content, term); count > 0 {
			if count > 4 {
				count = 4
			}
			score += count * 2
			matched = true
		}
		if matched {
			score += 6
		}
	}
	return score
}

func chunkSearchTerms(query string) []string {
	const chineseQuestionRunes = "的吗呢啊"
	seen := map[string]struct{}{}
	terms := make([]string, 0, 16)
	add := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			return
		}
		if _, exists := seen[term]; exists {
			return
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	addBlock := func(block []rune, chinese bool) {
		if chinese {
			text := string(block)
			for _, prefix := range []string{"请问", "关于", "有关"} {
				text = strings.TrimPrefix(text, prefix)
			}
			for _, suffix := range []string{"是什么", "怎么样", "有哪些", "怎么做", "如何", "为何"} {
				text = strings.TrimSuffix(text, suffix)
			}
			block = []rune(text)
		}
		if len(block) == 0 {
			return
		}
		add(string(block))
		if chinese && len(block) > 2 {
			for index := 0; index+2 <= len(block) && len(terms) < 16; index += 2 {
				add(string(block[index : index+2]))
			}
		}
	}

	var block []rune
	blockIsChinese := false
	flush := func() {
		addBlock(block, blockIsChinese)
		block = nil
	}
	for _, current := range []rune(query) {
		isChinese := unicode.Is(unicode.Han, current)
		if isChinese && strings.ContainsRune(chineseQuestionRunes, current) {
			flush()
			continue
		}
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			flush()
			continue
		}
		if len(block) > 0 && isChinese != blockIsChinese {
			flush()
		}
		blockIsChinese = isChinese
		block = append(block, current)
	}
	flush()
	return terms
}
