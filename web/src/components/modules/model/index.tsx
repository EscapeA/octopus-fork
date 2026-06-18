'use client';

import { useMemo, useState } from 'react';
import { MotionConfig } from 'motion/react';
import { useModelMarket } from '@/api/endpoints/model';
import { useTranslations } from 'next-intl';
import { ModelItem } from './Item';
import { MobileModelItem } from './MobileModelItem';
import { EndpointsView } from './EndpointsView';
import { useIsMobile } from '@/hooks/use-mobile';
import { useSearchStore, useToolbarViewOptionsStore } from '@/components/modules/toolbar';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { sortModelMarketItems } from './sort';
import { useModelFilters, MODEL_CAPABILITY_OPTIONS, type ModelCapabilityFilter } from './useModelFilters';
import { cn } from '@/lib/utils';

export function Model() {
    const t = useTranslations('model');
    const tEndpoints = useTranslations('endpoints');
    const tFilter = useTranslations('modelFilter');
    const { data: market } = useModelMarket();
    const isMobile = useIsMobile();
    const [view, setView] = useState<'market' | 'endpoints'>('market');
    const pageKey = 'model' as const;
    const searchTerm = useSearchStore((s) => s.getSearchTerm(pageKey));
    const layout = useToolbarViewOptionsStore((s) => s.getLayout(pageKey));
    const filter = useToolbarViewOptionsStore((s) => s.modelFilter);
    const modelSortMode = useToolbarViewOptionsStore((s) => s.modelSortMode);
    const modelLatencyUnit = useToolbarViewOptionsStore((s) => s.modelLatencyUnit);

    // 多维筛选本地状态：能力 / 厂商 / 归一化去重。
    const [capability, setCapability] = useState<ModelCapabilityFilter>('all');
    const [provider, setProvider] = useState<string>('');
    const [dedupe, setDedupe] = useState(false);

    const sortedModels = useMemo(() => {
        const items = market?.items ?? [];
        return sortModelMarketItems(items, modelSortMode);
    }, [market, modelSortMode]);

    const pricedFiltered = useMemo(() => {
        const hasPricing = (model: (typeof sortedModels)[number]) =>
            model.input + model.output + model.cache_read + model.cache_write > 0;
        if (filter === 'priced') return sortedModels.filter(hasPricing);
        if (filter === 'free') return sortedModels.filter((m) => !hasPricing(m));
        return sortedModels;
    }, [sortedModels, filter]);

    const { visible: visibleModels, providers } = useModelFilters({
        items: pricedFiltered,
        searchTerm,
        capability,
        provider,
        dedupe,
    });
    const hasAnyModel = (market?.items.length ?? 0) > 0;

    return (
        <section className="relative flex h-full min-h-0 flex-col gap-3 overflow-y-auto overscroll-contain rounded-t-xl pb-3 sm:gap-4 sm:pb-4 md:pb-4" aria-label={pageKey}>
            <div className="flex items-center gap-0.5 self-start rounded-lg border border-border/35 bg-card p-0.5 sm:gap-1 sm:p-1">
                <button
                    type="button"
                    onClick={() => setView('market')}
                    className={cn(
                        'rounded-md px-3 py-1.5 text-xs font-medium transition-colors sm:px-4 sm:py-2 sm:text-sm',
                        view === 'market'
                            ? 'bg-primary text-primary-foreground shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    {t('marketTitle')}
                </button>
                <button
                    type="button"
                    onClick={() => setView('endpoints')}
                    className={cn(
                        'rounded-md px-3 py-1.5 text-xs font-medium transition-colors sm:px-4 sm:py-2 sm:text-sm',
                        view === 'endpoints'
                            ? 'bg-primary text-primary-foreground shadow-sm'
                            : 'text-muted-foreground hover:text-foreground',
                    )}
                >
                    {tEndpoints('title')}
                </button>
            </div>

            {view === 'endpoints' ? (
                <EndpointsView />
            ) : (
            <>
            {/* 多维筛选条：能力 / 厂商 / 归一化去重 */}
            <div className="flex flex-col gap-2 rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                <div className="flex flex-wrap items-center gap-1.5">
                    <span className="mr-1 text-xs font-medium text-muted-foreground">{tFilter('capability')}</span>
                    {MODEL_CAPABILITY_OPTIONS.map((value) => {
                        const active = capability === value;
                        const labelKey = value === 'all' ? 'all' : value;
                        return (
                            <button
                                key={value}
                                type="button"
                                onClick={() => setCapability(value)}
                                aria-pressed={active}
                                className={cn(
                                    'rounded-md border px-2.5 py-1 text-xs font-medium transition-colors',
                                    active
                                        ? 'border-primary/40 bg-primary/10 text-primary'
                                        : 'border-border bg-card text-muted-foreground hover:text-foreground',
                                )}
                            >
                                {value === 'all' ? tFilter('all') : tFilter(`capability.${labelKey}`)}
                            </button>
                        );
                    })}
                </div>

                <div className="flex flex-wrap items-center gap-1.5">
                    <span className="mr-1 text-xs font-medium text-muted-foreground">{tFilter('provider')}</span>
                    <button
                        type="button"
                        onClick={() => setProvider('')}
                        aria-pressed={provider === ''}
                        className={cn(
                            'rounded-md border px-2.5 py-1 text-xs font-medium transition-colors',
                            provider === ''
                                ? 'border-primary/40 bg-primary/10 text-primary'
                                : 'border-border bg-card text-muted-foreground hover:text-foreground',
                        )}
                    >
                        {tFilter('all')}
                    </button>
                    {providers.map((value) => {
                        const active = provider === value;
                        return (
                            <button
                                key={value}
                                type="button"
                                onClick={() => setProvider(active ? '' : value)}
                                aria-pressed={active}
                                className={cn(
                                    'rounded-md border px-2.5 py-1 text-xs font-medium transition-colors',
                                    active
                                        ? 'border-primary/40 bg-primary/10 text-primary'
                                        : 'border-border bg-card text-muted-foreground hover:text-foreground',
                                )}
                            >
                                {value}
                            </button>
                        );
                    })}
                </div>

                <div className="flex items-center gap-2">
                    <button
                        type="button"
                        onClick={() => setDedupe((v) => !v)}
                        aria-pressed={dedupe}
                        className={cn(
                            'inline-flex items-center gap-1.5 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors',
                            dedupe
                                ? 'border-primary/40 bg-primary/10 text-primary'
                                : 'border-border bg-card text-muted-foreground hover:text-foreground',
                        )}
                    >
                        {tFilter('dedupe')}
                    </button>
                    <span className="text-xs text-muted-foreground">{tFilter('dedupeHint')}</span>
                </div>
            </div>

            {visibleModels.length > 0 ? (
                <section className="relative flex min-h-0 flex-1 flex-col rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                    <div className="relative min-h-0 flex-1">
                        {isMobile ? (
                            <VirtualizedGrid
                                items={visibleModels}
                                layout="list"
                                columns={{ default: 1 }}
                                estimateItemHeight={132}
                                getItemKey={(model) => `m-model-${model.name}`}
                                renderItem={(model) => <MobileModelItem model={model} latencyUnit={modelLatencyUnit} />}
                                bottomPaddingClassName="pb-3 md:pb-4"
                            />
                        ) : (
                            <MotionConfig transition={{ layout: { duration: 0 } }}>
                                <VirtualizedGrid
                                    items={visibleModels}
                                    layout={layout}
                                    columns={{ default: 1, sm: 2, md: 2, lg: 3 }}
                                    estimateItemHeight={228}
                                    getItemKey={(model) => `model-${model.name}`}
                                    renderItem={(model) => <ModelItem model={model} layout={layout} latencyUnit={modelLatencyUnit} />}
                                    bottomPaddingClassName="pb-3 md:pb-4"
                                />
                            </MotionConfig>
                        )}
                    </div>
                </section>
            ) : (
                <section className="rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                    <div className="relative flex min-h-[18rem] items-center justify-center overflow-hidden rounded-xl border border-dashed border-border/35 bg-card py-6">
                        <div className="relative flex flex-col items-center gap-4 px-6 text-center">
                            <div className="flex items-end gap-3">
                                <span className="h-24 w-16 rounded-lg border border-border/30 bg-card" />
                                <span className="h-28 w-20 rounded-xl border border-primary/18 bg-card" />
                                <span className="h-20 w-14 rounded-lg border border-border/30 bg-card" />
                            </div>
                            <p className="text-sm text-muted-foreground">
                                {hasAnyModel ? t('empty') : t('emptyAll')}
                            </p>
                        </div>
                    </div>
                </section>
            )}
            </>
            )}
        </section>
    );
}
