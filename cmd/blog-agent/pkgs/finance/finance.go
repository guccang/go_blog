package finance

import (
	"math"
)

// AssetCalculationRequest 资产计算请求
type AssetCalculationRequest struct {
	InitialCapital   float64 `json:"initialCapital"`   // 初始资金
	HousePrice       float64 `json:"housePrice"`       // 购房价格
	DownPayment      float64 `json:"downPayment"`      // 首付金额
	DownPaymentRatio float64 `json:"downPaymentRatio"` // 首付比例 (0-1)
	LoanRate         float64 `json:"loanRate"`         // 贷款利率 (年利率，如0.05表示5%)
	AppreciationRate float64 `json:"appreciationRate"` // 房屋增值利率 (年增长率，可为负)
	AnnualIncome     float64 `json:"annualIncome"`     // 家庭年收入
	IncomeGrowthRate float64 `json:"incomeGrowthRate"` // 收入年增长率 (默认0)
	Years            int     `json:"years"`            // 计算年数 (如5,10,30)
	ExpenseRatio     float64 `json:"expenseRatio"`     // 支出占收入比例 (默认0.5)
}

// YearlyAssetDetail 年度资产详情
type YearlyAssetDetail struct {
	Year              int     `json:"year"`              // 年份
	HouseValue        float64 `json:"houseValue"`        // 房屋价值
	LoanBalance       float64 `json:"loanBalance"`       // 贷款余额
	NetHouseEquity    float64 `json:"netHouseEquity"`    // 房屋净资产 (房屋价值 - 贷款余额)
	CashSavings       float64 `json:"cashSavings"`       // 现金储蓄
	TotalAssets       float64 `json:"totalAssets"`       // 总资产 (房屋净资产 + 现金储蓄)
	AnnualIncome      float64 `json:"annualIncome"`      // 当年年收入
	AnnualPayment     float64 `json:"annualPayment"`     // 当年贷款还款
	PrincipalPaid     float64 `json:"principalPaid"`     // 当年偿还本金
	InterestPaid      float64 `json:"interestPaid"`      // 当年支付利息
	AnnualExpense     float64 `json:"annualExpense"`     // 当年支出
	AnnualSavings     float64 `json:"annualSavings"`     // 当年储蓄
}

// AssetCalculationResponse 资产计算响应
type AssetCalculationResponse struct {
	Summary struct {
		InitialCapital    float64 `json:"initialCapital"`    // 初始资金
		HousePrice        float64 `json:"housePrice"`        // 购房价格
		DownPayment       float64 `json:"downPayment"`       // 首付金额
		LoanAmount        float64 `json:"loanAmount"`        // 贷款金额
		LoanRate          float64 `json:"loanRate"`          // 贷款利率
		AppreciationRate  float64 `json:"appreciationRate"`  // 增值利率
		AnnualIncome      float64 `json:"annualIncome"`      // 初始年收入
		MonthlyPayment    float64 `json:"monthlyPayment"`    // 月供
		AnnualPayment     float64 `json:"annualPayment"`     // 年供
		TotalYears        int     `json:"totalYears"`        // 总年数
		FinalTotalAssets  float64 `json:"finalTotalAssets"`  // 最终总资产
		FinalHouseValue   float64 `json:"finalHouseValue"`   // 最终房屋价值
		FinalLoanBalance  float64 `json:"finalLoanBalance"`  // 最终贷款余额
		FinalCashSavings  float64 `json:"finalCashSavings"`  // 最终现金储蓄
		TotalInterestPaid float64 `json:"totalInterestPaid"` // 总支付利息
		TotalPrincipalPaid float64 `json:"totalPrincipalPaid"` // 总偿还本金
	} `json:"summary"`
	YearlyDetails []YearlyAssetDetail `json:"yearlyDetails"` // 年度详细数据
}

