'use client';

import { Suspense, useState, useEffect, useCallback } from 'react';
import { CONTENT_MAP } from './config';
import type { RouteId } from './config';
import { useTranslations } from 'next-intl';
import { LoadingState } from '@/components/common/LoadingState';

/**
 * Keep-alive 路由渲染器。
 *
 * 已访问过的路由组件保持挂载（state 保留、query observer 不断开），
 * 仅用 CSS display 切换可见性。这避免了每次切 tab 都卸载/重挂载
 * 组件导致的重新请求、recharts 重建、motion 动画累积等问题。
 *
 * 非活跃路由设为 display:none 并加 aria-hidden + inert，对辅助技术隐藏。
 */
export function ContentLoader({ activeRoute }: { activeRoute: RouteId }) {
    const t = useTranslations('common');
    // 已挂载过的路由集合 —— 一旦访问就保持挂载
    const [mountedRoutes, setMountedRoutes] = useState<Set<RouteId>>(() => new Set([activeRoute]));

    // activeRoute 变化时，把新路由加入集合（首次访问触发挂载）
    useEffect(() => {
        setMountedRoutes((prev) => {
            if (prev.has(activeRoute)) return prev;
            const next = new Set(prev);
            next.add(activeRoute);
            return next;
        });
    }, [activeRoute]);

    const isActive = useCallback((routeId: RouteId) => routeId === activeRoute, [activeRoute]);

    const routes = Array.from(mountedRoutes);

    return (
        <div className="h-full min-h-0">
            {routes.map((routeId) => {
                const Component = CONTENT_MAP[routeId];
                if (!Component) return null;

                const active = isActive(routeId);
                return (
                    <div
                        key={routeId}
                        className="h-full min-h-0"
                        style={{ display: active ? undefined : 'none' }}
                        aria-hidden={!active}
                        inert={!active}
                    >
                        {active ? (
                            <Suspense fallback={<LoadingState />}>
                                <Component />
                            </Suspense>
                        ) : (
                            <Suspense fallback={null}>
                                <Component />
                            </Suspense>
                        )}
                    </div>
                );
            })}
            {routes.length === 0 && (
                <div className="flex items-center justify-center h-64">
                    <p className="text-muted-foreground">{t('routeNotFound', { route: activeRoute })}</p>
                </div>
            )}
        </div>
    );
}
