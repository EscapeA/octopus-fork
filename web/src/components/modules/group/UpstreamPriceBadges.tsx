'use client';

import { cn } from '@/lib/utils';
import type { ChannelUpstreamPrice } from '@/api/endpoints/model';

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

export function UpstreamPriceBadges({
    price,
    balance,
    className,
}: {
    price?: ChannelUpstreamPrice | null;
    balance?: number | null;
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

    // 按次：用单价估算剩余可用次数（余额 / 单次价格）。
    // 优先用输入价，没有则用输出价。
    const perCallUnitPrice = isPerCall
        ? (price && price.input > 0 ? price.input : price && price.output > 0 ? price.output : 0)
        : 0;
    const remainingCalls =
        isPerCall && hasBalance && perCallUnitPrice > 0
            ? balance! / perCallUnitPrice
            : null;
    const remainingCallsText =
        remainingCalls !== null ? formatRemainingCalls(remainingCalls) : '';

    if (!hasPrice && !hasBalance) {
        return null;
    }

    // 单行紧凑：绿输入 / 红输出 / 黄缓存 共用单位，余额单独蓝色。
    // 按次时额外显示 (N/次) 估算可用次数。
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
        </span>
    );
}
