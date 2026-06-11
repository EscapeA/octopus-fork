'use client';

import { memo, useMemo, useState } from 'react';
import { useModelCapabilities, type ModelCapability } from '@/api/endpoints/model';
import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { useTranslations } from 'next-intl';

// 桌面行与表头共用的列宽模板，保证虚拟化行与表头对齐。
const CAPABILITY_GRID_COLS = 'minmax(0,1.6fr) minmax(0,2fr) 7rem 7rem';

function CapabilityBadge({ endpoint, t }: { endpoint: string; t: ReturnType<typeof useTranslations> }) {
    if (endpoint === '*') {
        return <Badge variant="secondary" className="text-[10px]">{t('capabilities.autoEndpoint')}</Badge>;
    }
    return <Badge variant="outline" className="font-mono text-[10px]">{endpoint}</Badge>;
}

const CapabilityRow = memo(function CapabilityRow({ cap, t }: { cap: ModelCapability; t: ReturnType<typeof useTranslations> }) {
    return (
        <div
            className="grid items-center border-b border-border/30 transition-colors hover:bg-muted/40"
            style={{ gridTemplateColumns: CAPABILITY_GRID_COLS }}
        >
            <div className="px-3 py-2.5 text-sm font-medium sm:px-4">{cap.name}</div>
            <div className="px-3 py-2.5 sm:px-4">
                <div className="flex flex-wrap gap-1">
                    {cap.endpoints.map((ep) => (
                        <CapabilityBadge key={ep} endpoint={ep} t={t} />
                    ))}
                </div>
            </div>
            <div className="px-3 py-2.5 text-center sm:px-4">
                {cap.conversation ? (
                    <span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                        {t('capabilities.yes')}
                    </span>
                ) : (
                    <span className="text-xs text-muted-foreground">—</span>
                )}
            </div>
            <div className="px-3 py-2.5 text-center sm:px-4">
                {cap.available ? (
                    <span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                        {t('capabilities.active')}
                    </span>
                ) : (
                    <span className="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-amber-500" />
                        {t('capabilities.down')}
                    </span>
                )}
            </div>
        </div>
    );
});

const CapabilityCard = memo(function CapabilityCard({ cap, t }: { cap: ModelCapability; t: ReturnType<typeof useTranslations> }) {
    return (
        <div className="rounded-lg border border-border/30 bg-card p-3 transition-colors hover:border-primary/18">
            <div className="flex items-start justify-between gap-2">
                <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium text-foreground">{cap.name}</div>
                    <div className="mt-1.5 flex flex-wrap gap-1">
                        {cap.endpoints.map((ep) => (
                            <CapabilityBadge key={ep} endpoint={ep} t={t} />
                        ))}
                    </div>
                </div>
                {cap.available ? (
                    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2 py-0.5 text-[0.65rem] text-emerald-600 dark:text-emerald-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                        {t('capabilities.active')}
                    </span>
                ) : (
                    <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-amber-500/20 bg-amber-500/10 px-2 py-0.5 text-[0.65rem] text-amber-600 dark:text-amber-400">
                        <span className="inline-block h-1.5 w-1.5 rounded-full bg-amber-500" />
                        {t('capabilities.down')}
                    </span>
                )}
            </div>
            {cap.conversation && !cap.endpoints.includes('*') && (
                <div className="mt-2 inline-flex items-center gap-1 text-[0.68rem] text-emerald-600 dark:text-emerald-400">
                    <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-500" />
                    {t('capabilities.conversation')}
                </div>
            )}
        </div>
    );
});

