package persistence

import (
	"encoding/json"
	"strings"
)

// ProductResearchSource 记录研究结论来自哪里，便于人工复核。
type ProductResearchSource struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Kind    string `json:"kind"`
	Snippet string `json:"snippet"`
	Fetched bool   `json:"fetched"`
}

// ProductCard 是按账户隔离保存的产品灵感卡。
type ProductCard struct {
	ID                string                  `json:"id"`
	Account           string                  `json:"-"`
	IsNew             bool                    `json:"is_new"`
	Name              string                  `json:"name"`
	SourceURL         string                  `json:"source_url"`
	CoverURL          string                  `json:"cover_url"`
	ProductType       string                  `json:"product_type"`
	Summary           string                  `json:"summary"`
	Positioning       string                  `json:"positioning"`
	TargetUsers       string                  `json:"target_users"`
	Problem           string                  `json:"problem"`
	CoreLoop          string                  `json:"core_loop"`
	CoreMechanism     string                  `json:"core_mechanism"`
	KeyMechanics      []string                `json:"key_mechanics"`
	FeedbackRewards   string                  `json:"feedback_rewards"`
	SocialMechanism   string                  `json:"social_mechanism"`
	Surprise          string                  `json:"surprise"`
	Retention         string                  `json:"retention"`
	BusinessModel     string                  `json:"business_model"`
	Strengths         []string                `json:"strengths"`
	UserComplaints    []string                `json:"user_complaints"`
	CompetitiveEdge   string                  `json:"competitive_edge"`
	TransferableIdeas []string                `json:"transferable_ideas"`
	Opportunities     []string                `json:"opportunities"`
	Tags              []string                `json:"tags"`
	ResearchSources   []ProductResearchSource `json:"research_sources"`
	Confidence        map[string]string       `json:"confidence"`
	Evidence          map[string][]string     `json:"evidence"`
	LastResearchedAt  string                  `json:"last_researched_at"`
	CreatedAt         string                  `json:"created_at"`
	UpdatedAt         string                  `json:"updated_at"`
}

type productCardJSON struct {
	ideas, opportunities, tags          string
	keyMechanics, strengths, complaints string
	sources, confidence, evidence       string
}

func encodeProductCardJSON(card ProductCard) (productCardJSON, error) {
	values := []any{card.TransferableIdeas, card.Opportunities, card.Tags, card.KeyMechanics,
		card.Strengths, card.UserComplaints, card.ResearchSources, card.Confidence, card.Evidence}
	encoded := make([]string, len(values))
	for index, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return productCardJSON{}, err
		}
		encoded[index] = string(data)
	}
	return productCardJSON{
		ideas: encoded[0], opportunities: encoded[1], tags: encoded[2], keyMechanics: encoded[3],
		strengths: encoded[4], complaints: encoded[5], sources: encoded[6], confidence: encoded[7], evidence: encoded[8],
	}, nil
}

