import assert from 'node:assert/strict';
import test from 'node:test';

import { normalizeModelName } from './normalize.ts';

test('normalizeModelName keeps a plain model name as-is (lowercased)', () => {
    assert.equal(normalizeModelName('kimi-k2.5'), 'kimi-k2.5');
    assert.equal(normalizeModelName('GPT-5.2'), 'gpt-5.2');
});

test('normalizeModelName strips path prefixes such as provider/@cf/agent', () => {
    assert.equal(normalizeModelName('moonshotai/kimi-k2.5'), 'kimi-k2.5');
    assert.equal(normalizeModelName('@cf/moonshotai/kimi-k2.5'), 'kimi-k2.5');
    assert.equal(normalizeModelName('agent/kimi-k2.5'), 'kimi-k2.5');
});

test('normalizeModelName strips router prefixes such as dmxapi-/agent-', () => {
    assert.equal(normalizeModelName('dmxapi-kimi-k2.5'), 'kimi-k2.5');
    assert.equal(normalizeModelName('agent-kimi-k2.5'), 'kimi-k2.5');
});

test('normalizeModelName strips functional suffixes such as -cc/-fast/-thinking', () => {
    assert.equal(normalizeModelName('kimi-k2.5-cc'), 'kimi-k2.5');
    assert.equal(normalizeModelName('Kimi-K2.5-CC'), 'kimi-k2.5');
    assert.equal(normalizeModelName('gpt-5.2-fast'), 'gpt-5.2');
    assert.equal(normalizeModelName('claude-3.5-thinking'), 'claude-3.5');
});

test('normalizeModelName strips stacked suffixes', () => {
    assert.equal(normalizeModelName('kimi-k2.5-cc-fast'), 'kimi-k2.5');
});

test('normalizeModelName combines path + router prefix + suffix', () => {
    assert.equal(normalizeModelName('@cf/moonshotai/dmxapi-kimi-k2.5-cc'), 'kimi-k2.5');
});

test('normalizeModelName returns empty string for empty input', () => {
    assert.equal(normalizeModelName(''), '');
    assert.equal(normalizeModelName('   '), '');
});
