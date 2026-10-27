function getContractCurrencyCode(currency) {
  const value = currency && typeof currency === 'object'
    ? (currency.value !== undefined ? currency.value : currency.dictValue)
    : currency;

  return String(value || 'CNY');
}

export function getContractAmountFields(formValue = {}) {
  const currencyCode = getContractCurrencyCode(formValue.currency);
  const originalAmount = formValue.contractSum;

  return {
    currencyCode,
    originalAmount,
    contractSum: currencyCode.toUpperCase() === 'CNY'
      ? originalAmount
      : formValue.contractAmountRmb,
  };
}

export function getContractRmbAmount(formValue = {}) {
  return getContractAmountFields(formValue).contractSum;
}
