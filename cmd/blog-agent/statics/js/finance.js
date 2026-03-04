// 家庭资产计算器 - 前端逻辑

// 全局变量
let currentData = null;
let charts = {};

// DOM 元素
const elements = {
    // 输入字段
    initialCapital: document.getElementById('initialCapital'),
    housePrice: document.getElementById('housePrice'),
    downPayment: document.getElementById('downPayment'),
    downPaymentRatio: document.getElementById('downPaymentRatio'),
    loanRate: document.getElementById('loanRate'),
    appreciationRate: document.getElementById('appreciationRate'),
    annualIncome: document.getElementById('annualIncome'),
    incomeGrowthRate: document.getElementById('incomeGrowthRate'),
    years: document.getElementById('years'),
    expenseRatio: document.getElementById('expenseRatio'),

    // 按钮
    calculateBtn: document.getElementById('calculateBtn'),
    resetBtn: document.getElementById('resetBtn'),
    loadDefaultsBtn: document.getElementById('loadDefaultsBtn'),
    exportBtn: document.getElementById('exportBtn'),

    // 结果显示区域
    loading: document.getElementById('loading'),
    errorMessage: document.getElementById('errorMessage'),
    successMessage: document.getElementById('successMessage'),
    results: document.getElementById('results'),
    noResults: document.getElementById('noResults'),

    // 摘要网格
    summaryGrid: document.getElementById('summaryGrid'),

    // 表格
    detailsTableBody: document.getElementById('detailsTableBody'),
    paymentTableBody: document.getElementById('paymentTableBody'),

    // 标签页
    tabBtns: document.querySelectorAll('.tab-btn'),
    tabContents: document.querySelectorAll('.tab-content')
};

// 初始化
document.addEventListener('DOMContentLoaded', function() {
    initEventListeners();
    loadDefaultValues();
});

// 初始化事件监听器
function initEventListeners() {
    // 计算按钮
    elements.calculateBtn.addEventListener('click', calculateAssets);

    // 重置按钮
    elements.resetBtn.addEventListener('click', resetForm);

    // 加载示例按钮
    elements.loadDefaultsBtn.addEventListener('click', loadDefaultValues);

    // 导出按钮
    elements.exportBtn.addEventListener('click', exportData);

    // 输入字段联动：首付金额和首付比例
    elements.downPayment.addEventListener('input', updateDownPaymentRatio);
    elements.downPaymentRatio.addEventListener('input', updateDownPaymentAmount);
    elements.housePrice.addEventListener('input', updateDownPaymentFields);

    // 标签页切换
    elements.tabBtns.forEach(btn => {
        btn.addEventListener('click', function() {
            const tabId = this.dataset.tab;
            switchTab(tabId);
        });
    });

    // 允许按Enter键触发计算
    document.addEventListener('keypress', function(e) {
        if (e.key === 'Enter' && e.target.tagName === 'INPUT') {
            calculateAssets();
        }
    });
}

// 更新首付比例
function updateDownPaymentRatio() {
    const housePrice = parseFloat(elements.housePrice.value) || 0;
    const downPayment = parseFloat(elements.downPayment.value) || 0;

    if (housePrice > 0) {
        const ratio = (downPayment / housePrice) * 100;
        elements.downPaymentRatio.value = ratio.toFixed(1);
    }
}

// 更新首付金额
function updateDownPaymentAmount() {
    const housePrice = parseFloat(elements.housePrice.value) || 0;
    const ratio = parseFloat(elements.downPaymentRatio.value) || 0;

    if (housePrice > 0 && ratio >= 0 && ratio <= 100) {
        const amount = housePrice * (ratio / 100);
        elements.downPayment.value = Math.round(amount);
    }
}

// 更新首付字段
function updateDownPaymentFields() {
    updateDownPaymentRatio();
}

// 切换标签页
function switchTab(tabId) {
    // 更新激活的标签按钮
    elements.tabBtns.forEach(btn => {
        if (btn.dataset.tab === tabId) {
            btn.classList.add('active');
        } else {
            btn.classList.remove('active');
        }
    });

    // 显示对应的标签内容
    elements.tabContents.forEach(content => {
        if (content.id === `tab-${tabId}`) {
            content.classList.add('active');
        } else {
            content.classList.remove('active');
        }
    });

    // 如果切换到图表标签，重新渲染图表
    if (tabId === 'charts' && currentData) {
        renderAnalysisCharts();
    }
}