// CalculateAssets 计算家庭资产
func CalculateAssets(req AssetCalculationRequest) AssetCalculationResponse {
	// 确保首付金额和比例一致
	if req.DownPayment <= 0 && req.DownPaymentRatio > 0 {
		req.DownPayment = req.HousePrice * req.DownPaymentRatio
	} else if req.DownPayment > 0 && req.DownPaymentRatio <= 0 {
		req.DownPaymentRatio = req.DownPayment / req.HousePrice
	}

	// 计算贷款金额
	loanAmount := req.HousePrice - req.DownPayment
	if loanAmount < 0 {
		loanAmount = 0
	}

	// 计算月供 (等额本息)
	monthlyRate := req.LoanRate / 12.0
	months := 30 * 12 // 30年贷款期

	var monthlyPayment float64
	if monthlyRate > 0 && loanAmount > 0 {
		monthlyPayment = loanAmount * monthlyRate * math.Pow(1+monthlyRate, float64(months)) /
			(math.Pow(1+monthlyRate, float64(months)) - 1)
	}

	annualPayment := monthlyPayment * 12

	// 初始化响应
	resp := AssetCalculationResponse{}

	// 设置汇总信息
	resp.Summary.InitialCapital = req.InitialCapital
	resp.Summary.HousePrice = req.HousePrice
	resp.Summary.DownPayment = req.DownPayment
	resp.Summary.LoanAmount = loanAmount
	resp.Summary.LoanRate = req.LoanRate
	resp.Summary.AppreciationRate = req.AppreciationRate
	resp.Summary.AnnualIncome = req.AnnualIncome
	resp.Summary.MonthlyPayment = monthlyPayment
	resp.Summary.AnnualPayment = annualPayment
	resp.Summary.TotalYears = req.Years

	// 初始化变量
	houseValue := req.HousePrice
	loanBalance := loanAmount
	cashSavings := req.InitialCapital - req.DownPayment // 购房后剩余现金
	if cashSavings < 0 {
		cashSavings = 0
	}
	currentIncome := req.AnnualIncome

	totalInterestPaid := 0.0
	totalPrincipalPaid := 0.0

	// 计算每年数据
	for year := 1; year <= req.Years; year++ {
		// 计算当年还款明细
		annualInterest := 0.0
		annualPrincipal := 0.0

		if loanBalance > 0 {
			// 计算当年的还款分配 (简化计算，实际每月不同)
			remainingMonths := (30 - (year - 1)) * 12
			if remainingMonths > 0 {
				// 重新计算当年的月供分配
				remainingLoanBalance := loanBalance
				for month := 1; month <= 12; month++ {
					if remainingLoanBalance <= 0 {
						break
					}
					monthlyInterest := remainingLoanBalance * monthlyRate
					monthlyPrincipal := monthlyPayment - monthlyInterest
					if monthlyPrincipal > remainingLoanBalance {
						monthlyPrincipal = remainingLoanBalance
					}

					annualInterest += monthlyInterest
					annualPrincipal += monthlyPrincipal
					remainingLoanBalance -= monthlyPrincipal
				}
				loanBalance -= annualPrincipal
			}
		}

		// 更新房屋价值
		houseValue *= (1 + req.AppreciationRate)

		// 计算支出和储蓄
		annualExpense := currentIncome * req.ExpenseRatio
		availableForPayment := currentIncome - annualExpense
		annualSavings := availableForPayment - annualPayment

		// 更新现金储蓄（可能为负数，表示需要借贷）
		cashSavings += annualSavings

		// 更新总收入
		totalInterestPaid += annualInterest
		totalPrincipalPaid += annualPrincipal

		// 创建年度详情
		detail := YearlyAssetDetail{
			Year:           year,
			HouseValue:     round(houseValue, 2),
			LoanBalance:    round(loanBalance, 2),
			NetHouseEquity: round(houseValue-loanBalance, 2),
			CashSavings:    round(cashSavings, 2),
			TotalAssets:    round(houseValue-loanBalance+cashSavings, 2),
			AnnualIncome:   round(currentIncome, 2),
			AnnualPayment:  round(annualPayment, 2),
			PrincipalPaid:  round(annualPrincipal, 2),
			InterestPaid:   round(annualInterest, 2),
			AnnualExpense:  round(annualExpense, 2),
			AnnualSavings:  round(annualSavings, 2),
		}

		resp.YearlyDetails = append(resp.YearlyDetails, detail)

		// 更新下一年收入 (考虑增长)
		currentIncome *= (1 + req.IncomeGrowthRate)
	}

	// 设置最终汇总数据
	if len(resp.YearlyDetails) > 0 {
		lastDetail := resp.YearlyDetails[len(resp.YearlyDetails)-1]
		resp.Summary.FinalTotalAssets = lastDetail.TotalAssets
		resp.Summary.FinalHouseValue = lastDetail.HouseValue
		resp.Summary.FinalLoanBalance = lastDetail.LoanBalance
		resp.Summary.FinalCashSavings = lastDetail.CashSavings
	}
	resp.Summary.TotalInterestPaid = round(totalInterestPaid, 2)
	resp.Summary.TotalPrincipalPaid = round(totalPrincipalPaid, 2)

	return resp
}

// round 四舍五入到指定小数位
func round(value float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}