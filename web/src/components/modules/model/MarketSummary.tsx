'use client';

import { Boxes, Clock3, RefreshCw, RadioTower, Rows3 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { Button } from '@/components/ui/button';

type MarketSummaryValue = {
    model_count: number;
    coverage_count: number;
    unique_channel_count: number;
    average_latency_ms: number;
    last_update_time: string;
};

function formatLastUpdate(value: string, fallback: string) {
    if (!value) return fallback;

    const date = new Date(value);
    if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1) {
        return fallback;
    }

    return date.toLocaleString();
}

export function ModelMarketSummary({
    summary,
    onRefresh,
    isRefreshing,
}: {
    summary: MarketSummaryValue;
    onRefresh: () => void;
    isRefreshing: boolean;
}) {
    const t = useTranslations('model');
    const lastUpdateLabel = formatLastUpdate(summary.last_update_time, t('summary.neverUpdated'));

    const metrics = [
        {
            key: 'models',
            icon: Boxes,
            label: t('summary.modelCount'),
            value: summary.model_count.toLocaleString(),
        },
        {
            key: 'coverage',
            icon: Rows3,
            label: t('summary.coverage'),
            value: summary.coverage_count.toLocaleString(),
        },
        {
            key: 'unique',
            icon: RadioTower,
            label: t('summary.uniqueChannels'),
            value: summary.unique_channel_count.toLocaleString(),
        },
        {
            key: 'latency',
            icon: Clock3,
            label: t('summary.averageLatency'),
            value: summary.average_latency_ms > 0 ? `${summary.average_latency_ms}ms` : '—',
        },
    ];

    return (
        <section className="relative overflow-hidden rounded-xl border border-border/35 bg-card p-3.5 text-card-foreground md:p-4.5">
            <div className="relative flex flex-col gap-3">
                <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap sm:items-center sm:justify-end">
                    <div className="flex items-center gap-2 rounded-lg border-border/30 bg-card px-3 py-2 text-xs text-muted-foreground sm:text-sm">
                        <Clock3 className="h-4 w-4 text-primary" />
                        <span className="truncate">{t('summary.lastUpdate')}: {lastUpdateLabel}</span>
                    </div>
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={onRefresh}
                        disabled={isRefreshing}
                        className="h-10 rounded-lg border-border/30 bg-card px-3.5"
                    >
                        <RefreshCw className={`mr-2 h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
                        {isRefreshing ? t('summary.refreshing') : t('summary.refresh')}
                    </Button>
                </div>

                <div className="grid grid-cols-2 gap-2 xl:grid-cols-4">
                    {metrics.map((metric) => (
                        <div key={metric.key} className="group relative overflow-hidden rounded-lg border border-border/30 bg-card px-3 py-2.5 transition-[transform,border-color] duration-300 hover:-translate-y-0.5 hover:border-primary/18">
                            <div className="relative flex items-center gap-2 text-[11px] text-muted-foreground md:text-sm">
                                <metric.icon className="h-4 w-4 text-primary" />
                                <span>{metric.label}</span>
                            </div>
                            <div className="relative mt-1.5 text-[1.8rem] font-semibold tracking-tight md:mt-2 md:text-[1.75rem]">{metric.value}</div>
                        </div>
                    ))}
                </div>
            </div>
        </section>
    );
}