// 加载默认值
function loadDefaultValues() {
    showLoading('正在加载默认值...');

    fetch('/api/finance/defaults')
        .then(response => {
            if (!response.ok) {
                throw new Error(`HTTP error! status: ${response.status}`);
            }
            return response.json();
        })
        .then(data => {
            // 填充表单
            elements.initialCapital.value = data.initialCapital || '';
            elements.housePrice.value = data.housePrice || '';
            elements.downPayment.value = data.downPayment || '';
            elements.downPaymentRatio.value = (data.downPaymentRatio * 100) || '';
            elements.loanRate.value = (data.loanRate * 100) || '';
            elements.appreciationRate.value = (data.appreciationRate * 100) || '';
            elements.annualIncome.value = data.annualIncome || '';
            elements.incomeGrowthRate.value = (data.incomeGrowthRate * 100) || '';
            elements.years.value = data.years || '';
            elements.expenseRatio.value = (data.expenseRatio * 100) || '';

            hideLoading();
            showSuccess('已加载默认参数值');
        })
        .catch(error => {
            console.error('Error loading defaults:', error);
            hideLoading();
            showError('加载默认值失败，使用内置默认值');

            // 使用内置默认值
            const defaults = {
                initialCapital: 500000,
                housePrice: 5000000,
                downPayment: 1500000,
                downPaymentRatio: 0.3,
                loanRate: 0.04,
                appreciationRate: 0.03,
                annualIncome: 300000,
                incomeGrowthRate: 0.05,
                years: 30,
                expenseRatio: 0.5
            };

            elements.initialCapital.value = defaults.initialCapital;
            elements.housePrice.value = defaults.housePrice;
            elements.downPayment.value = defaults.downPayment;
            elements.downPaymentRatio.value = defaults.downPaymentRatio * 100;
            elements.loanRate.value = defaults.loanRate * 100;
            elements.appreciationRate.value = defaults.appreciationRate * 100;
            elements.annualIncome.value = defaults.annualIncome;
            elements.incomeGrowthRate.value = defaults.incomeGrowthRate * 100;
            elements.years.value = defaults.years;
            elements.expenseRatio.value = defaults.expenseRatio * 100;
        });
}

// 计算资产
function calculateAssets() {
    // 验证输入
    if (!validateInputs()) {
        return;
    }

    // 收集数据
    const requestData = {
        initialCapital: parseFloat(elements.initialCapital.value) || 0,
        housePrice: parseFloat(elements.housePrice.value) || 0,
        downPayment: parseFloat(elements.downPayment.value) || 0,
        downPaymentRatio: (parseFloat(elements.downPaymentRatio.value) || 0) / 100,
        loanRate: (parseFloat(elements.loanRate.value) || 0) / 100,
        appreciationRate: (parseFloat(elements.appreciationRate.value) || 0) / 100,
        annualIncome: parseFloat(elements.annualIncome.value) || 0,
        incomeGrowthRate: (parseFloat(elements.incomeGrowthRate.value) || 0) / 100,
        years: parseInt(elements.years.value) || 30,
        expenseRatio: (parseFloat(elements.expenseRatio.value) || 50) / 100
    };

    // 显示加载状态
    showLoading('正在计算资产...');
    elements.results.style.display = 'none';
    elements.noResults.style.display = 'none';

    // 发送请求
    fetch('/api/finance/calculate', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(requestData)
    })
    .then(response => {
        if (!response.ok) {
            return response.text().then(text => {
                throw new Error(`HTTP error! status: ${response.status}, message: ${text}`);
            });
        }
        return response.json();
    })
    .then(data => {
        currentData = data;
        hideLoading();
        showSuccess('计算完成！');
        displayResults(data);
    })
    .catch(error => {
        console.error('Error calculating assets:', error);
        hideLoading();
        showError(`计算失败: ${error.message}`);
    });
}

