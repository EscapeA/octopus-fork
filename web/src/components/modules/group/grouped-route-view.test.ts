import assert from 'node:assert/strict';
import test from 'node:test';
import {
    buildGroupedRouteModelBuckets,
    UNGROUPED_BUCKET_ID,
    type GroupedRouteModelRow,
} from './grouped-view.ts';

function group_(id: number, name: string, items: { group_id: number; channel_id: number; model_name: string; priority: number; weight: number }[]): {
    id: number;
    name: string;
    endpoint_type: string;
    mode: number;
    match_regex: string;
    items: typeof items;
} {
    return { id, name, endpoint_type: 'chat', mode: 1, match_regex: '', items };
}

function channel(channelId: number, modelName: string, channelName = `#${channelId}`, enabled = true): GroupedRouteModelRow {
    return { id: `${channelId}-${modelName}`, name: modelName, channel_id: channelId, channel_name: channelName, enabled };
}

test('buildGroupedRouteModelBuckets preserves group order and sorts members by item priority', () => {
    const groups = [
        group_(2, 'B', [
            { group_id: 2, channel_id: 1, model_name: 'c', priority: 5, weight: 1 },
            { group_id: 2, channel_id: 1, model_name: 'a', priority: 1, weight: 1 },
        ]),
        group_(1, 'A', [
            { group_id: 1, channel_id: 1, model_name: 'b', priority: 2, weight: 1 },
        ]),
    ];
    const modelChannels = [channel(1, 'a', 'A'), channel(1, 'b', 'B'), channel(1, 'c', 'C')];

    const buckets = buildGroupedRouteModelBuckets(groups, modelChannels);

    assert.deepEqual(buckets.map((b) => b.id), [2, 1], 'group order should follow input');
    assert.deepEqual(
        buckets.flatMap((b) => b.models.map((m) => m.name)),
        ['a', 'c', 'b'],
        'members should be sorted by priority across preserved group order',
    );
});

test('model-channel rows not referenced by any group item appear once under ungrouped after real groups', () => {
    const groups = [group_(1, 'G', [{ group_id: 1, channel_id: 1, model_name: 'b', priority: 1, weight: 1 }])];
    const modelChannels = [channel(1, 'a', 'A'), channel(1, 'b', 'B'), channel(2, 'c', 'C')];

    const buckets = buildGroupedRouteModelBuckets(groups, modelChannels);
    const ungrouped = buckets.find((b) => b.id === UNGROUPED_BUCKET_ID);

    assert.ok(ungrouped, 'expected ungrouped bucket');
    assert.deepEqual(ungrouped.models.map((m) => m.name), ['a', 'c']);
});

test('searching by model name keeps the containing group bucket and filters its member rows', () => {
    const groups = [group_(1, 'G', [
        { group_id: 1, channel_id: 1, model_name: 'a', priority: 1, weight: 1 },
        { group_id: 1, channel_id: 1, model_name: 'b', priority: 2, weight: 1 },
    ])];
    const modelChannels = [channel(1, 'a', 'A'), channel(1, 'b', 'B')];

    const buckets = buildGroupedRouteModelBuckets(groups, modelChannels, 'b');

    assert.equal(buckets.length, 1, 'group bucket should remain');
    assert.deepEqual(buckets[0].models.map((m) => m.name), ['b']);
});

test('searching by group name keeps that group with full member list', () => {
    const groups = [
        group_(1, 'Alpha', [{ group_id: 1, channel_id: 1, model_name: 'a', priority: 1, weight: 1 }]),
        group_(2, 'Beta', [{ group_id: 2, channel_id: 1, model_name: 'b', priority: 1, weight: 1 }]),
    ];
    const modelChannels = [channel(1, 'a', 'A'), channel(1, 'b', 'B')];

    const buckets = buildGroupedRouteModelBuckets(groups, modelChannels, 'alpha');

    assert.equal(buckets.length, 1);
    assert.equal(buckets[0].id, 1);
    assert.deepEqual(buckets[0].models.map((m) => m.name), ['a']);
});
