'use client';

import { useMemo } from 'react';
import { useModelMarket } from '@/api/endpoints/model';
import { ModelItem } from './Item';
import { useSearchStore, useToolbarViewOptionsStore } from '@/components/modules/toolbar';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { sortModelMarketItems } from './sort';

export function Model() {
    const { data: market } = useModelMarket();
    const pageKey = 'model' as const;
    const searchTerm = useSearchStore((s) => s.getSearchTerm(pageKey));
    const layout = useToolbarViewOptionsStore((s) => s.getLayout(pageKey));
    const filter = useToolbarViewOptionsStore((s) => s.modelFilter);
    const modelSortMode = useToolbarViewOptionsStore((s) => s.modelSortMode);

    const sortedModels = useMemo(() => {
        const items = market?.items ?? [];
        return sortModelMarketItems(items, modelSortMode);
    }, [market, modelSortMode]);

    const visibleModels = useMemo(() => {
        const term = searchTerm.toLowerCase().trim();
        const byName = !term ? sortedModels : sortedModels.filter((m) => m.name.toLowerCase().includes(term));
        const hasPricing = (model: (typeof byName)[number]) =>
            model.input + model.output + model.cache_read + model.cache_write > 0;

        if (filter === 'priced') {
            return byName.filter(hasPricing);
        }
        if (filter === 'free') {
            return byName.filter((m) => !hasPricing(m));
        }

        return byName;
    }, [sortedModels, searchTerm, filter]);

    return (
        <section className="relative flex h-full min-h-0 flex-col" aria-label={pageKey}>
            <section className="relative flex min-h-0 flex-1 flex-col rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                <div className="relative min-h-0 flex-1">
                    {visibleModels.length > 0 ? (
                        <VirtualizedGrid
                            items={visibleModels}
                            layout={layout}
                            columns={{ default: 1, sm: 2, md: 2, lg: 3 }}
                            estimateItemHeight={228}
                            getItemKey={(model) => `model-${model.name}`}
                            renderItem={(model) => <ModelItem model={model} layout={layout} />}
                        />
                    ) : (
                        <div className="relative flex h-full min-h-[18rem] items-center justify-center overflow-hidden rounded-xl border border-dashed border-border/35 bg-card">
                            <div className="relative flex items-end gap-3">
                                <span className="h-24 w-16 rounded-lg border border-border/30 bg-card" />
                                <span className="h-28 w-20 rounded-xl border border-primary/18 bg-card" />
                                <span className="h-20 w-14 rounded-lg border border-border/30 bg-card" />
                            </div>
                        </div>
                    )}
                </div>
            </section>
        </section>
    );
}
