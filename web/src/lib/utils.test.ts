import test from 'node:test';
import assert from 'node:assert/strict';

import { significantDecimalPlaces, formatMoney, registerSettingStoreGetter } from './utils.ts';

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

// ---- formatMoney: chinaMode unit suffix order (issue: "1.87¥万") ----
// All render sites compose value + unit in that order, so the currency
// symbol must be a suffix ("万元") not a midfix ("¥万").
function withChinaMode(chinaMode: boolean, exchangeRate: number, fn: () => void) {
    const prev = registerSettingStoreGetter(() => ({ chinaMode, exchangeRate }));
    try {
        fn();
    } finally {
        registerSettingStoreGetter(prev as never);
    }
}

test('formatMoney chinaMode small amount: value + "元"', () => {
    withChinaMode(true, 7.2, () => {
        // 1 USD * 7.2 = 7.20 RMB -> "7.20" + "元"
        const r = formatMoney(1).formatted;
        assert.equal(r.value, '7.20');
        assert.equal(r.unit, '元');
        assert.equal(r.value + r.unit, '7.20元');
    });
});

test('formatMoney chinaMode large amount: value + "万元"', () => {
    withChinaMode(true, 7.2, () => {
        // 18700 USD * 7.2 = 134640 RMB -> "13.46" + "万元"
        const r = formatMoney(18700).formatted;
        assert.equal(r.unit, '万元');
        assert.equal(r.value + r.unit, '13.46万元');
    });
});

test('formatMoney chinaMode huge amount: value + "亿元"', () => {
    withChinaMode(true, 7.2, () => {
        // 2e7 USD * 7.2 = 1.44e8 RMB -> "1.44" + "亿元"
        const r = formatMoney(20_000_000).formatted;
        assert.equal(r.unit, '亿元');
        assert.equal(r.value + r.unit, '1.44亿元');
    });
});