// 验证输入
function validateInputs() {
    // 检查必填字段
    if (!elements.housePrice.value || parseFloat(elements.housePrice.value) <= 0) {
        showError('购房价格必须大于0');
        elements.housePrice.focus();
        return false;
    }

    if (!elements.downPayment.value && !elements.downPaymentRatio.value) {
        showError('请输入首付金额或首付比例');
        elements.downPayment.focus();
        return false;
    }

    if (elements.downPayment.value && parseFloat(elements.downPayment.value) > parseFloat(elements.housePrice.value)) {
        showError('首付金额不能超过购房价格');
        elements.downPayment.focus();
        return false;
    }

    if (elements.downPaymentRatio.value && (parseFloat(elements.downPaymentRatio.value) < 0 || parseFloat(elements.downPaymentRatio.value) > 100)) {
        showError('首付比例必须在0-100%之间');
        elements.downPaymentRatio.focus();
        return false;
    }

    if (elements.loanRate.value && parseFloat(elements.loanRate.value) < 0) {
        showError('贷款利率不能为负数');
        elements.loanRate.focus();
        return false;
    }

    if (elements.annualIncome.value && parseFloat(elements.annualIncome.value) < 0) {
        showError('家庭年收入不能为负数');
        elements.annualIncome.focus();
        return false;
    }

    if (!elements.years.value || parseInt(elements.years.value) <= 0) {
        showError('计算年数必须大于0');
        elements.years.focus();
        return false;
    }

    if (elements.expenseRatio.value && (parseFloat(elements.expenseRatio.value) < 0 || parseFloat(elements.expenseRatio.value) > 100)) {
        showError('支出比例必须在0-100%之间');
        elements.expenseRatio.focus();
        return false;
    }

    return true;
}

// 显示结果
function displayResults(data) {
    elements.noResults.style.display = 'none';
    elements.results.style.display = 'block';

    // 更新摘要信息
    updateSummary(data);

    // 更新表格数据
    updateTables(data);

    // 渲染图表
    renderCharts(data);
}

// 更新摘要信息
function updateSummary(data) {
    const summary = data.summary;
    const html = `
        <div class="summary-item">
            <span class="summary-label">初始资金</span>
            <span class="summary-value">${formatCurrency(summary.initialCapital)}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">购房价格</span>
            <span class="summary-value">${formatCurrency(summary.housePrice)}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">贷款金额</span>
            <span class="summary-value">${formatCurrency(summary.loanAmount)}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">月供</span>
            <span class="summary-value">${formatCurrency(summary.monthlyPayment)}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">最终总资产</span>
            <span class="summary-value ${summary.finalTotalAssets >= summary.initialCapital ? 'positive' : 'negative'}">
                ${formatCurrency(summary.finalTotalAssets)}
            </span>
        </div>
        <div class="summary-item">
            <span class="summary-label">最终房屋价值</span>
            <span class="summary-value">${formatCurrency(summary.finalHouseValue)}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">最终贷款余额</span>
            <span class="summary-value">${formatCurrency(summary.finalLoanBalance)}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">总支付利息</span>
            <span class="summary-value">${formatCurrency(summary.totalInterestPaid)}</span>
        </div>
    `;

    elements.summaryGrid.innerHTML = html;
}

// 更新表格数据
function updateTables(data) {
    // 更新年度明细表格
    let detailsHtml = '';
    let paymentHtml = '';

    data.yearlyDetails.forEach(detail => {
        detailsHtml += `
            <tr>
                <td>第${detail.year}年</td>
                <td>${formatCurrency(detail.houseValue)}</td>
                <td>${formatCurrency(detail.loanBalance)}</td>
                <td>${formatCurrency(detail.netHouseEquity)}</td>
                <td>${formatCurrency(detail.cashSavings)}</td>
                <td><strong>${formatCurrency(detail.totalAssets)}</strong></td>
                <td>${formatCurrency(detail.annualIncome)}</td>
                <td>${formatCurrency(detail.annualSavings)}</td>
            </tr>
        `;

        paymentHtml += `
            <tr>
                <td>第${detail.year}年</td>
                <td>${formatCurrency(detail.annualPayment)}</td>
                <td>${formatCurrency(detail.principalPaid)}</td>
                <td>${formatCurrency(detail.interestPaid)}</td>
                <td>${formatCurrency(detail.annualExpense)}</td>
                <td>${formatCurrency(detail.annualSavings)}</td>
            </tr>
        `;
    });

    elements.detailsTableBody.innerHTML = detailsHtml;
    elements.paymentTableBody.innerHTML = paymentHtml;
}

