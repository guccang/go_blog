package persistence

type PIUsageRecord struct {
	Provider         string `json:"provider"`
	Model            string `json:"model"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	DurationMs       int64  `json:"duration_ms"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
}

type PIUsageStats struct {
	Requests         int   `json:"requests"`
	PromptTokens     int   `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	TotalTokens      int   `json:"total_tokens"`
	DurationMs       int64 `json:"duration_ms"`
}

func RecordPIUsage(account, provider, model string, promptTokens, completionTokens, totalTokens int, durationMs int64, status string) error {
	_, err := requireSQLite().Exec(`INSERT INTO pi_usage(account,provider,model,prompt_tokens,completion_tokens,total_tokens,duration_ms,status,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, account, provider, model, promptTokens, completionTokens, totalTokens, durationMs, status, sqliteNow())
	return err
}

func GetPIUsage(account string, limit int) (PIUsageStats, []PIUsageRecord, error) {
	if limit < 1 || limit > 100 {
		limit = 30
	}
	stats := PIUsageStats{}
	err := requireSQLite().QueryRow(`SELECT COUNT(*),COALESCE(SUM(prompt_tokens),0),COALESCE(SUM(completion_tokens),0),COALESCE(SUM(total_tokens),0),COALESCE(SUM(duration_ms),0) FROM pi_usage WHERE account=?`, account).Scan(&stats.Requests, &stats.PromptTokens, &stats.CompletionTokens, &stats.TotalTokens, &stats.DurationMs)
	if err != nil {
		return stats, nil, err
	}
	rows, err := requireSQLite().Query(`SELECT provider,model,prompt_tokens,completion_tokens,total_tokens,duration_ms,status,created_at FROM pi_usage WHERE account=? ORDER BY id DESC LIMIT ?`, account, limit)
	if err != nil {
		return stats, nil, err
	}
	defer rows.Close()
	records := make([]PIUsageRecord, 0)
	for rows.Next() {
		var item PIUsageRecord
		if err := rows.Scan(&item.Provider, &item.Model, &item.PromptTokens, &item.CompletionTokens, &item.TotalTokens, &item.DurationMs, &item.Status, &item.CreatedAt); err != nil {
			return stats, nil, err
		}
		records = append(records, item)
	}
	return stats, records, rows.Err()
}
