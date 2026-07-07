import type { Group } from '../../../api/endpoints/group';
import type { LLMChannel } from '../../../api/endpoints/model';

export const UNGROUPED_BUCKET_ID = '__ungrouped__' as const;

export interface GroupedRouteModelRow {
    id: string;
    name: string;
    channel_id: number;
    channel_name: string;
    enabled: boolean;
    item_id?: number;
    priority?: number;
    weight?: number;
    group_id?: number;
}

export interface GroupedRouteModelBucket {
    id: number | string;
    key: string;
    kind: 'group' | 'ungrouped';
    name: string;
    category?: string;
    endpoint_type?: string;
    group?: Group;
    models: GroupedRouteModelRow[];
    matchedByGroupName: boolean;
}

export interface BuildGroupedRouteModelBucketsOptions {
    assignedGroups?: Group[];
}

function rowMatches(row: GroupedRouteModelRow, term: string) {
    return row.name.toLowerCase().includes(term) || row.channel_name.toLowerCase().includes(term);
}

function groupMatches(group: Group, term: string) {
    if (!term) return true;
    return [group.name, group.category, group.endpoint_type]
        .filter(Boolean)
        .some((value) => value!.toLowerCase().includes(term));
}

function buildChannelLookup(modelChannels: LLMChannel[]) {
    const lookup = new Map<string, LLMChannel>();
    for (const channelModel of modelChannels) {
        lookup.set(`${channelModel.channel_id}-${channelModel.name}`, channelModel);
    }
    return lookup;
}

function buildAssignedKeySet(groups: Group[]) {
    const assignedKeys = new Set<string>();
    for (const group of groups) {
        for (const item of group.items ?? []) {
            assignedKeys.add(`${item.channel_id}-${item.model_name}`);
        }
    }
    return assignedKeys;
}

export function buildGroupedRouteModelBuckets(
    groups: Group[] | undefined,
    modelChannels: LLMChannel[] | undefined,
    searchTerm = '',
    options: BuildGroupedRouteModelBucketsOptions = {},
): GroupedRouteModelBucket[] {
    const sourceGroups = groups ?? [];
    const sourceModelChannels = modelChannels ?? [];
    const term = searchTerm.trim().toLowerCase();
    const channelLookup = buildChannelLookup(sourceModelChannels);
    const assignedKeys = buildAssignedKeySet(options.assignedGroups ?? sourceGroups);
    const buckets: GroupedRouteModelBucket[] = [];

    for (const group of sourceGroups) {
        const matchedByGroupName = groupMatches(group, term);
        const models = [...(group.items ?? [])]
            .sort((left, right) => left.priority - right.priority)
            .map((item): GroupedRouteModelRow => {
                const key = `${item.channel_id}-${item.model_name}`;
                const channelModel = channelLookup.get(key);
                return {
                    id: key,
                    name: item.model_name,
                    channel_id: item.channel_id,
                    channel_name: channelModel?.channel_name ?? `#${item.channel_id}`,
                    enabled: channelModel?.enabled ?? true,
                    item_id: item.id,
                    priority: item.priority,
                    weight: item.weight,
                    group_id: group.id,
                };
            });
        const visibleModels = term && !matchedByGroupName ? models.filter((model) => rowMatches(model, term)) : models;

        if (!term || matchedByGroupName || visibleModels.length > 0) {
            buckets.push({
                id: group.id ?? group.name,
                key: `group-${group.id ?? group.name}`,
                kind: 'group',
                name: group.name,
                category: group.category,
                endpoint_type: group.endpoint_type,
                group,
                models: visibleModels,
                matchedByGroupName,
            });
        }
    }

    const ungroupedModels = sourceModelChannels
        .filter((channelModel) => !assignedKeys.has(`${channelModel.channel_id}-${channelModel.name}`))
        .map((channelModel): GroupedRouteModelRow => ({
            id: `${channelModel.channel_id}-${channelModel.name}`,
            name: channelModel.name,
            channel_id: channelModel.channel_id,
            channel_name: channelModel.channel_name,
            enabled: channelModel.enabled,
        }))
        .filter((model) => !term || rowMatches(model, term));

    if (ungroupedModels.length > 0) {
        buckets.push({
            id: UNGROUPED_BUCKET_ID,
            key: UNGROUPED_BUCKET_ID,
            kind: 'ungrouped',
            name: UNGROUPED_BUCKET_ID,
            models: ungroupedModels,
            matchedByGroupName: false,
        });
    }

    return buckets;
}
