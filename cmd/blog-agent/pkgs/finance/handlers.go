package finance

import (
	"encoding/json"
	log "mylog"
	"net/http"
	"view"
)

// HandleFinancePage 资产计算主页
func HandleFinancePage(w http.ResponseWriter, r *http.Request) {
	// 渲染资产计算主页
	view.PageFinance(w)
}

// HandleCalculateAssets 计算家庭资产API
func HandleCalculateAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AssetCalculationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.ErrorF(log.ModuleFinance, "Failed to decode request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 验证必要参数
	if req.HousePrice <= 0 {
		http.Error(w, "购房价格必须大于0", http.StatusBadRequest)
		return
	}
	if req.DownPayment <= 0 && req.DownPaymentRatio <= 0 {
		http.Error(w, "必须提供首付金额或首付比例", http.StatusBadRequest)
		return
	}
	if req.AnnualIncome < 0 {
		http.Error(w, "家庭年收入不能为负", http.StatusBadRequest)
		return
	}
	if req.Years <= 0 {
		req.Years = 30 // 默认30年
	}
	if req.Years > 50 {
		req.Years = 50 // 限制最大50年
	}
	if req.ExpenseRatio <= 0 {
		req.ExpenseRatio = 0.5 // 默认支出占收入50%
	}
	if req.ExpenseRatio >= 1 {
		req.ExpenseRatio = 0.9 // 限制最大90%
	}

	log.DebugF(log.ModuleFinance, "Calculating assets for %d years", req.Years)

	// 执行计算
	resp := CalculateAssets(req)

	// 返回结果
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.ErrorF(log.ModuleFinance, "Failed to encode response: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// HandleGetDefaultValues 获取默认值API
func HandleGetDefaultValues(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	defaultValues := AssetCalculationRequest{
		InitialCapital:   500000,   // 50万初始资金
		HousePrice:       5000000,  // 500万房价
		DownPayment:      1500000,  // 150万首付
		DownPaymentRatio: 0.3,      // 30%首付比例
		LoanRate:         0.04,     // 4%贷款利率
		AppreciationRate: 0.03,     // 3%年增值
		AnnualIncome:     300000,   // 30万年收入
		IncomeGrowthRate: 0.05,     // 5%收入年增长
		Years:            30,       // 30年
		ExpenseRatio:     0.5,      // 50%支出比例
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(defaultValues); err != nil {
		log.ErrorF(log.ModuleFinance, "Failed to encode default values: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}