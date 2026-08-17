package persistence

import (
	"encoding/json"
	"strings"
)

// ClipboardItem is one account-scoped text and image transfer record.
type ClipboardItem struct {
	ID        string   `json:"id"`
	Account   string   `json:"-"`
	Text      string   `json:"text"`
	ImageIDs  []string `json:"image_ids"`
	CreatedAt string   `json:"created_at"`
}

func SaveClipboardItem(item ClipboardItem) error {
	images, err := json.Marshal(item.ImageIDs)
	if err != nil {
		return err
	}
	_, err = requireSQLite().Exec(`INSERT INTO clipboard_items(id,account,text_content,image_ids_json,created_at)
		VALUES(?,?,?,?,?)`, item.ID, item.Account, item.Text, string(images), item.CreatedAt)
	return err
}

func ListClipboardItems(account string, limit int) ([]ClipboardItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := requireSQLite().Query(`SELECT id,account,text_content,image_ids_json,created_at
		FROM clipboard_items WHERE account=? ORDER BY created_at DESC,id DESC LIMIT ?`, account, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ClipboardItem, 0)
	for rows.Next() {
		var item ClipboardItem
		var images string
		if err := rows.Scan(&item.ID, &item.Account, &item.Text, &images, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(images), &item.ImageIDs); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func DeleteClipboardItem(account, id string) (bool, error) {
	result, err := requireSQLite().Exec(`DELETE FROM clipboard_items WHERE account=? AND id=?`, account, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}
