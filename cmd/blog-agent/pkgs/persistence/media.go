package persistence

import "module"

// MediaAsset is the SQLite metadata for a locally stored editor attachment.
type MediaAsset struct {
	ID           string
	Account      string
	StorageName  string
	OriginalName string
	MIMEType     string
	SizeBytes    int64
	CreatedAt    string
}

func SaveMediaAsset(asset MediaAsset) error {
	_, err := requireSQLite().Exec(`INSERT INTO media_assets(id,account,storage_name,original_name,mime_type,size_bytes,created_at)
		VALUES(?,?,?,?,?,?,?)`, asset.ID, asset.Account, asset.StorageName, asset.OriginalName, asset.MIMEType, asset.SizeBytes, asset.CreatedAt)
	return err
}

func GetMediaAsset(account, id string) (*MediaAsset, error) {
	asset := &MediaAsset{}
	err := requireSQLite().QueryRow(`SELECT id,account,storage_name,original_name,mime_type,size_bytes,created_at
		FROM media_assets WHERE id=? AND account=?`, id, account).Scan(
		&asset.ID, &asset.Account, &asset.StorageName, &asset.OriginalName, &asset.MIMEType, &asset.SizeBytes, &asset.CreatedAt)
	if err != nil {
		return nil, err
	}
	return asset, nil
}

// GetPublicBlogMediaAsset returns an asset only when a safe public blog owned by
// the same account references its media URL. Private clipboard/editor uploads
// therefore remain inaccessible without a session.
func GetPublicBlogMediaAsset(id string) (*MediaAsset, error) {
	asset := &MediaAsset{}
	err := requireSQLite().QueryRow(`SELECT DISTINCT m.id,m.account,m.storage_name,m.original_name,m.mime_type,m.size_bytes,m.created_at
		FROM media_assets m JOIN blogs b ON b.account=m.account
		WHERE m.id=? AND b.encrypt=0 AND (b.auth_type & ?) != 0 AND (b.auth_type & ?) = 0
		AND instr(b.content,'/media/' || m.id)>0 LIMIT 1`,
		id, module.EAuthType_public, module.EAuthType_diary|module.EAuthType_encrypt).Scan(
		&asset.ID, &asset.Account, &asset.StorageName, &asset.OriginalName, &asset.MIMEType, &asset.SizeBytes, &asset.CreatedAt)
	if err != nil {
		return nil, err
	}
	return asset, nil
}
