'use client';

import { useMemo } from 'react';
import { Coins, Database, Gauge, HardDrive, Layers3 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import type { OpsProviderPromptCacheProviderItem, OpsProviderPromptCacheSummary } from '@/api/endpoints/ops';
import { useOpsCacheStatus } from '@/api/endpoints/ops';
import { MetricCard, QueryState, formatPercent, formatUnixTime } from '@/components/modules/analytics/shared';
import { formatProviderPromptCacheCount, getProviderPromptCacheTrendTokens } from './cache-format';
import { ChartContainer, ChartTooltip, ChartTooltipContent } from '@/components/ui/chart';
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts';

type CacheTranslations = (key: string) => string;

function formatCount(n: number | undefined) {
    const value = n ?? 0;
    if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1)}M`;
    if (value >= 1_000) return `${(value / 1_000).toFixed(1)}K`;
    return String(value);
}

function formatCurrency(value: number | undefined) {
    return (value ?? 0).toFixed(4);
}

function ProviderPromptCacheRow({
    provider,
}: {
    provider: OpsProviderPromptCacheProviderItem;
}) {
    return (
        <tr className="border-b border-border/40 align-top last:border-0">
            <td className="px-3 py-3">
                <div className="text-sm font-semibold">{provider.channel_name}</div>
            </td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">{formatCount(provider.request_count)}</td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">
                {formatPercent(provider.cache_rate).formatted.value}
                {formatPercent(provider.cache_rate).formatted.unit}
            </td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">{formatCount(provider.cache_read_tokens)}</td>
            <td className="px-3 py-3 text-right text-sm tabular-nums">{formatCount(provider.cache_write_tokens)}</td>
            <td className="px-3 py-3 text-right text-sm font-semibold tabular-nums">
                ${formatCurrency(provider.estimated_cost_saved)}
            </td>
        </tr>
    );
}

function formatChartData(trend: Array<{ timestamp: number; cache_read_tokens?: number; cache_write_tokens?: number; request_count?: number }>) {
    return trend.map((point) => ({
        time: formatUnixTime(point.timestamp),
        cache_read_tokens: point.cache_read_tokens ?? 0,
        cache_write_tokens: point.cache_write_tokens ?? 0,
        request_count: point.request_count ?? 0,
    }));
}

function buildChartConfig(t: CacheTranslations) {
    return {
        cache_read_tokens: {
            label: t('cache.providerPrompt.metrics.cacheReadTokens'),
            color: 'hsl(var(--chart-1))',
        },
        cache_write_tokens: {
            label: t('cache.providerPrompt.metrics.cacheWriteTokens'),
            color: 'hsl(var(--chart-2))',
        },
    };
}


function TrendTooltipValue({ value, name, t }: { value: number; name: string; t: CacheTranslations }) {
    const label = name === 'cache_read_tokens'
        ? t('cache.providerPrompt.metrics.cacheReadTokens')
        : t('cache.providerPrompt.metrics.cacheWriteTokens');
    return (
        <div className="flex items-center justify-between gap-4">
            <span className="text-muted-foreground">{label}</span>
            <span className="font-mono font-medium tabular-nums">{formatCount(value)}</span>
        </div>
    );
}

function ProviderPromptCacheView({
    data,
    t,
}: {
    data: OpsProviderPromptCacheSummary;
    t: CacheTranslations;
}) {
    const trend = data.trend ?? [];
    const readTokens = formatProviderPromptCacheCount(data.cache_read_tokens);
    const writeTokens = formatProviderPromptCacheCount(data.cache_write_tokens);
    const hasTrendActivity = trend.some((item) => item.request_count > 0 || item.cache_read_tokens > 0 || item.cache_write_tokens > 0);
    const missingUsageHint = `${t('cache.providerPrompt.providers.empty')} (${data.parsed_log_count}/${data.sampled_log_count})`;

    const chartData = useMemo(() => formatChartData(trend), [trend]);
    const chartConfig = useMemo(() => buildChartConfig(t), [t]);

    return (
        <div className="space-y-4">
            <div className="grid grid-cols-2 gap-3 md:gap-4 md:grid-cols-2 xl:grid-cols-5">
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheRate')}
                    value={formatPercent(data.cache_rate).formatted.value}
                    unit={formatPercent(data.cache_rate).formatted.unit}
                    icon={Gauge}
                    accentClassName="bg-emerald-500/10 text-emerald-600"
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheReuseRatio')}
                    value={formatPercent(data.cache_reuse_ratio).formatted.value}
                    unit={formatPercent(data.cache_reuse_ratio).formatted.unit}
                    icon={Layers3}
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheReadTokens')}
                    value={readTokens.value}
                    unit={readTokens.unit}
                    icon={HardDrive}
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.cacheWriteTokens')}
                    value={writeTokens.value}
                    unit={writeTokens.unit}
                    icon={Database}
                />
                <MetricCard
                    title={t('cache.providerPrompt.metrics.estimatedCostSaved')}
                    value={formatCurrency(data.estimated_cost_saved)}
                    unit="$"
                    icon={Coins}
                    accentClassName="bg-chart-4/10 text-chart-4"
                />
            </div>

            <article className="min-w-0 rounded-xl border border-border/60 bg-card p-4">
                <div className="grid min-w-0 gap-6">
                    <div className="min-w-0 space-y-3">
                        <div>
                            <h4 className="text-sm font-semibold">{t('cache.providerPrompt.providers.title')}</h4>
                            <p className="mt-1 text-sm leading-6 text-muted-foreground">
                                {t('cache.providerPrompt.providers.description')}
                            </p>
                        </div>
                        {data.providers.length === 0 ? (
                            <div className="rounded-lg border border-dashed border-border/40 px-4 py-6 text-center text-sm text-muted-foreground">
                                {t('cache.providerPrompt.providers.empty')}
                            </div>
                        ) : (
                            <div className="overflow-x-auto rounded-xl border border-border/60">
                                <table className="w-full min-w-[640px] text-left md:min-w-[720px]">
                                    <thead>
                                        <tr className="border-b border-border/40 bg-muted/30">
                                            <th className="px-3 py-2.5 text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.name')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.requests')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.cacheRate')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.cacheReadTokens')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.cacheWriteTokens')}
                                            </th>
                                            <th className="px-3 py-2.5 text-right text-xs font-medium text-muted-foreground">
                                                {t('cache.providerPrompt.providers.columns.estimatedCostSaved')}
                                            </th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {data.providers.map((provider) => (
                                            <ProviderPromptCacheRow key={provider.channel_id} provider={provider} />
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        )}
                    </div>

                    <div className="min-w-0 space-y-3">
                        <div>
                            <h4 className="text-sm font-semibold">{t('cache.providerPrompt.trend.title')}</h4>
                            <p className="mt-1 text-sm leading-6 text-muted-foreground">
                                {t('cache.providerPrompt.trend.description')}
                            </p>
                        </div>
                        <div className="min-w-0 rounded-xl border border-border/60 bg-card p-4">
                            {!data.usage_signal_available ? (
                                <div className="rounded-lg border border-dashed border-border/40 px-4 py-6 text-center text-sm text-muted-foreground">
                                    <p>{missingUsageHint}</p>
                                </div>
                            ) : !hasTrendActivity ? (
                                <div className="rounded-lg border border-dashed border-border/40 px-4 py-6 text-center text-sm text-muted-foreground">
                                    {t('cache.providerPrompt.providers.empty')}
                                </div>
                            ) : (
                                <ChartContainer config={chartConfig} className="h-[16rem] w-full">
                                    <BarChart
                                        data={chartData}
                                        margin={{ top: 16, right: 8, bottom: 0, left: 0 }}
                                    >
                                        <CartesianGrid strokeDasharray="3 3" vertical={false} />
                                        <XAxis
                                            dataKey="time"
                                            tickLine={false}
                                            axisLine={false}
                                            tick={{ fontSize: 11 }}
                                        />
                                        <YAxis
                                            tickLine={false}
                                            axisLine={false}
                                            tick={{ fontSize: 11 }}
                                            tickFormatter={(v: number) => formatCount(v)}
                                        />
                                        <ChartTooltip
                                            cursor={{ fill: 'hsl(var(--foreground) / 0.06)' }}
                                            content={
                                                <ChartTooltipContent
                                                    indicator="dot"
                                                    nameKey="time"
                                                    labelFormatter={(_value, payload) => {
                                                        const time = payload?.[0]?.payload?.time;
                                                        return typeof time === 'string' ? <div className="font-semibold">{time}</div> : null;
                                                    }}
                                                    formatter={(value, name) => (
                                                        <TrendTooltipValue value={value as number} name={name as string} t={t} />
                                                    )}
                                                />
                                            }
                                        />
                                        <Bar
                                            dataKey="cache_read_tokens"
                                            fill="var(--color-cache_read_tokens)"
                                            radius={[4, 4, 0, 0]}
                                            maxBarSize={48}
                                        />
                                        <Bar
                                            dataKey="cache_write_tokens"
                                            fill="var(--color-cache_write_tokens)"
                                            radius={[4, 4, 0, 0]}
                                            maxBarSize={48}
                                        />
                                    </BarChart>
                                </ChartContainer>
                            )}
                        </div>
                    </div>
                </div>
            </article>
        </div>
    );
}

export function Cache() {
    const t = useTranslations('ops');
    const { data, isLoading, error } = useOpsCacheStatus();

    return (
        <section className="rounded-xl border border-border/35 bg-card p-5 text-card-foreground">
            <div className="mb-4 space-y-1">
                <h3 className="text-base font-semibold">{t('tabs.cache')}</h3>
                <p className="text-sm leading-6 text-muted-foreground">
                    {t('cache.providerPrompt.description')}
                </p>
            </div>

            <QueryState
                loading={isLoading}
                error={error}
                empty={!data}
                emptyLabel={t('states.loading')}
            >
                {data ? <ProviderPromptCacheView data={data.provider_prompt_cache} t={t} /> : null}
            </QueryState>
        </section>
    );
}
