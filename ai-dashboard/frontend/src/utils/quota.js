
/**
 * @param {string} str
 * @returns {Boolean}
 */
export function quotaStrToNumber(quotaStr) {
  if (quotaStr === 'NA') {
    return 2147483647
  }
  return Number(quotaStr)
}

export function quotaNumberToStr(quota) {
  if (quota === 2147483647) {
    return 'NA'
  }
  return String(quota)
}