func SaveProductCardWithAccount(account string, card ProductCard) error {
	data, err := encodeProductCardJSON(card)
	if err != nil {
		return err
	}
	_, err = requireSQLite().Exec(`INSERT INTO product_cards(
		id,account,name,is_new,source_url,cover_url,product_type,summary,positioning,target_users,problem,
		core_loop,core_mechanism,key_mechanics_json,feedback_rewards,social_mechanism,surprise,
		retention,business_model,strengths_json,user_complaints_json,competitive_edge,
		transferable_ideas_json,opportunities_json,tags_json,research_sources_json,
		confidence_json,evidence_json,last_researched_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		card.ID, account, card.Name, card.IsNew, card.SourceURL, card.CoverURL, card.ProductType, card.Summary,
		card.Positioning, card.TargetUsers, card.Problem, card.CoreLoop, card.CoreMechanism,
		data.keyMechanics, card.FeedbackRewards, card.SocialMechanism, card.Surprise,
		card.Retention, card.BusinessModel, data.strengths, data.complaints, card.CompetitiveEdge,
		data.ideas, data.opportunities, data.tags, data.sources, data.confidence, data.evidence,
		card.LastResearchedAt, card.CreatedAt, card.UpdatedAt)
	return err
}

func UpdateProductCardWithAccount(account string, card ProductCard) (bool, error) {
	data, err := encodeProductCardJSON(card)
	if err != nil {
		return false, err
	}
	result, err := requireSQLite().Exec(`UPDATE product_cards SET
		name=?,is_new=?,source_url=?,cover_url=?,product_type=?,summary=?,positioning=?,target_users=?,problem=?,
		core_loop=?,core_mechanism=?,key_mechanics_json=?,feedback_rewards=?,social_mechanism=?,
		surprise=?,retention=?,business_model=?,strengths_json=?,user_complaints_json=?,competitive_edge=?,
		transferable_ideas_json=?,opportunities_json=?,tags_json=?,research_sources_json=?,confidence_json=?,
		evidence_json=?,last_researched_at=?,updated_at=? WHERE account=? AND id=?`,
		card.Name, card.IsNew, card.SourceURL, card.CoverURL, card.ProductType, card.Summary, card.Positioning,
		card.TargetUsers, card.Problem, card.CoreLoop, card.CoreMechanism, data.keyMechanics,
		card.FeedbackRewards, card.SocialMechanism, card.Surprise, card.Retention, card.BusinessModel,
		data.strengths, data.complaints, card.CompetitiveEdge, data.ideas, data.opportunities,
		data.tags, data.sources, data.confidence, data.evidence, card.LastResearchedAt, card.UpdatedAt,
		account, strings.TrimSpace(card.ID))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

const productCardSelect = `SELECT id,name,is_new,source_url,cover_url,product_type,summary,positioning,
	target_users,problem,core_loop,core_mechanism,key_mechanics_json,feedback_rewards,social_mechanism,
	surprise,retention,business_model,strengths_json,user_complaints_json,competitive_edge,
	transferable_ideas_json,opportunities_json,tags_json,research_sources_json,confidence_json,
	evidence_json,last_researched_at,created_at,updated_at FROM product_cards`

func scanProductCard(scanner interface{ Scan(...any) error }, account string) (ProductCard, error) {
	var card ProductCard
	var data productCardJSON
	err := scanner.Scan(&card.ID, &card.Name, &card.IsNew, &card.SourceURL, &card.CoverURL, &card.ProductType,
		&card.Summary, &card.Positioning, &card.TargetUsers, &card.Problem, &card.CoreLoop,
		&card.CoreMechanism, &data.keyMechanics, &card.FeedbackRewards, &card.SocialMechanism,
		&card.Surprise, &card.Retention, &card.BusinessModel, &data.strengths, &data.complaints,
		&card.CompetitiveEdge, &data.ideas, &data.opportunities, &data.tags, &data.sources,
		&data.confidence, &data.evidence, &card.LastResearchedAt, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return ProductCard{}, err
	}
	card.Account = account
	if err := decodeProductCardJSON(&card, data); err != nil {
		return ProductCard{}, err
	}
	return card, nil
}

func ListProductCardsWithAccount(account string) ([]ProductCard, error) {
	rows, err := requireSQLite().Query(productCardSelect+` WHERE account=? ORDER BY updated_at DESC,id DESC`, account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ProductCard, 0)
	for rows.Next() {
		card, err := scanProductCard(rows, account)
		if err != nil {
			return nil, err
		}
		items = append(items, card)
	}
	return items, rows.Err()
}

func GetProductCardWithAccount(account, id string) (*ProductCard, error) {
	card, err := scanProductCard(requireSQLite().QueryRow(productCardSelect+` WHERE account=? AND id=?`, account, strings.TrimSpace(id)), account)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func GetProductCardBySourceURLWithAccount(account, sourceURL string) (*ProductCard, error) {
	card, err := scanProductCard(requireSQLite().QueryRow(productCardSelect+` WHERE account=? AND source_url=? ORDER BY updated_at DESC LIMIT 1`, account, strings.TrimSpace(sourceURL)), account)
	if err != nil {
		return nil, err
	}
	return &card, nil
}

func MarkProductCardViewedWithAccount(account, id string) (bool, error) {
	result, err := requireSQLite().Exec(`UPDATE product_cards SET is_new=0 WHERE account=? AND id=?`, account, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func DeleteProductCardWithAccount(account, id string) (bool, error) {
	result, err := requireSQLite().Exec(`DELETE FROM product_cards WHERE account=? AND id=?`, account, strings.TrimSpace(id))
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count > 0, err
}

func decodeProductCardJSON(card *ProductCard, data productCardJSON) error {
	targets := []struct {
		raw    string
		target any
	}{
		{data.ideas, &card.TransferableIdeas}, {data.opportunities, &card.Opportunities},
		{data.tags, &card.Tags}, {data.keyMechanics, &card.KeyMechanics},
		{data.strengths, &card.Strengths}, {data.complaints, &card.UserComplaints},
		{data.sources, &card.ResearchSources}, {data.confidence, &card.Confidence},
		{data.evidence, &card.Evidence},
	}
	for _, item := range targets {
		if err := json.Unmarshal([]byte(item.raw), item.target); err != nil {
			return err
		}
	}
	return nil
}
