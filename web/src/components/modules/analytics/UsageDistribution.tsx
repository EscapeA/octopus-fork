'use client';

import { useMemo, useState } from 'react';
import { PieChart as PieChartIcon } from 'lucide-react';
import { useTranslations } from 'next-intl';
import {
    useAnalyticsUtilization,
    useAnalyticsChannelModel,
    type AnalyticsRange,
    type AnalyticsModelBreakdownItem,
    type AnalyticsChannelModelItem,
} from '@/api/endpoints/analytics';
import { ObservatorySection, QueryState } from './shared';
import { formatCount, formatMoney, cn } from '@/lib/utils';
import { Tabs, TabsList, TabsTrigger } from '@/components/animate-ui/components/animate/tabs';
import { useAnalyticsCacheTtl } from './cache-context';

type Metric = 'request_count' | 'total_cost' | 'total_tokens';

const TOP_N = 8;

function pickMetricValue(item: AnalyticsModelBreakdownItem | AnalyticsChannelModelItem, metric: Metric): number {
    return item[metric] ?? 0;
}

function formatMetricValue(value: number, metric: Metric): string {
    if (metric === 'total_cost') {
        const f = formatMoney(value).formatted;
        return `${f.value}${f.unit}`;
    }
    const f = formatCount(value).formatted;
    return `${f.value}${f.unit}`;
}

export function UsageDistribution({ range }: { range: AnalyticsRange }) {
    const t = useTranslations('analytics');
    const cacheTtl = useAnalyticsCacheTtl();
    const [metric, setMetric] = useState<Metric>('request_count');
    const [dimension, setDimension] = useState<'model' | 'channel-model'>('model');

    const { data: utilization, isLoading: isUtilLoading, error: utilError } = useAnalyticsUtilization(range, cacheTtl);
    const { data: channelModel, isLoading: isCmLoading, error: cmError } = useAnalyticsChannelModel(range, undefined, cacheTtl);

    const isLoading = dimension === 'model' ? isUtilLoading : isCmLoading;
    const error = dimension === 'model' ? utilError : cmError;

    const rows = useMemo(() => {
        if (dimension === 'model') {
            return (utilization?.model_breakdown ?? []) as AnalyticsModelBreakdownItem[];
        }
        return (channelModel ?? []) as AnalyticsChannelModelItem[];
    }, [dimension, utilization, channelModel]);

    const chartData = useMemo(() => {
        const withValues = rows
            .map((item) => {
                const value = pickMetricValue(item, metric);
                const name = dimension === 'model'
                    ? (item as AnalyticsModelBreakdownItem).model_name
                    : `${(item as AnalyticsChannelModelItem).channel_name || `#${(item as AnalyticsChannelModelItem).channel_id}`} / ${(item as AnalyticsChannelModelItem).model_name}`;
                return { name: name || '—', value };
            })
            .filter((d) => d.value > 0);

        const sorted = withValues.sort((a, b) => b.value - a.value);
        const top = sorted.slice(0, TOP_N);
        const rest = sorted.slice(TOP_N);
        if (rest.length > 0) {
            const restSum = rest.reduce((acc, d) => acc + d.value, 0);
            top.push({ name: t('usageDistribution.others'), value: restSum });
        }
        return top;
    }, [rows, metric, dimension, t]);

    const total = useMemo(() => chartData.reduce((acc, d) => acc + d.value, 0), [chartData]);

    const isEmpty = !chartData || chartData.length === 0 || total <= 0;

    const maxBarWidth = 100;
    const palette = [
        'var(--chart-1)',
        'var(--chart-2)',
        'var(--chart-3)',
        'var(--chart-4)',
        'var(--chart-5)',
        'oklch(0.62 0.10 200)',
        'oklch(0.70 0.10 145)',
        'oklch(0.60 0.10 20)',
        'oklch(0.68 0.08 300)',
    ];

    return (
        <ObservatorySection
            eyebrow={t('cards.channelModel.title')}
            title={t('usageDistribution.title')}
            description={t('usageDistribution.description')}
            icon={PieChartIcon}
            actions={
                <div className="flex flex-wrap items-center gap-2">
                    <Tabs value={dimension} onValueChange={(v) => setDimension(v as 'model' | 'channel-model')}>
                        <TabsList className="rounded-lg border border-border/30 bg-card p-1">
                            <TabsTrigger value="model" className="text-xs">{t('usageDistribution.byModel')}</TabsTrigger>
                            <TabsTrigger value="channel-model" className="text-xs">{t('usageDistribution.byChannelModel')}</TabsTrigger>
                        </TabsList>
                    </Tabs>
                    <Tabs value={metric} onValueChange={(v) => setMetric(v as Metric)}>
                        <TabsList className="rounded-lg border border-border/30 bg-card p-1">
                            <TabsTrigger value="request_count" className="text-xs">{t('usageDistribution.metric.requests')}</TabsTrigger>
                            <TabsTrigger value="total_cost" className="text-xs">{t('usageDistribution.metric.cost')}</TabsTrigger>
                            <TabsTrigger value="total_tokens" className="text-xs">{t('usageDistribution.metric.tokens')}</TabsTrigger>
                        </TabsList>
                    </Tabs>
                </div>
            }
        >
            <QueryState
                loading={isLoading}
                error={error}
                empty={isEmpty}
                emptyLabel={isLoading ? t('states.loading') : t('usageDistribution.empty')}
            >
                <div className="space-y-2.5">
                    {chartData.map((d, idx) => {
                        const percent = total > 0 ? (d.value / total) * 100 : 0;
                        const width = total > 0 ? Math.max((d.value / total) * maxBarWidth, 1.5) : 0;
                        return (
                            <div key={`${d.name}-${idx}`} className="rounded-lg border border-border/25 bg-card px-3 py-2.5">
                                <div className="flex items-center justify-between gap-3 text-xs">
                                    <span className="truncate font-medium" title={d.name}>{d.name}</span>
                                    <span className="shrink-0 tabular-nums text-muted-foreground">
                                        {formatMetricValue(d.value, metric)}
                                        <span className="ml-2 text-foreground/80">{percent.toFixed(1)}%</span>
                                    </span>
                                </div>
                                <div className="mt-1.5 h-2 w-full overflow-hidden rounded-full bg-muted/50">
                                    <div
                                        className="h-full rounded-full transition-all"
                                        style={{ width: `${width}%`, backgroundColor: palette[idx % palette.length] }}
                                    />
                                </div>
                            </div>
                        );
                    })}
                    {chartData.length > 0 && (
                        <div className={cn('pt-1 text-right text-xs text-muted-foreground')}>
                            {t('usageDistribution.total')}: {formatMetricValue(total, metric)}
                        </div>
                    )}
                </div>
            </QueryState>
        </ObservatorySection>
    );
}
