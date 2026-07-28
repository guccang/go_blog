package persistence

// MediaAsset is the SQLite metadata for a locally stored editor image.
type MediaAsset struct {
	ID          string
	Account     string
	StorageName string
	MIMEType    string
	SizeBytes   int64
	CreatedAt   string
}

func SaveMediaAsset(asset MediaAsset) error {
	_, err := requireSQLite().Exec(`INSERT INTO media_assets(id,account,storage_name,mime_type,size_bytes,created_at)
		VALUES(?,?,?,?,?,?)`, asset.ID, asset.Account, asset.StorageName, asset.MIMEType, asset.SizeBytes, asset.CreatedAt)
	return err
}

func GetMediaAsset(account, id string) (*MediaAsset, error) {
	asset := &MediaAsset{}
	err := requireSQLite().QueryRow(`SELECT id,account,storage_name,mime_type,size_bytes,created_at
		FROM media_assets WHERE id=? AND account=?`, id, account).Scan(
		&asset.ID, &asset.Account, &asset.StorageName, &asset.MIMEType, &asset.SizeBytes, &asset.CreatedAt)
	if err != nil {
		return nil, err
	}
	return asset, nil
}
