'use client';

import { useMemo, useState, useCallback } from 'react';

/**
 * 排序维度。issue #114：新增 success_rate 排序。
 * - requests：请求数（默认）
 * - success_rate：成功率
 * - cost：费用
 */
export type AnalyticsSortKey = 'requests' | 'success_rate' | 'cost';

export type AnalyticsSortOrder = 'desc' | 'asc';

interface SortableItem {
    request_count: number;
    success_rate: number;
    total_cost: number;
}

/**
 * 对分析 breakdown 项做前端排序。数据已含 success_rate / request_count /
 * total_cost，无需后端改动。同值回退到请求数降序，再回退到名称（由调用方处理）。
 */
export function useBreakdownSort<T extends SortableItem>(items: T[], defaultKey: AnalyticsSortKey = 'requests') {
    const [sortKey, setSortKey] = useState<AnalyticsSortKey>(defaultKey);
    const [sortOrder, setSortOrder] = useState<AnalyticsSortOrder>('desc');

    const toggleOrder = useCallback(() => {
        setSortOrder((prev) => (prev === 'desc' ? 'asc' : 'desc'));
    }, []);

    const sorted = useMemo(() => {
        const result = [...items];
        const dir = sortOrder === 'desc' ? -1 : 1;
        result.sort((a, b) => {
            let cmp = 0;
            switch (sortKey) {
                case 'success_rate':
                    cmp = a.success_rate - b.success_rate;
                    break;
                case 'cost':
                    cmp = a.total_cost - b.total_cost;
                    break;
                default:
                    cmp = a.request_count - b.request_count;
                    break;
            }
            if (cmp !== 0) return cmp * dir;
            // 同值回退：请求数降序（无论主排序方向，请求多的优先更直观）
            if (a.request_count !== b.request_count) return b.request_count - a.request_count;
            return 0;
        });
        return result;
    }, [items, sortKey, sortOrder]);

    return { sorted, sortKey, setSortKey, sortOrder, toggleOrder };
}
