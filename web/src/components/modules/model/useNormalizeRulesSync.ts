'use client';

import { useEffect } from 'react';
import { useSettingList, SettingKey } from '@/api/endpoints/setting';
import { setNormalizeRules, type ExplicitMapping } from './normalize';

/**
 * 从 DB Setting 读取归一化规则并注入 normalize 模块。
 *
 * 归一化规则（路由前缀 / 功能后缀 / 显式变体→基准名映射）由用户在设置页维护或
 * 由离线 AI 分析后导入，持久化在 DB。这里在设置列表就绪后把自定义规则注入运行时
 * 覆盖；前缀/后缀空数组回退到内置默认，显式映射空数组表示不启用。
 */
export function useNormalizeRulesSync() {
    const { data: settings } = useSettingList();

    useEffect(() => {
        if (!settings) return;
        const getStrings = (key: string): string[] => {
            const raw = settings.find((s) => s.key === key)?.value ?? '[]';
            try {
                const parsed = JSON.parse(raw);
                return Array.isArray(parsed) ? parsed.filter((x) => typeof x === 'string') : [];
            } catch {
                return [];
            }
        };
        const getMappings = (key: string): ExplicitMapping[] => {
            const raw = settings.find((s) => s.key === key)?.value ?? '[]';
            try {
                const parsed = JSON.parse(raw);
                if (!Array.isArray(parsed)) return [];
                return parsed
                    .filter((x): x is ExplicitMapping =>
                        x && typeof x === 'object'
                        && typeof x.variant === 'string' && x.variant.trim() !== ''
                        && typeof x.canonical === 'string' && x.canonical.trim() !== '')
                    .map((x) => ({ variant: x.variant, canonical: x.canonical }));
            } catch {
                return [];
            }
        };
        setNormalizeRules({
            routerPrefixes: getStrings(SettingKey.ModelNormalizeRouterPrefixes),
            functionalSuffixes: getStrings(SettingKey.ModelNormalizeFunctionalSuffixes),
            explicitMappings: getMappings(SettingKey.ModelNormalizeExplicitMappings),
        });
    }, [settings]);
}