export function CapabilitiesPanel() {
    const t = useTranslations('model');
    const { data: capabilities, isLoading, error, refetch } = useModelCapabilities();
    const [search, setSearch] = useState('');

    const filtered = useMemo(() => {
        if (!capabilities) return [];
        const term = search.toLowerCase().trim();
        if (!term) return capabilities;
        return capabilities.filter((c) => c.name.toLowerCase().includes(term));
    }, [capabilities, search]);

    // 在一次遍历内统计三个计数，避免在渲染体内对 filtered 反复执行 filter
    // （输入搜索词时每次按键都会触发，行数多时成本明显）。
    const { autoCount, convCount, nonConvCount } = useMemo(() => {
        let auto = 0;
        let conv = 0;
        for (const c of filtered) {
            const isAuto = c.endpoints.includes('*');
            if (isAuto) {
                auto++;
            } else if (c.conversation) {
                conv++;
            }
        }
        return {
            autoCount: auto,
            convCount: conv,
            nonConvCount: filtered.length - auto - conv,
        };
    }, [filtered]);

    if (isLoading) {
        return (
            <section className="rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                <LoadingState />
            </section>
        );
    }

    if (error) {
        return (
            <section className="rounded-xl border border-border/35 bg-card p-3 text-card-foreground md:p-4">
                <ErrorState message={error.message} onRetry={() => refetch()} />
            </section>
        );
    }

    return (
        <section className="relative flex h-full min-h-0 flex-col gap-3 sm:gap-4">
            <div className="flex flex-nowrap items-center gap-2 overflow-x-auto rounded-xl border border-border/35 bg-card px-3 py-2 text-xs text-muted-foreground sm:flex-wrap sm:gap-4 sm:px-4 sm:py-3 sm:text-sm">
                <span className="whitespace-nowrap">{t('capabilities.total')}: <strong className="text-foreground">{filtered.length}</strong></span>
                <span className="whitespace-nowrap">{t('capabilities.autoEndpoint')}: <strong className="text-foreground">{autoCount}</strong></span>
                <span className="whitespace-nowrap">{t('capabilities.conversation')}: <strong className="text-foreground">{convCount}</strong></span>
                <span className="whitespace-nowrap">{t('capabilities.nonConversation')}: <strong className="text-foreground">{nonConvCount}</strong></span>
            </div>

            <div className="flex min-h-0 flex-1 flex-col rounded-xl border border-border/35 bg-card">
                <div className="flex items-center gap-2 border-b border-border/30 px-3 py-2 sm:px-4 sm:py-2.5">
                    <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground sm:h-4 sm:w-4" />
                    <input
                        type="text"
                        placeholder={t('capabilities.searchPlaceholder')}
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        className="min-w-0 flex-1 bg-transparent text-xs outline-none placeholder:text-muted-foreground sm:text-sm"
                    />
                </div>

                {filtered.length === 0 ? (
                    <div className="px-4 py-12 text-center text-sm text-muted-foreground">
                        {search ? t('capabilities.emptySearch') : t('capabilities.empty')}
                    </div>
                ) : (
                    <>
                        {/* Mobile: virtualized card list */}
                        <div className="min-h-0 flex-1 sm:hidden">
                            <VirtualizedGrid
                                items={filtered}
                                layout="list"
                                columns={{ default: 1 }}
                                estimateItemHeight={92}
                                gap={8}
                                getItemKey={(cap) => `cap-card-${cap.name}`}
                                renderItem={(cap) => <CapabilityCard cap={cap} t={t} />}
                                bottomPaddingClassName="pb-3"
                            />
                        </div>

                        {/* Desktop: sticky header + virtualized rows */}
                        <div className="hidden min-h-0 flex-1 flex-col sm:flex">
                            <div
                                className="grid border-b border-border/30 bg-muted/50 text-xs uppercase text-muted-foreground"
                                style={{ gridTemplateColumns: CAPABILITY_GRID_COLS }}
                            >
                                <div className="px-4 py-2.5 font-medium">{t('capabilities.model')}</div>
                                <div className="px-4 py-2.5 font-medium">{t('capabilities.endpoints')}</div>
                                <div className="px-4 py-2.5 text-center font-medium">{t('capabilities.conversation')}</div>
                                <div className="px-4 py-2.5 text-center font-medium">{t('capabilities.status')}</div>
                            </div>
                            <div className="min-h-0 flex-1">
                                <VirtualizedGrid
                                    items={filtered}
                                    layout="list"
                                    columns={{ default: 1 }}
                                    estimateItemHeight={45}
                                    gap={0}
                                    getItemKey={(cap) => `cap-row-${cap.name}`}
                                    renderItem={(cap) => <CapabilityRow cap={cap} t={t} />}
                                    bottomPaddingClassName="pb-0"
                                />
                            </div>
                        </div>
                    </>
                )}
            </div>

            <p className="px-1 text-[0.68rem] text-muted-foreground sm:text-xs">
                {t('capabilities.note')}
            </p>
        </section>
    );
}
