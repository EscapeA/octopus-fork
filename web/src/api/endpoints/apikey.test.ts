import assert from 'node:assert/strict';
import test from 'node:test';
import { registerSettingStoreGetter } from '../../lib/utils.ts';

import { formatAPIKeyStatsResponse, getAPIKeyCostProgress } from './apikey-format.ts';

// 单位风格跟随语言：本用例断言 K/M/B 风格，显式固定为英文（默认语言是简中 → 万/亿）。
registerSettingStoreGetter(() => 'en');

test('formatAPIKeyStatsResponse formats nested stats response without flattening info', () => {
    const formatted = formatAPIKeyStatsResponse({
        stats: {
            api_key_id: 7,
            input_token: 1200,
            output_token: 800,
            input_cost: 0.12,
            output_cost: 0.24,
            wait_time: 1500,
            request_success: 3,
            request_failed: 1,
            latency_p50: 100,
            latency_p95: 200,
            latency_p99: 300,
            ftut_avg: 50,
            ftut_p50: 45,
            ftut_p95: 80,
            ftut_p99: 120,
            histogram_lt_100: 10,
            histogram_100_500: 20,
            histogram_500_1k: 5,
            histogram_1k_5k: 2,
            histogram_gt_5k: 1,
        },
        info: {
            id: 7,
            name: 'dashboard key',
            api_key: 'sk-octopus-********1234',
            enabled: true,
            supported_models: 'gpt-4o',
        },
    });

    assert.equal(formatted.stats.api_key_id, 7);
    assert.equal(formatted.stats.total_token.formatted.value, '2.00');
    assert.equal(formatted.stats.total_token.formatted.unit, 'K');
    assert.equal(formatted.stats.request_count.formatted.value, '4.00');
    assert.equal(formatted.stats.request_count.formatted.unit, '');
    assert.equal(formatted.info.name, 'dashboard key');
    assert.equal(formatted.info.supported_models, 'gpt-4o');
});

// fork: 币种固定人民币（store 已删 chinaMode/exchangeRate），因此一律以 chinaMode=true、汇率=1 调用。
test('getAPIKeyCostProgress guards unlimited quota and clamps to range', () => {
    assert.equal(getAPIKeyCostProgress(50, 0, true, 1), 0);
    assert.equal(getAPIKeyCostProgress(Number.NaN, 100, true, 1), 0);
    assert.equal(getAPIKeyCostProgress(-1, 100, true, 1), 0);
    assert.equal(getAPIKeyCostProgress(150, 100, true, 1), 100);
    assert.ok(Math.abs(getAPIKeyCostProgress(25, 100, true, 1) - 25) < 1e-10);
});
test('API Key quota progress uses matching currency units at any exchange rate', () => {
    const usedCost = 14; // RMB 原值，无换算
    assert.ok(Math.abs(getAPIKeyCostProgress(usedCost, 100, true, 1) - 14) < 1e-10);
    // 非人民币模式（历史传入）依旧按原值处理，不因汇率放大/缩小
    assert.ok(Math.abs(getAPIKeyCostProgress(usedCost, 100, false, 7.2) - 14) < 1e-10);
});
