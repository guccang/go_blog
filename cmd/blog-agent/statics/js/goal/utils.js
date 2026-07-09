// statics/js/goal/utils.js
const LEVELS = ['daily', 'weekly', 'monthly', 'yearly'];
const LEVEL_LABELS = { daily: '日目标', weekly: '周目标', monthly: '月目标', yearly: '年目标' };
const PRIORITY_LABELS = { high: '高优', medium: '普通', low: '低优' };
const STATUS_LABELS = { pending: '待开始', in_progress: '进行中', completed: '已完成', cancelled: '已取消' };
const PARENT_LEVEL = { daily: 'weekly', weekly: 'monthly', monthly: 'yearly', yearly: null };

function today() {
  return new Date().toISOString().split('T')[0];
}

function currentPeriod(level) {
  const now = new Date();
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, '0');
  const d = String(now.getDate()).padStart(2, '0');

  switch (level) {
    case 'daily': return `${y}-${m}-${d}`;
    case 'weekly':
      const jan1 = new Date(y, 0, 1);
      const week = Math.ceil(((now - jan1) / 86400000 + jan1.getDay() + 1) / 7);
      return `${y}-W${String(week).padStart(2, '0')}`;
    case 'monthly': return `${y}-${m}`;
    case 'yearly': return `${y}`;
  }
}

function periodLabel(level, period) {
  if (!period) return '';
  switch (level) {
    case 'daily': {
      const d = new Date(period);
      const days = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'];
      return `${period} ${days[d.getDay()]}`;
    }
    case 'weekly': {
      const [y, w] = period.split('-W');
      return `${y}年第${parseInt(w)}周`;
    }
    case 'monthly': {
      const [y, m] = period.split('-');
      return `${y}年${parseInt(m)}月`;
    }
    case 'yearly': return `${period}年`;
    default: return period;
  }
}

export { LEVELS, LEVEL_LABELS, PRIORITY_LABELS, STATUS_LABELS, PARENT_LEVEL, today, currentPeriod, periodLabel };
