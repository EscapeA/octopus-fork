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
 */

// 已知的路由商 / 平台前缀（出现在模型名开头，与底层模型无关）。
const ROUTER_PREFIXES = [
    'dmxapi-',
    'agent-',
    'openai-',   // openai-/anthropic- 等路由前缀（不区分大小写匹配）
    'anthropic-',
];

// 已知的功能性后缀（与模型本体无关，常见于渠道商二次命名）。
const FUNCTIONAL_SUFFIXES = [
    '-cc',
    '-fast',
    '-thinking',
    '-preview',
    '-beta',
    '-latest',
];

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
    let result = name.trim();

    // 1. 剥离路径前缀：取最后一个 `/` 之后的部分。
    const slashIndex = result.lastIndexOf('/');
    if (slashIndex >= 0) {
        result = result.slice(slashIndex + 1);
    }

    // 2. 剥离已知的路由商前缀（大小写不敏感）。
    const lower = result.toLowerCase();
    for (const prefix of ROUTER_PREFIXES) {
        if (lower.startsWith(prefix)) {
            result = result.slice(prefix.length);
            break;
        }
    }

    // 3. 剥离已知的功能性后缀（大小写不敏感，循环处理叠加后缀）。
    let changed = true;
    while (changed) {
        changed = false;
        const currentLower = result.toLowerCase();
        for (const suffix of FUNCTIONAL_SUFFIXES) {
            if (currentLower.endsWith(suffix) && result.length > suffix.length) {
                result = result.slice(0, -suffix.length);
                changed = true;
                break;
            }
        }
    }

    // 4. 规范化为小写，便于聚合。
    return result.toLowerCase();
}
