'use client';

import { createContext, useContext } from 'react';
import type { AnalyticsCacheTtl } from '@/api/endpoints/analytics';

/**
 * AnalyticsCacheTtlContext 在分析中心页面级提供缓存 TTL 设置，
 * 避免向每个子组件 prop-drilling。index.tsx 持有 state 并通过
 * Provider 注入；子组件用 useAnalyticsCacheTtl() 读取。
 */
const AnalyticsCacheTtlContext = createContext<AnalyticsCacheTtl>('30s');

export function AnalyticsCacheTtlProvider({
    value,
    children,
}: {
    value: AnalyticsCacheTtl;
    children: React.ReactNode;
}) {
    return <AnalyticsCacheTtlContext.Provider value={value}>{children}</AnalyticsCacheTtlContext.Provider>;
}

export function useAnalyticsCacheTtl(): AnalyticsCacheTtl {
    return useContext(AnalyticsCacheTtlContext);
}