// 渲染图表
function renderCharts(data) {
    // 销毁现有图表
    Object.values(charts).forEach(chart => {
        if (chart) chart.destroy();
    });
    charts = {};

    // 准备数据
    const years = data.yearlyDetails.map(d => `第${d.year}年`);
    const totalAssets = data.yearlyDetails.map(d => d.totalAssets);
    const houseValues = data.yearlyDetails.map(d => d.houseValue);
    const loanBalances = data.yearlyDetails.map(d => d.loanBalance);
    const cashSavings = data.yearlyDetails.map(d => d.cashSavings);
    const annualIncomes = data.yearlyDetails.map(d => d.annualIncome);
    const annualSavings = data.yearlyDetails.map(d => d.annualSavings);
    const annualExpenses = data.yearlyDetails.map(d => d.annualExpense);
    const principalPaid = data.yearlyDetails.map(d => d.principalPaid);
    const interestPaid = data.yearlyDetails.map(d => d.interestPaid);

    // 1. 主要资产增长图表
    const assetsChartCtx = document.getElementById('assetsChart').getContext('2d');
    charts.assetsChart = new Chart(assetsChartCtx, {
        type: 'line',
        data: {
            labels: years,
            datasets: [
                {
                    label: '总资产',
                    data: totalAssets,
                    borderColor: '#3498db',
                    backgroundColor: 'rgba(52, 152, 219, 0.1)',
                    borderWidth: 3,
                    fill: true,
                    tension: 0.2
                },
                {
                    label: '房屋价值',
                    data: houseValues,
                    borderColor: '#2ecc71',
                    backgroundColor: 'rgba(46, 204, 113, 0.1)',
                    borderWidth: 2,
                    fill: false,
                    tension: 0.2
                },
                {
                    label: '现金储蓄',
                    data: cashSavings,
                    borderColor: '#9b59b6',
                    backgroundColor: 'rgba(155, 89, 182, 0.1)',
                    borderWidth: 2,
                    fill: false,
                    tension: 0.2
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: '资产增长趋势',
                    font: { size: 16 }
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `${context.dataset.label}: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return formatCurrency(value, true);
                        }
                    }
                }
            }
        }
    });

    // 2. 明细图表（年度明细标签页）
    const detailsChartCtx = document.getElementById('detailsChart').getContext('2d');
    charts.detailsChart = new Chart(detailsChartCtx, {
        type: 'bar',
        data: {
            labels: years,
            datasets: [
                {
                    label: '房屋净资产',
                    data: data.yearlyDetails.map(d => d.netHouseEquity),
                    backgroundColor: 'rgba(52, 152, 219, 0.7)',
                    borderColor: '#2980b9',
                    borderWidth: 1
                },
                {
                    label: '现金储蓄',
                    data: cashSavings,
                    backgroundColor: 'rgba(46, 204, 113, 0.7)',
                    borderColor: '#27ae60',
                    borderWidth: 1
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: '资产构成（房屋净资产 vs 现金储蓄）',
                    font: { size: 14 }
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `${context.dataset.label}: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return formatCurrency(value, true);
                        }
                    }
                }
            }
        }
    });

    // 3. 还款明细图表
    const paymentChartCtx = document.getElementById('paymentChart').getContext('2d');
    charts.paymentChart = new Chart(paymentChartCtx, {
        type: 'bar',
        data: {
            labels: years,
            datasets: [
                {
                    label: '偿还本金',
                    data: principalPaid,
                    backgroundColor: 'rgba(46, 204, 113, 0.7)',
                    borderColor: '#27ae60',
                    borderWidth: 1
                },
                {
                    label: '支付利息',
                    data: interestPaid,
                    backgroundColor: 'rgba(231, 76, 60, 0.7)',
                    borderColor: '#c0392b',
                    borderWidth: 1
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: {
                    display: true,
                    text: '还款构成（本金 vs 利息）',
                    font: { size: 14 }
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `${context.dataset.label}: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return formatCurrency(value, true);
                        }
                    }
                }
            }
        }
    });

    // 分析图表将在切换到"分析图表"标签页时渲染
}

// 渲染分析图表
function renderAnalysisCharts() {
    if (!currentData) return;

    const years = currentData.yearlyDetails.map(d => `第${d.year}年`);
    const houseValues = currentData.yearlyDetails.map(d => d.houseValue);
    const loanBalances = currentData.yearlyDetails.map(d => d.loanBalance);
    const annualIncomes = currentData.yearlyDetails.map(d => d.annualIncome);
    const annualSavings = currentData.yearlyDetails.map(d => d.annualSavings);
    const annualExpenses = currentData.yearlyDetails.map(d => d.annualExpense);

    // 1. 房屋价值图表
    const houseValueChartCtx = document.getElementById('houseValueChart').getContext('2d');
    if (charts.houseValueChart) charts.houseValueChart.destroy();
    charts.houseValueChart = new Chart(houseValueChartCtx, {
        type: 'line',
        data: {
            labels: years,
            datasets: [{
                label: '房屋价值',
                data: houseValues,
                borderColor: '#e74c3c',
                backgroundColor: 'rgba(231, 76, 60, 0.1)',
                borderWidth: 2,
                fill: true,
                tension: 0.3
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: { display: true, text: '房屋价值增长', font: { size: 14 } },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `房屋价值: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    ticks: {
                        callback: function(value) {
                            return formatCurrency(value, true);
                        }
                    }
                }
            }
        }
    });

    // 2. 贷款余额图表
    const loanBalanceChartCtx = document.getElementById('loanBalanceChart').getContext('2d');
    if (charts.loanBalanceChart) charts.loanBalanceChart.destroy();
    charts.loanBalanceChart = new Chart(loanBalanceChartCtx, {
        type: 'line',
        data: {
            labels: years,
            datasets: [{
                label: '贷款余额',
                data: loanBalances,
                borderColor: '#f39c12',
                backgroundColor: 'rgba(243, 156, 18, 0.1)',
                borderWidth: 2,
                fill: true,
                tension: 0.3
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: { display: true, text: '贷款余额变化', font: { size: 14 } },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `贷款余额: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return formatCurrency(value, true);
                        }
                    }
                }
            }
        }
    });

    // 3. 收入支出图表
    const incomeExpenseChartCtx = document.getElementById('incomeExpenseChart').getContext('2d');
    if (charts.incomeExpenseChart) charts.incomeExpenseChart.destroy();
    charts.incomeExpenseChart = new Chart(incomeExpenseChartCtx, {
        type: 'bar',
        data: {
            labels: years,
            datasets: [
                {
                    label: '年收入',
                    data: annualIncomes,
                    backgroundColor: 'rgba(52, 152, 219, 0.7)',
                    borderColor: '#2980b9',
                    borderWidth: 1
                },
                {
                    label: '年支出',
                    data: annualExpenses,
                    backgroundColor: 'rgba(231, 76, 60, 0.7)',
                    borderColor: '#c0392b',
                    borderWidth: 1
                }
            ]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: { display: true, text: '收入 vs 支出', font: { size: 14 } },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `${context.dataset.label}: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    beginAtZero: true,
                    ticks: {
                        callback: function(value) {
                            return formatCurrency(value, true);
                        }
                    }
                }
            }
        }
    });

    // 4. 储蓄图表
    const savingsChartCtx = document.getElementById('savingsChart').getContext('2d');
    if (charts.savingsChart) charts.savingsChart.destroy();
    charts.savingsChart = new Chart(savingsChartCtx, {
        type: 'line',
        data: {
            labels: years,
            datasets: [{
                label: '年储蓄',
                data: annualSavings,
                borderColor: '#27ae60',
                backgroundColor: 'rgba(39, 174, 96, 0.1)',
                borderWidth: 2,
                fill: true,
                tension: 0.3
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                title: { display: true, text: '年储蓄变化', font: { size: 14 } },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            return `年储蓄: ${formatCurrency(context.raw)}`;
                        }
                    }
                }
            },
            scales: {
                y: {
                    ticks: {
                        callback: function(value) {
                            return formatCurrency(value, true);
                        }
                    }
                }
            }
        }
    });
}

