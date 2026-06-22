/**
 * 模型名归一化工具
 *
 * 同一基础模型在不同渠道/路由商下会有多种命名变体，例如：
 *   kimi-k2.5
 *   @cf/moonshotai/kimi-k2.5
 *   dmxapi-kimi-k2.5
 *   moonshotai/kimi-k2.5
 *   agent/kimi-k2.5
 *   kimi-k2.5-cc
 *
 * 这些都应归一为 `kimi-k2.5`，用于「按模型聚合/去重」视图。
 *
 * 归一化只用于分组展示与去重，不会修改原始模型名。
 *
 * 规则来源分三层（按优先级）：
 *   1. 显式映射（runtimeExplicitMappings）—— 来自 DB Setting，点对点 variant→canonical，
 *      通常由离线 AI 分析产出。命中即返回，不再走前缀/后缀剥离。
 *   2. 内置默认（BUILTIN_ROUTER_PREFIXES / BUILTIN_FUNCTIONAL_SUFFIXES）—— 编译期兜底。
 *   3. 运行时覆盖（setNormalizeRules 注入）—— 来自 DB Setting，用户可在设置页增删。
 * 注入为空数组 / null 时回退到内置默认，保证未配置时行为与历史一致。
 *
 * 规则变化通过订阅机制对外广播，使依赖归一化结果的 React memo 能在规则更新时
 * 失效重算（见 useNormalizeRulesVersion）。
 */

import { useSyncExternalStore } from 'react';

export interface NormalizeExplicitMapping {
    variant: string;
    canonical: string;
}

// 内置的默认路由商 / 平台前缀（出现在模型名开头，与底层模型无关）。
const BUILTIN_ROUTER_PREFIXES = [
    'dmxapi-',
    'agent-',
    'openai-',   // openai-/anthropic- 等路由前缀（不区分大小写匹配）
    'anthropic-',
];

// 内置的默认功能性后缀（与模型本体无关，常见于渠道商二次命名）。
const BUILTIN_FUNCTIONAL_SUFFIXES = [
    '-cc',
    '-fast',
    '-thinking',
    '-preview',
    '-beta',
    '-latest',
];

// 显式变体→基准名映射的元素结构。
export interface ExplicitMapping {
    variant: string;
    canonical: string;
}

// 运行时覆盖规则。null 表示未配置，使用内置默认（显式映射无内置默认）。
let runtimeRouterPrefixes: string[] | null = null;
let runtimeFunctionalSuffixes: string[] | null = null;
// variant(小写) → canonical。空表示未配置显式映射。
let runtimeExplicitMappings: Map<string, string> | null = null;

// 规则版本号：每次 setNormalizeRules 改变规则时自增，供订阅者判断是否需要重算。
let rulesVersion = 0;
// 订阅者集合，规则变化时逐个通知。
const subscribers = new Set<() => void>();

function notifyRulesChange() {
    rulesVersion += 1;
    for (const sub of subscribers) {
        try {
            sub();
        } catch {
            /* 单个订阅者异常不影响其他订阅者 */
        }
    }
}

/**
 * 注入运行时归一化规则（来自 DB Setting）。
 * 传入空数组或 null/undefined 表示清除对应覆盖，前缀/后缀回退到内置默认。
 * 显式映射无内置默认，空表示不启用。
 *
 * 规则发生实质变化时会自增版本号并通知订阅者，使依赖该规则的 React memo
 * 失效重算（避免规则已更新但视图仍按旧规则去重的 bug）。
 */
export function setNormalizeRules(rules?: {
    routerPrefixes?: string[] | null;
    functionalSuffixes?: string[] | null;
    explicitMappings?: ExplicitMapping[] | null;
}) {
    const nextRouterPrefixes = rules?.routerPrefixes && rules.routerPrefixes.length > 0
        ? rules.routerPrefixes
        : null;
    const nextFunctionalSuffixes = rules?.functionalSuffixes && rules.functionalSuffixes.length > 0
        ? rules.functionalSuffixes
        : null;
    const mappings = rules?.explicitMappings && rules.explicitMappings.length > 0
        ? rules.explicitMappings
        : null;
    const nextExplicitMappings = mappings
        ? new Map(mappings.map((m) => [m.variant.toLowerCase(), m.canonical]))
        : null;

    // 只有规则实质变化时才广播，避免设置列表 refetch 触发的无效重算。
    if (runtimeRouterPrefixes === nextRouterPrefixes
        && runtimeFunctionalSuffixes === nextFunctionalSuffixes
        && runtimeExplicitMappings === nextExplicitMappings) {
        return;
    }
    runtimeRouterPrefixes = nextRouterPrefixes;
    runtimeFunctionalSuffixes = nextFunctionalSuffixes;
    runtimeExplicitMappings = nextExplicitMappings;
    notifyRulesChange();
}

