package persistence

import "strings"

const (
	ProductScanQueued    = "queued"
	ProductScanRunning   = "running"
	ProductScanSucceeded = "succeeded"
	ProductScanFailed    = "failed"
)

// ProductScanJob 记录后台扫描状态，页面刷新和服务重启后仍可恢复。
type ProductScanJob struct {
	ID           string `json:"id"`
	Account      string `json:"-"`
	SourceURL    string `json:"source_url"`
	Provider     string `json:"provider"`
	Status       string `json:"status"`
	ProductID    string `json:"product_id"`
	ErrorMessage string `json:"error_message"`
	CreatedAt    string `json:"created_at"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
}

const productScanJobSelect = `SELECT id,account,source_url,provider,status,product_id,error_message,
	created_at,started_at,finished_at FROM product_scan_jobs`

func scanProductScanJob(scanner interface{ Scan(...any) error }) (ProductScanJob, error) {
	var job ProductScanJob
	err := scanner.Scan(&job.ID, &job.Account, &job.SourceURL, &job.Provider, &job.Status,
		&job.ProductID, &job.ErrorMessage, &job.CreatedAt, &job.StartedAt, &job.FinishedAt)
	return job, err
}

func SaveProductScanJob(job ProductScanJob) error {
	_, err := requireSQLite().Exec(`INSERT INTO product_scan_jobs(
		id,account,source_url,provider,status,product_id,error_message,created_at,started_at,finished_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, job.ID, job.Account, job.SourceURL, job.Provider, job.Status,
		job.ProductID, job.ErrorMessage, job.CreatedAt, job.StartedAt, job.FinishedAt)
	return err
}

func GetProductScanJob(id string) (*ProductScanJob, error) {
	job, err := scanProductScanJob(requireSQLite().QueryRow(productScanJobSelect+` WHERE id=?`, strings.TrimSpace(id)))
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func GetActiveProductScanJobWithAccount(account, sourceURL string) (*ProductScanJob, error) {
	job, err := scanProductScanJob(requireSQLite().QueryRow(productScanJobSelect+
		` WHERE account=? AND source_url=? AND status IN (?,?) ORDER BY created_at DESC,id DESC LIMIT 1`,
		account, strings.TrimSpace(sourceURL), ProductScanQueued, ProductScanRunning))
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func ListProductScanJobsWithAccount(account string, limit int) ([]ProductScanJob, error) {
	if limit < 1 || limit > 30 {
		limit = 12
	}
	rows, err := requireSQLite().Query(productScanJobSelect+` WHERE account=? ORDER BY created_at DESC,id DESC LIMIT ?`, account, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]ProductScanJob, 0)
	for rows.Next() {
		job, err := scanProductScanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func ListQueuedProductScanJobs() ([]ProductScanJob, error) {
	rows, err := requireSQLite().Query(productScanJobSelect+` WHERE status=? ORDER BY created_at,id`, ProductScanQueued)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := make([]ProductScanJob, 0)
	for rows.Next() {
		job, err := scanProductScanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func RecoverRunningProductScanJobs() error {
	_, err := requireSQLite().Exec(`UPDATE product_scan_jobs SET status=?,started_at='',error_message=''
		WHERE status=?`, ProductScanQueued, ProductScanRunning)
	return err
}

func ClaimProductScanJob(id, startedAt string) (bool, error) {
	result, err := requireSQLite().Exec(`UPDATE product_scan_jobs SET status=?,started_at=?,error_message=''
		WHERE id=? AND status=?`, ProductScanRunning, startedAt, strings.TrimSpace(id), ProductScanQueued)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func CompleteProductScanJob(id, productID, finishedAt string) error {
	_, err := requireSQLite().Exec(`UPDATE product_scan_jobs SET status=?,product_id=?,error_message='',finished_at=?
		WHERE id=?`, ProductScanSucceeded, strings.TrimSpace(productID), finishedAt, strings.TrimSpace(id))
	return err
}

func FailProductScanJob(id, message, finishedAt string) error {
	_, err := requireSQLite().Exec(`UPDATE product_scan_jobs SET status=?,error_message=?,finished_at=?
		WHERE id=?`, ProductScanFailed, strings.TrimSpace(message), finishedAt, strings.TrimSpace(id))
	return err
}