// 重置表单
function resetForm() {
    elements.initialCapital.value = '';
    elements.housePrice.value = '';
    elements.downPayment.value = '';
    elements.downPaymentRatio.value = '';
    elements.loanRate.value = '';
    elements.appreciationRate.value = '';
    elements.annualIncome.value = '';
    elements.incomeGrowthRate.value = '';
    elements.years.value = '';
    elements.expenseRatio.value = '';

    elements.results.style.display = 'none';
    elements.noResults.style.display = 'block';

    // 销毁图表
    Object.values(charts).forEach(chart => {
        if (chart) chart.destroy();
    });
    charts = {};

    currentData = null;
    hideMessages();
    showSuccess('表单已重置');
}

// 导出数据
function exportData() {
    if (!currentData) {
        showError('没有可导出的数据，请先进行计算');
        return;
    }

    // 创建CSV内容
    let csvContent = "家庭资产计算结果\n\n";

    // 添加摘要信息
    csvContent += "摘要信息\n";
    const summary = currentData.summary;
    csvContent += `初始资金,${summary.initialCapital}\n`;
    csvContent += `购房价格,${summary.housePrice}\n`;
    csvContent += `首付金额,${summary.downPayment}\n`;
    csvContent += `贷款金额,${summary.loanAmount}\n`;
    csvContent += `贷款利率,${summary.loanRate}\n`;
    csvContent += `房屋增值率,${summary.appreciationRate}\n`;
    csvContent += `初始年收入,${summary.annualIncome}\n`;
    csvContent += `月供,${summary.monthlyPayment}\n`;
    csvContent += `年供,${summary.annualPayment}\n`;
    csvContent += `计算年数,${summary.totalYears}\n`;
    csvContent += `最终总资产,${summary.finalTotalAssets}\n`;
    csvContent += `最终房屋价值,${summary.finalHouseValue}\n`;
    csvContent += `最终贷款余额,${summary.finalLoanBalance}\n`;
    csvContent += `最终现金储蓄,${summary.finalCashSavings}\n`;
    csvContent += `总支付利息,${summary.totalInterestPaid}\n`;
    csvContent += `总偿还本金,${summary.totalPrincipalPaid}\n\n`;

    // 添加年度明细表头
    csvContent += "年度明细\n";
    csvContent += "年份,房屋价值,贷款余额,房屋净资产,现金储蓄,总资产,年收入,年还款额,偿还本金,支付利息,年支出,年储蓄\n";

    // 添加年度明细数据
    currentData.yearlyDetails.forEach(detail => {
        csvContent += `${detail.year},${detail.houseValue},${detail.loanBalance},${detail.netHouseEquity},`;
        csvContent += `${detail.cashSavings},${detail.totalAssets},${detail.annualIncome},${detail.annualPayment},`;
        csvContent += `${detail.principalPaid},${detail.interestPaid},${detail.annualExpense},${detail.annualSavings}\n`;
    });

    // 创建Blob并下载
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    const url = URL.createObjectURL(blob);

    link.setAttribute('href', url);
    link.setAttribute('download', `家庭资产计算_${new Date().toISOString().slice(0,10)}.csv`);
    link.style.visibility = 'hidden';

    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    showSuccess('数据已导出为CSV文件');
}

