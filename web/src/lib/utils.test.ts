import test from 'node:test';
import assert from 'node:assert/strict';

import { significantDecimalPlaces, formatMoney, formatCount, prefersChineseUnits, registerSettingStoreGetter } from './utils.ts';

test('significantDecimalPlaces trims trailing zeros for integer-like counts', () => {
    assert.equal(significantDecimalPlaces('5.00'), 0);
    assert.equal(significantDecimalPlaces('0.00'), 0);
});

test('significantDecimalPlaces keeps one decimal when only the second digit is zero', () => {
    assert.equal(significantDecimalPlaces('1.50'), 1);
    assert.equal(significantDecimalPlaces('12.30'), 1);
});

test('significantDecimalPlaces keeps two decimals when both are meaningful', () => {
    assert.equal(significantDecimalPlaces('1.23'), 2);
    assert.equal(significantDecimalPlaces('95.05'), 2);
});

test('significantDecimalPlaces returns 0 for integers and non-strings', () => {
    assert.equal(significantDecimalPlaces('5'), 0);
    assert.equal(significantDecimalPlaces('123'), 0);
    assert.equal(significantDecimalPlaces(5), 0);
    assert.equal(significantDecimalPlaces(undefined), 0);
});

// ---- 币种固定人民币：金额原值直出，不存在汇率换算 ----
// All render sites compose value + unit in that order, so the currency symbol
// must live in the unit (suffix) rather than being prefixed.
function withLocale(locale: string, fn: () => void) {
    const prev = registerSettingStoreGetter(() => locale);
    try {
        fn();
    } finally {
        if (prev) registerSettingStoreGetter(prev);
    }
}

test('formatMoney 不做汇率换算：1 元就是 1 元（回归：美元 ×7.2）', () => {
    withLocale('zh-Hans', () => {
        const r = formatMoney(1).formatted;
        assert.equal(r.value, '1.00');
        assert.equal(r.unit, '元');
        assert.equal(r.value + r.unit, '1.00元');
    });
});

test('formatMoney 简中：万元 / 亿元 单位', () => {
    withLocale('zh-Hans', () => {
        let r = formatMoney(18700).formatted;
        assert.equal(r.value + r.unit, '1.87万元');
        r = formatMoney(144_000_000).formatted;
        assert.equal(r.value + r.unit, '1.44亿元');
    });
});

test('formatMoney 繁中同样使用万/亿', () => {
    withLocale('zh-Hant', () => {
        const r = formatMoney(18700).formatted;
        assert.equal(r.value + r.unit, '1.87万元');
    });
});

test('formatMoney 英文：¥ 作为单位后缀', () => {
    withLocale('en', () => {
        let r = formatMoney(1).formatted;
        assert.equal(r.value + r.unit, '1.00¥');
        r = formatMoney(20_000_000).formatted;
        assert.equal(r.value + r.unit, '20.00M¥');
    });
});

test('formatCount 单位风格跟随语言', () => {
    withLocale('zh-Hans', () => {
        const r = formatCount(12345).formatted;
        assert.equal(r.value + r.unit, '1.23万');
    });
    withLocale('en', () => {
        const r = formatCount(12345).formatted;
        assert.equal(r.value + r.unit, '12.35K');
    });
});

test('prefersChineseUnits 跟随语言', () => {
    withLocale('zh-Hans', () => assert.equal(prefersChineseUnits(), true));
    withLocale('zh-Hant', () => assert.equal(prefersChineseUnits(), true));
    withLocale('en', () => assert.equal(prefersChineseUnits(), false));
});
