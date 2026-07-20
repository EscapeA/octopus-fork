'use client';

import { cn } from '@/lib/utils';
import type { ChannelUpstreamMetrics, ChannelUpstreamPrice } from '@/api/endpoints/model';

function formatPrice(value: number): string {
    if (!Number.isFinite(value) || value <= 0) {
        return '';
    }
    if (value >= 100) {
        return value.toFixed(0);
    }
    if (value >= 10) {
        return value.toFixed(1);
    }
    if (value >= 1) {
        return value.toFixed(2);
    }
    if (value >= 0.01) {
        return value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '');
    }
    return value.toFixed(4).replace(/0+$/, '').replace(/\.$/, '');
}

function formatBalance(value: number): string {
    if (!Number.isFinite(value)) {
        return '';
    }
    const abs = Math.abs(value);
    if (abs >= 100) {
        return value.toFixed(1);
    }
    if (abs >= 1) {
        return value.toFixed(2);
    }
    return value.toFixed(3).replace(/0+$/, '').replace(/\.$/, '');
}

function formatRemainingCalls(value: number): string {
    if (!Number.isFinite(value) || value < 0) {
        return '';
    }
    if (value >= 100) {
        return Math.floor(value).toLocaleString('en-US');
    }
    if (value >= 10) {
        return value.toFixed(1).replace(/\.0$/, '');
    }
    if (Number.isInteger(value)) {
        return String(value);
    }
    return value.toFixed(1).replace(/0+$/, '').replace(/\.$/, '');
}

// NewAPI 模型广场：延迟 ms 较大时显示为秒（如 139000ms → 139s）。
function formatLatency(latencyMs: number): string {
    if (!Number.isFinite(latencyMs) || latencyMs <= 0) {
        return '';
    }
    if (latencyMs >= 1000) {
        const seconds = latencyMs / 1000;
        if (seconds >= 100) {
            return `${Math.round(seconds)}s`;
        }
        if (seconds >= 10) {
            return `${seconds.toFixed(0)}s`;
        }
        return `${seconds.toFixed(1).replace(/\.0$/, '')}s`;
    }
    return `${Math.round(latencyMs)}ms`;
}

function formatThroughput(tps: number): string {
    if (!Number.isFinite(tps) || tps <= 0) {
        return '';
    }
    if (tps >= 100) {
        return `${Math.round(tps)}t`;
    }
    if (tps >= 10) {
        return `${tps.toFixed(0)}t`;
    }
    return `${tps.toFixed(1).replace(/\.0$/, '')}t`;
}

// 状态条：按成功率 0-1 映射 1-3 格，颜色绿/黄/红。
function statusLevel(successRate: number): 0 | 1 | 2 | 3 {
    if (!Number.isFinite(successRate) || successRate <= 0) {
        return 0;
    }
    if (successRate >= 0.9) {
        return 3;
    }
    if (successRate >= 0.7) {
        return 2;
    }
    if (successRate >= 0.4) {
        return 1;
    }
    return 1;
}

function StatusBars({ successRate }: { successRate: number }) {
    const level = statusLevel(successRate);
    if (level === 0) {
        return null;
    }
    const color =
        successRate >= 0.9
            ? 'bg-emerald-500'
            : successRate >= 0.7
              ? 'bg-amber-500'
              : 'bg-red-500';
    const heights = ['h-1.5', 'h-2.5', 'h-3.5'] as const;
    return (
        <span
            className="inline-flex items-end gap-0.5"
            title={`成功率 ${(successRate * 100).toFixed(0)}%`}
        >
            {heights.map((h, idx) => (
                <span
                    key={h}
                    className={cn(
                        'w-0.5 rounded-sm',
                        h,
                        idx < level ? color : 'bg-muted-foreground/25',
                    )}
                />
            ))}
        </span>
    );
}

