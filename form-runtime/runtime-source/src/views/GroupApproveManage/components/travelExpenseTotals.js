/**
 * 优先根据子表当前数据求和，避免表单虚拟合计字段未及时刷新。
 */
export function getTravelExpenseTotals(value) {
  const tableNames = [
    'expenseBudgetList',
    'expenseInAccountInfoList',
    'travelPersonnelVoList',
    'expenseDetailList'
  ];

  return tableNames.map(tableName => {
    const list = value[tableName];
    if (!Array.isArray(list)) {
      return value[`${tableName}_total`];
    }

    let hasInvalidAmount = false;
    const total = list.reduce((sum, item) => {
      const rawAmount = item && item.money;
      const amount = Number(rawAmount);
      if (rawAmount === null || rawAmount === undefined || (typeof rawAmount === 'string' && rawAmount.trim() === '') || !Number.isFinite(amount)) {
        hasInvalidAmount = true;
        return sum;
      }
      return sum + amount;
    }, 0);

    return hasInvalidAmount ? `invalid:${tableName}` : total;
  });
}

export function areTravelExpenseTotalsEqual(values) {
  const normalizedValues = values.map(value => {
    if (value === null || value === undefined || (typeof value === 'string' && value.trim() === '')) {
      return `empty:${String(value)}`;
    }

    const number = Number(value);
    return Number.isFinite(number) ? `number:${number.toFixed(2)}` : `raw:${String(value)}`;
  });

  return new Set(normalizedValues).size <= 1;
}
