import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

/**
 * Lazily read the current locale from the persisted setting store.
 * Avoids a hard import-cycle: consumers in components already use the store
 * directly; utils.ts only needs the value at call-time.
 */
type LocaleGetter = () => string;

let _localeGetter: LocaleGetter | null = null;

/** Registers the locale getter and returns the previous one (tests restore with it). */
export function registerSettingStoreGetter(getter: LocaleGetter): LocaleGetter | null {
  const prev = _localeGetter;
  _localeGetter = getter;
  return prev;
}

/**
 * 单位风格跟随语言：简中/繁中用 万/亿，其它语言用 K/M/B。
 * 币种固定人民币（¥）——金额原值即人民币，不存在任何汇率换算。
 */
function isChineseLocale(): boolean {
  const locale = _localeGetter ? _localeGetter() : 'zh-Hans';
  return locale.startsWith('zh');
}

/** 单位风格判断：简中/繁中 → 万/亿（图表刻度等直接判断的场景用）。 */
export function prefersChineseUnits(): boolean {
  return isChineseLocale();
}

function formatNumber(num: number | undefined, compare: number[], units: string[]): { value: string, unit: string } {
  if (num === undefined) return { value: "0.00", unit: units[0] };
  else if (num >= compare[0]) return { value: (num / compare[0]).toFixed(2), unit: units[1] };
  else if (num >= compare[1]) return { value: (num / compare[1]).toFixed(2), unit: units[2] };
  else if (num >= compare[2]) return { value: (num / compare[2]).toFixed(2), unit: units[3] };
  else if (num >= compare[3]) return { value: (num / compare[3]).toFixed(2), unit: units[4] };
  else return { value: (num).toFixed(2), unit: units[5] };
}

/**
 * Format a count (token count, request count, etc.).
 * 简中/繁中：万 (10k) / 亿 (100M) only — 千 is skipped；其它语言：K/M/B。
 */
export function formatCount(num: number | undefined): { raw: number, formatted: { value: string, unit: string } } {
  const v = num ?? 0;
  if (isChineseLocale()) {
    if (v >= 100_000_000) return { raw: v, formatted: { value: (v / 100_000_000).toFixed(2), unit: '亿' } };
    if (v >= 10_000)      return { raw: v, formatted: { value: (v / 10_000).toFixed(2), unit: '万' } };
    return { raw: v, formatted: { value: v.toLocaleString(), unit: '' } };
  }
  return {
    raw: v,
    formatted: formatNumber(v, [1000000000, 1000000, 1000, 1], ['', 'B', 'M', 'K', '', '']),
  };
}

/**
 * Format a monetary amount. 金额本身即人民币（¥/M tokens 计价），原值直出不做换算。
 * 简中/繁中：元/万元/亿元；其它语言：¥/K¥/M¥/B¥。
 */
export function formatMoney(num: number | undefined): { raw: number, formatted: { value: string, unit: string } } {
  const v = num ?? 0;
  if (isChineseLocale()) {
    if (v >= 100_000_000) return { raw: v, formatted: { value: (v / 100_000_000).toFixed(2), unit: '亿元' } };
    if (v >= 10_000)      return { raw: v, formatted: { value: (v / 10_000).toFixed(2), unit: '万元' } };
    return { raw: v, formatted: { value: v.toFixed(2), unit: '元' } };
  }
  return {
    raw: v,
    formatted: formatNumber(v, [1000000000, 1000000, 1000, 1], ['¥', 'B¥', 'M¥', 'K¥', '¥', '¥']),
  };
}

export function formatTime(ms: number | undefined): { raw: number, formatted: { value: string, unit: string } } {
  return {
    raw: ms ?? 0,
    formatted: formatNumber(ms, [86400000, 3600000, 60000, 1000], ['', 'd', 'h', 'm', 's', 'ms']),
  };
}

// significantDecimalPlaces returns how many decimals to render for a numeric
// display string, trimming trailing zeros so counts like "5.00" show as "5"
// while genuine precision such as "1.5" or "1.23" is preserved.
export function significantDecimalPlaces(value: string | number | undefined): number {
  if (typeof value !== 'string') return 0;
  const fracPart = value.split('.')[1];
  if (!fracPart) return 0;
  return Math.min(2, fracPart.replace(/0+$/, '').length);
}