// 工具函数：格式化货币
function formatCurrency(value, short = false) {
    if (value === null || value === undefined || isNaN(value)) {
        return '¥0';
    }

    if (value >= 100000000) {
        return `¥${(value / 100000000).toFixed(2)}亿`;
    } else if (value >= 10000) {
        return `¥${(value / 10000).toFixed(2)}万`;
    } else if (short && value >= 1000) {
        return `¥${(value / 1000).toFixed(1)}千`;
    } else {
        return `¥${value.toFixed(0)}`;
    }
}

// 工具函数：显示加载状态
function showLoading(message) {
    elements.loading.style.display = 'block';
    if (message) {
        elements.loading.querySelector('p').textContent = message;
    }
    hideMessages();
}

// 工具函数：隐藏加载状态
function hideLoading() {
    elements.loading.style.display = 'none';
}

// 工具函数：显示错误消息
function showError(message) {
    elements.errorMessage.textContent = message;
    elements.errorMessage.style.display = 'block';
    elements.successMessage.style.display = 'none';

    // 3秒后自动隐藏
    setTimeout(hideMessages, 5000);
}

// 工具函数：显示成功消息
function showSuccess(message) {
    elements.successMessage.textContent = message;
    elements.successMessage.style.display = 'block';
    elements.errorMessage.style.display = 'none';

    // 3秒后自动隐藏
    setTimeout(hideMessages, 3000);
}

// 工具函数：隐藏所有消息
function hideMessages() {
    elements.errorMessage.style.display = 'none';
    elements.successMessage.style.display = 'none';
}