export function UpstreamPriceBadges({
    price,
    balance,
    metrics,
    className,
}: {
    price?: ChannelUpstreamPrice | null;
    balance?: number | null;
    metrics?: ChannelUpstreamMetrics | null;
    className?: string;
}) {
    const isPerCall = price?.billing_mode === 'per_call';
    const unit = isPerCall ? '次' : 'M';
    const inputText = price ? formatPrice(price.input) : '';
    const outputText = price ? formatPrice(price.output) : '';
    const cacheText = price ? formatPrice(price.cache_read) : '';
    const hasPrice = Boolean(inputText || outputText || cacheText);
    const hasBalance = typeof balance === 'number' && Number.isFinite(balance);
    const balanceText = hasBalance ? formatBalance(balance!) : '';

    const perCallUnitPrice = isPerCall
        ? price && price.input > 0
            ? price.input
            : price && price.output > 0
              ? price.output
              : 0
        : 0;
    const remainingCalls =
        isPerCall && hasBalance && perCallUnitPrice > 0
            ? balance! / perCallUnitPrice
            : null;
    const remainingCallsText =
        remainingCalls !== null ? formatRemainingCalls(remainingCalls) : '';

    const latencyText = metrics ? formatLatency(metrics.latency_ms) : '';
    const tpsText = metrics ? formatThroughput(metrics.avg_tps) : '';
    const hasStatus = Boolean(metrics && metrics.success_rate > 0);
    const hasMetrics = Boolean(latencyText || tpsText || hasStatus);

    if (!hasPrice && !hasBalance && !hasMetrics) {
        return null;
    }

    return (
        <span
            className={cn(
                'inline-flex max-w-full min-w-0 items-center gap-x-1.5 overflow-hidden text-[10px] font-semibold leading-none tabular-nums md:text-[11px]',
                className,
            )}
            title={[
                inputText ? `输入 ${inputText}$/${unit}` : '',
                outputText ? `输出 ${outputText}$/${unit}` : '',
                cacheText ? `缓存读 ${cacheText}$/${unit}` : '',
                balanceText ? `余额 ${balanceText}$` : '',
                remainingCallsText ? `约可用 ${remainingCallsText} 次` : '',
                latencyText ? `延迟 ${latencyText}` : '',
                tpsText ? `吞吐 ${tpsText}` : '',
                hasStatus ? `成功率 ${(metrics!.success_rate * 100).toFixed(0)}%` : '',
            ]
                .filter(Boolean)
                .join(' · ')}
        >
            {hasPrice ? (
                <span className="inline-flex min-w-0 items-center gap-x-1 overflow-hidden">
                    {inputText ? (
                        <span className="shrink-0 text-emerald-600 dark:text-emerald-400">{inputText}</span>
                    ) : null}
                    {inputText && outputText ? (
                        <span className="shrink-0 text-muted-foreground/50">/</span>
                    ) : null}
                    {outputText ? (
                        <span className="shrink-0 text-red-500 dark:text-red-400">{outputText}</span>
                    ) : null}
                    {(inputText || outputText) && cacheText ? (
                        <span className="shrink-0 text-muted-foreground/50">/</span>
                    ) : null}
                    {cacheText ? (
                        <span className="shrink-0 text-amber-500 dark:text-amber-400">{cacheText}</span>
                    ) : null}
                    <span className="shrink-0 text-muted-foreground">$/{unit}</span>
                </span>
            ) : null}
            {hasBalance ? (
                <span className="shrink-0 text-sky-600 dark:text-sky-400">{balanceText}$</span>
            ) : null}
            {remainingCallsText ? (
                <span className="shrink-0 font-bold text-violet-600 dark:text-violet-400">
                    ({remainingCallsText}/次)
                </span>
            ) : null}
            {hasMetrics ? (
                <span className="inline-flex shrink-0 items-center gap-x-1.5 text-muted-foreground">
                    {latencyText ? <span className="shrink-0">{latencyText}</span> : null}
                    {tpsText ? <span className="shrink-0">{tpsText}</span> : null}
                    {hasStatus ? <StatusBars successRate={metrics!.success_rate} /> : null}
                </span>
            ) : null}
        </span>
    );
}
