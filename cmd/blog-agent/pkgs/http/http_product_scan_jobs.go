package http

import (
	"context"
	"database/sql"
	"errors"
	log "mylog"
	"persistence"
	"piagent"
	"sync"
	"time"
)

const productScanWorkerCount = 2

var (
	productScanQueue       = make(chan string, 128)
	productScanWorkersOnce sync.Once
)

func startProductScanWorkers() {
	productScanWorkersOnce.Do(func() {
		if err := persistence.RecoverRunningProductScanJobs(); err != nil {
			log.ErrorF(log.ModuleHandler, "recover product scan jobs: %v", err)
		}
		for index := 0; index < productScanWorkerCount; index++ {
			go productScanWorker()
		}
		jobs, err := persistence.ListQueuedProductScanJobs()
		if err != nil {
			log.ErrorF(log.ModuleHandler, "list queued product scan jobs: %v", err)
			return
		}
		go func() {
			for _, job := range jobs {
				productScanQueue <- job.ID
			}
		}()
	})
}

func enqueueProductScanJob(id string) {
	productScanQueue <- id
}

func productScanWorker() {
	for id := range productScanQueue {
		runProductScanJob(id)
	}
}

func runProductScanJob(id string) {
	now := productTimestamp()
	claimed, err := persistence.ClaimProductScanJob(id, now)
	if err != nil || !claimed {
		if err != nil {
			log.ErrorF(log.ModuleHandler, "claim product scan job %s: %v", id, err)
		}
		return
	}
	job, err := persistence.GetProductScanJob(id)
	if err != nil {
		failProductScanJob(id, "读取扫描任务失败")
		return
	}
	result, err := piagent.ScanProductURL(context.Background(), job.Account, job.SourceURL, job.Provider)
	if err != nil {
		failProductScanJob(id, err.Error())
		return
	}
	card := productDraftToCard(result.Draft)
	normalizeProductCard(&card)
	card.SourceURL = job.SourceURL
	if validateOptionalProductURL(card.CoverURL, "封面链接") != nil {
		card.CoverURL = ""
	}
	card.IsNew = true
	card.UpdatedAt = productTimestamp()

	existing, lookupErr := persistence.GetProductCardBySourceURLWithAccount(job.Account, job.SourceURL)
	switch {
	case lookupErr == nil:
		card.ID = existing.ID
		card.CreatedAt = existing.CreatedAt
		if _, err := persistence.UpdateProductCardWithAccount(job.Account, card); err != nil {
			failProductScanJob(id, "自动更新产品卡失败")
			return
		}
	case errors.Is(lookupErr, sql.ErrNoRows):
		card.ID, err = newProductID()
		if err != nil {
			failProductScanJob(id, "生成产品编号失败")
			return
		}
		card.CreatedAt = card.UpdatedAt
		if err := persistence.SaveProductCardWithAccount(job.Account, card); err != nil {
			failProductScanJob(id, "自动保存产品卡失败")
			return
		}
	default:
		failProductScanJob(id, "检查已有产品卡失败")
		return
	}

	if err := persistence.RecordPIUsage(job.Account, result.Provider, result.Model,
		result.Usage.PromptTokens, result.Usage.CompletionTokens, result.Usage.TotalTokens,
		result.DurationMs, "product_scan"); err != nil {
		log.ErrorF(log.ModuleHandler, "record product scan usage for job %s: %v", id, err)
	}
	if err := persistence.CompleteProductScanJob(id, card.ID, productTimestamp()); err != nil {
		log.ErrorF(log.ModuleHandler, "complete product scan job %s: %v", id, err)
	}
}

func failProductScanJob(id, message string) {
	message = limitProductField(message, 600)
	if err := persistence.FailProductScanJob(id, message, productTimestamp()); err != nil {
		log.ErrorF(log.ModuleHandler, "fail product scan job %s: %v", id, err)
	}
}

func productDraftToCard(draft piagent.ProductDraft) persistence.ProductCard {
	return persistence.ProductCard{
		Name: draft.Name, SourceURL: draft.SourceURL, CoverURL: draft.CoverURL,
		ProductType: draft.ProductType, Summary: draft.Summary, Positioning: draft.Positioning,
		TargetUsers: draft.TargetUsers, Problem: draft.Problem, CoreLoop: draft.CoreLoop,
		CoreMechanism: draft.CoreMechanism, KeyMechanics: draft.KeyMechanics,
		FeedbackRewards: draft.FeedbackRewards, SocialMechanism: draft.SocialMechanism,
		Surprise: draft.Surprise, Retention: draft.Retention, BusinessModel: draft.BusinessModel,
		Strengths: draft.Strengths, UserComplaints: draft.UserComplaints,
		CompetitiveEdge: draft.CompetitiveEdge, TransferableIdeas: draft.TransferableIdeas,
		Opportunities: draft.Opportunities, Tags: draft.Tags, ResearchSources: draft.ResearchSources,
		Confidence: draft.Confidence, Evidence: draft.Evidence, LastResearchedAt: draft.LastResearchedAt,
	}
}

func productTimestamp() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
