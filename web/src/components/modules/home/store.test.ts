import assert from 'node:assert/strict';
import test from 'node:test';
import { normalizeOverviewRange, statsRefreshIntervalMs, STATS_REFRESH_INTERVAL_OPTIONS } from './store.ts';

test('normalizeOverviewRange defaults to 7d when value is invalid', () => {
    assert.equal(normalizeOverviewRange('unexpected'), '7d');
});

test('normalizeOverviewRange preserves supported values', () => {
    assert.equal(normalizeOverviewRange('7d'), '7d');
    assert.equal(normalizeOverviewRange('30d'), '30d');
    assert.equal(normalizeOverviewRange('90d'), '90d');
});

test('statsRefreshIntervalMs converts interval to milliseconds', () => {
    assert.equal(statsRefreshIntervalMs('5s'), 5000);
    assert.equal(statsRefreshIntervalMs('10s'), 10000);
    assert.equal(statsRefreshIntervalMs('15s'), 15000);
    assert.equal(statsRefreshIntervalMs('30s'), 30000);
    assert.equal(statsRefreshIntervalMs('60s'), 60000);
});

test('statsRefreshIntervalMs returns false for off to disable polling', () => {
    assert.equal(statsRefreshIntervalMs('off'), false);
});

test('STATS_REFRESH_INTERVAL_OPTIONS covers 5s 10s 15s 30s 60s off', () => {
    assert.deepEqual([...STATS_REFRESH_INTERVAL_OPTIONS], ['5s', '10s', '15s', '30s', '60s', 'off']);
});
