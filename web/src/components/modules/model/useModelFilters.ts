'use client';

import { useMemo } from 'react';
import { useModelCapabilities } from '@/api/endpoints/model';
import { getModelIcon } from '@/lib/model-icons';
import { normalizeModelName, useNormalizeRulesVersion } from './normalize';
import type { ModelMarketItem } from '@/api/endpoints/model';

/** 模型支持的 endpoint 能力类型。 */
export type ModelCapabilityFilter = 'all' | 'chat' | 'embeddings' | 'rerank' | 'moderations' | 'image_generation' | 'audio_speech' | 'audio_transcription' | 'video_generation' | 'music_generation' | 'search';

export const MODEL_CAPABILITY_OPTIONS: ModelCapabilityFilter[] = [
    'all',
    'chat',
    'embeddings',
    'rerank',
    'image_generation',
    'audio_speech',
    'audio_transcription',
    'video_generation',
    'music_generation',
    'search',
];

/**
 * 推断模型原始厂商（如 Kimi / OpenAI / Anthropic），基于品牌前缀匹配。
 * 同一基础模型的不同命名变体（kimi-k2.5 / moonshotai/kimi-k2.5 / dmxapi-kimi-k2.5）
 * 都会归到同一厂商。
 */
export function inferModelProvider(name: string): string {
    return getModelIcon(name).label;
}

interface FilterInputs {
    items: ModelMarketItem[];
    searchTerm: string;
    capability: ModelCapabilityFilter;
    provider: string; // '' 表示全部
    dedupe: boolean;
}

interface FilterResult {
    /** 经过筛选（与可选去重）后的可见模型列表。 */
    visible: ModelMarketItem[];
    /** 当前数据中出现的所有厂商标签（用于渲染厂商筛选 chips）。 */
    providers: string[];
}

/**
 * 应用「能力 + 厂商 + 名称搜索 + 归一化去重」筛选链。
 *
 * 能力信息来自 `useModelCapabilities`，按模型名 join；
 * 厂商来自 `inferModelProvider`；归一化去重基于 `normalizeModelName`，
 * 同一归一名只保留第一条（按原顺序）。
 */
export function useModelFilters({ items, searchTerm, capability, provider, dedupe }: FilterInputs): FilterResult {
    const { data: capabilities } = useModelCapabilities();
    // 归一化规则保存在模块级变量里，需订阅其版本号才能在规则变更时让下方 useMemo 失效重算。
    const rulesVersion = useNormalizeRulesVersion();

    const capabilityByName = useMemo(() => {
        const map = new Map<string, string[]>();
        for (const cap of capabilities ?? []) {
            map.set(cap.name, cap.endpoints ?? []);
        }
        return map;
    }, [capabilities]);

    const providers = useMemo(() => {
        const set = new Set<string>();
        for (const item of items) {
            set.add(inferModelProvider(item.name));
        }
        return Array.from(set).sort((a, b) => a.localeCompare(b));
    }, [items]);

    const visible = useMemo(() => {
        // 归一化规则版本：normalizeModelName 读取模块级规则，需把它纳入依赖以在规则变更时重算。
        void rulesVersion;
        const term = searchTerm.toLowerCase().trim();
        let result = items;

        if (term) {
            result = result.filter((m) => m.name.toLowerCase().includes(term));
        }

        if (capability !== 'all') {
            result = result.filter((m) => {
                const endpoints = capabilityByName.get(m.name) ?? [];
                // chat 同时匹配 conversation 类端点（chat/deepseek/mimo/responses/messages）。
                if (capability === 'chat') {
                    return endpoints.some((e) => e === 'chat' || e === 'deepseek' || e === 'mimo' || e === 'responses' || e === 'messages');
                }
                return endpoints.includes(capability);
            });
        }

        if (provider) {
            result = result.filter((m) => inferModelProvider(m.name) === provider);
        }

        // dedupe 分支调用 normalizeModelName，依赖 rulesVersion 才能在规则变更时重算。
        if (dedupe) {
            const seen = new Set<string>();
            result = result.filter((m) => {
                const canonical = normalizeModelName(m.name);
                if (seen.has(canonical)) return false;
                seen.add(canonical);
                return true;
            });
        }

        return result;
    }, [items, searchTerm, capability, provider, dedupe, capabilityByName, rulesVersion]);

    return { visible, providers };
}