function subscribeRulesVersion(callback: () => void): () => void {
    subscribers.add(callback);
    return () => {
        subscribers.delete(callback);
    };
}

function getRulesVersionSnapshot(): number {
    return rulesVersion;
}

/**
 * 订阅归一化规则版本号的 React hook。
 *
 * 规则保存在模块级可变变量里，本身在 React 体系之外。本 hook 通过
 * useSyncExternalStore 把版本号接入 React，使依赖归一化结果的 useMemo
 * 能把返回值加进依赖数组，规则一变即失效重算。
 */
export function useNormalizeRulesVersion(): number {
    return useSyncExternalStore(subscribeRulesVersion, getRulesVersionSnapshot, getRulesVersionSnapshot);
}

/**
 * 返回当前生效的规则列表（运行时覆盖 ?? 内置默认）。
 * 供设置页展示与分析工具使用。
 */
export function getActiveRouterPrefixes(): string[] {
    return runtimeRouterPrefixes ?? BUILTIN_ROUTER_PREFIXES;
}

export function getActiveFunctionalSuffixes(): string[] {
    return runtimeFunctionalSuffixes ?? BUILTIN_FUNCTIONAL_SUFFIXES;
}

/**
 * 返回当前生效的显式映射列表（无内置默认，未配置时返回空数组）。
 */
export function getActiveExplicitMappings(): ExplicitMapping[] {
    if (!runtimeExplicitMappings) return [];
    return Array.from(runtimeExplicitMappings, ([variant, canonical]) => ({ variant, canonical }));
}

/**
 * 将模型名归一化为基础模型名。
 *
 * 处理步骤：
 * 1. 取最后一个 `/` 之后的部分（剥离 `provider/`、`@cf/org/`、`agent/` 等路径前缀）。
 * 2. 剥离已知的路由商前缀（大小写不敏感）。
 * 3. 剥离已知的功能性后缀（大小写不敏感，可能多个叠加）。
 * 4. 规范化为小写，便于跨命名变体聚合。
 *
 * @example
 * normalizeModelName('kimi-k2.5')                       // 'kimi-k2.5'
 * normalizeModelName('@cf/moonshotai/kimi-k2.5')        // 'kimi-k2.5'
 * normalizeModelName('dmxapi-kimi-k2.5')                // 'kimi-k2.5'
 * normalizeModelName('moonshotai/kimi-k2.5')            // 'kimi-k2.5'
 * normalizeModelName('agent/kimi-k2.5')                 // 'kimi-k2.5'
 * normalizeModelName('kimi-k2.5-cc')                    // 'kimi-k2.5'
 * normalizeModelName('Kimi-K2.5-CC')                    // 'kimi-k2.5'
 */
export function normalizeModelName(name: string): string {
    if (!name) return '';
    const trimmed = name.trim();

    // 0. 显式映射优先：AI 离线分析产出的点对点映射（变体→基准名），
    //    命中即直接返回（小写）。匹配对原始名大小写不敏感。
    const explicit = getActiveExplicitMappings();
    if (explicit.length > 0) {
        const lowerTrimmed = trimmed.toLowerCase();
        for (const m of explicit) {
            if (m.variant.toLowerCase() === lowerTrimmed) {
                return m.canonical.toLowerCase();
            }
        }
    }

    let result = trimmed;

    const routerPrefixes = getActiveRouterPrefixes();
    const functionalSuffixes = getActiveFunctionalSuffixes();

    // 1. 剥离路径前缀：取最后一个 `/` 之后的部分。
    const slashIndex = result.lastIndexOf('/');
    if (slashIndex >= 0) {
        result = result.slice(slashIndex + 1);
    }

    // 2. 剥离已知的路由商前缀（大小写不敏感）。
    const lower = result.toLowerCase();
    for (const prefix of routerPrefixes) {
        if (lower.startsWith(prefix.toLowerCase())) {
            result = result.slice(prefix.length);
            break;
        }
    }

    // 3. 剥离已知的功能性后缀（大小写不敏感，循环处理叠加后缀）。
    let changed = true;
    while (changed) {
        changed = false;
        const currentLower = result.toLowerCase();
        for (const suffix of functionalSuffixes) {
            const s = suffix.toLowerCase();
            if (currentLower.endsWith(s) && result.length > s.length) {
                result = result.slice(0, -s.length);
                changed = true;
                break;
            }
        }
    }

    // 4. 规范化为小写，便于聚合。
    return result.toLowerCase();
}
