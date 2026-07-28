package persistence

import (
	"database/sql"
	"module"
	"strings"
)

const (
	chunkMaxRunes     = 600
	chunkOverlapRunes = 80
)

type BlogChunkSearchResult struct {
	Title   string
	Heading string
	Content string
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
	terms := strings.Fields(query)
	if account == "" || len(terms) == 0 {
		return []BlogChunkSearchResult{}, nil
	}
	quoted := make([]string, 0, len(terms))
	for _, term := range terms {
		quoted = append(quoted, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	rows, err := requireSQLite().Query(`SELECT f.blog_title,f.heading,f.content
		FROM blog_chunks_fts f JOIN blogs b ON b.account=f.account AND b.title=f.blog_title
		WHERE f.account=? AND blog_chunks_fts MATCH ? AND b.encrypt=0 AND (b.auth_type & ?) = 0
		ORDER BY bm25(blog_chunks_fts) LIMIT ?`, account, strings.Join(quoted, " AND "), module.EAuthType_diary, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []BlogChunkSearchResult{}
	for rows.Next() {
		var item BlogChunkSearchResult
		if err := rows.Scan(&item.Title, &item.Heading, &item.Content); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
