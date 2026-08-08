'use client';

import { motion } from 'motion/react';
import { Activity, DollarSign, HardDrive, ShieldCheck, Waves } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useStatsToday } from '@/api/endpoints/stats';
import { useOpsCacheStatus, type OpsProviderPromptCacheTrendPoint } from '@/api/endpoints/ops';
import { useHomeStatsRefreshMs } from './store';
import { StatsRefreshControls } from './refresh-controls';
import { AnimatedNumber } from '@/components/common/AnimatedNumber';
import { EASING } from '@/lib/animations/fluid-transitions';
import { formatCount, formatMoney, formatTime } from '@/lib/utils';

export function HomeHero() {
    const t = useTranslations('home.hero');
    const statsRefreshMs = useHomeStatsRefreshMs();
    const { data: statsToday } = useStatsToday({ refetchIntervalMs: statsRefreshMs });
    const { data: cacheStatus } = useOpsCacheStatus();

    const requestCount = (statsToday?.request_success ?? 0) + (statsToday?.request_failed ?? 0);
    const successCount = statsToday?.request_success ?? 0;
    const totalCost = (statsToday?.input_cost ?? 0) + (statsToday?.output_cost ?? 0);
    const totalWaitTime = statsToday?.wait_time ?? 0;
    const successRate = requestCount > 0 ? (successCount / requestCount) * 100 : 0;
    const avgWait = requestCount > 0 ? totalWaitTime / requestCount : 0;

    // 今日缓存复用 Tokens：对 provider prompt cache 的 24h 趋势点求和（timestamp >= 本地时区今日 0 点）。
    // trend 按 UTC 整点对齐（后端 opsHourlyWindowStart），+8 时区下今日 0 点恰为整点边界，过滤无污染。
    const cacheTrend = cacheStatus?.provider_prompt_cache?.trend ?? ([] as OpsProviderPromptCacheTrendPoint[]);
    const todayCacheReadTokens = cacheTrend
        .filter((point) => point.timestamp >= Math.floor(new Date(new Date().getFullYear(), new Date().getMonth(), new Date().getDate()).getTime() / 1000))
        .reduce((sum, point) => sum + (point.cache_read_tokens ?? 0), 0);
    const todayCacheRead = formatCount(todayCacheReadTokens);

    const signals = [
        {
            key: 'requests',
            label: t('signals.requests'),
            value: formatCount(requestCount).formatted.value,
            unit: formatCount(requestCount).formatted.unit,
            icon: Activity,
            accent: 'bg-emerald-500/10 text-emerald-700',
        },
        {
            key: 'successRate',
            label: t('signals.successRate'),
            value: successRate.toFixed(2),
            unit: '%',
            icon: ShieldCheck,
            accent: 'bg-primary/10 text-primary',
        },
        {
            key: 'cost',
            label: t('signals.cost'),
            value: formatMoney(totalCost).formatted.value,
            unit: formatMoney(totalCost).formatted.unit,
            icon: DollarSign,
            accent: 'bg-amber-500/10 text-amber-700',
        },
        {
            key: 'cacheReadTokens',
            label: t('signals.cacheReadTokens'),
            value: todayCacheRead.formatted.value,
            unit: todayCacheRead.formatted.unit,
            icon: HardDrive,
            accent: 'bg-sky-500/10 text-sky-700',
        },
    ];

    const metrics = [
        {
            key: 'avgWait',
            label: t('metrics.avgWait'),
            value: formatTime(avgWait).formatted.value,
            unit: formatTime(avgWait).formatted.unit,
        },
        {
            key: 'successful',
            label: t('metrics.successful'),
            value: formatCount(successCount).formatted.value,
            unit: formatCount(successCount).formatted.unit,
        },
        {
            key: 'tokens',
            label: t('metrics.tokens'),
            value: formatCount((statsToday?.input_token ?? 0) + (statsToday?.output_token ?? 0)).formatted.value,
            unit: formatCount((statsToday?.input_token ?? 0) + (statsToday?.output_token ?? 0)).formatted.unit,
        },
        {
            key: 'costPerReq',
            label: t('metrics.costPerRequest'),
            value: formatMoney(requestCount > 0 ? totalCost / requestCount : 0).formatted.value,
            unit: formatMoney(requestCount > 0 ? totalCost / requestCount : 0).formatted.unit,
        },
    ];

    return (
        <motion.section
            className="relative rounded-xl border border-border bg-card p-5 text-card-foreground md:p-6 xl:p-7"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EASING.easeOutExpo }}
        >
            <div className="grid gap-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(20rem,0.88fr)]">
                <div className="space-y-5">
                    <div className="space-y-3">
                        <div className="flex flex-wrap items-center justify-between gap-3 sm:gap-4">
                            <div className="flex items-center gap-3 sm:gap-4">
                                <div className="grid h-11 w-11 sm:h-14 sm:w-14 shrink-0 place-items-center overflow-hidden rounded-lg border border-border bg-card text-primary">
                                    <Waves className="h-5 w-5 sm:h-6 sm:w-6" strokeWidth={1.5} />
                                </div>
                                <div className="space-y-1">
                                    <h1 className="text-[1.65rem] font-semibold tracking-tight sm:text-2xl md:text-3xl lg:text-4xl">{t('title')}</h1>
                                    {t('subtitle') ? (
                                        <p className="text-sm leading-6 text-muted-foreground md:text-base">{t('subtitle')}</p>
                                    ) : null}
                                </div>
                            </div>
                            <StatsRefreshControls />
                        </div>

                        {t('description') ? (
                            <p className="max-w-2xl text-sm leading-7 text-muted-foreground md:text-[15px]">
                                {t('description')}
                            </p>
                        ) : null}
                    </div>

                    <div className="grid grid-cols-2 gap-2.5 sm:gap-3 xl:grid-cols-4">
                        {metrics.map((metric) => (
                            <div key={metric.key} className="group rounded-lg border border-border bg-card px-3 py-2.5 transition-colors duration-200 hover:border-border/80 hover:bg-muted/30 sm:px-4 sm:py-3">
                                <div className="mb-2 h-1 w-10 rounded-full bg-primary/20 transition-all duration-300 group-hover:w-14 group-hover:bg-primary/30" />
                                <div className="text-xs font-medium text-muted-foreground">{metric.label}</div>
                                <div className="mt-1 flex items-baseline gap-1">
                                    <span className="text-xl font-semibold sm:text-2xl">
                                        <AnimatedNumber value={metric.value} />
                                    </span>
                                    {metric.unit ? <span className="text-sm text-muted-foreground">{metric.unit}</span> : null}
                                </div>
                            </div>
                        ))}
                    </div>
                </div>

                <div className="grid grid-cols-2 gap-2.5 sm:gap-3 xl:grid-cols-4">
                    {signals.map((signal) => (
                        <article
                            key={signal.key}
                            className="group rounded-lg border border-border bg-card p-3 sm:p-4 transition-[transform,border-color] duration-300 hover:-translate-y-0.5 hover:border-border/80"
                        >
                            <div className="flex items-start justify-between gap-3">
                                <div className={`flex h-10 w-10 items-center justify-center rounded-lg ${signal.accent}`}>
                                    <signal.icon className="h-[1.125rem] w-[1.125rem]" strokeWidth={1.5} />
                                </div>
                            </div>
                            <div className="mt-5 text-xs text-muted-foreground">{signal.label}</div>
                            <div className="mt-2 flex items-baseline gap-1">
                                <span className="text-2xl font-semibold tracking-tight">
                                    <AnimatedNumber value={signal.value} />
                                </span>
                                {signal.unit ? <span className="text-sm text-muted-foreground">{signal.unit}</span> : null}
                            </div>
                        </article>
                    ))}
                </div>
            </div>
        </motion.section>
    );
}
