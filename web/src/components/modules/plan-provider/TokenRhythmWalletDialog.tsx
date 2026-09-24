'use client';

import { Loader2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { cn } from '@/lib/utils';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { useTokenRhythmWallet, type PlanProvider } from '@/api/endpoints/plan-provider';

// 金额格式：与基元律动官网一致 —— 分位向上取整（ceil 到 2 位小数）。
// 依据（官网显示值 vs 上游原始值实测）：
//   220.82454716 → 220.83、66.82454716 → 66.83（四舍五入会得 .82，故为 ceil）
//   −1.17545284 → −1.17（向零取整）、0.019507 → 0.02
// 减 1e-6 抵消浮点乘法误差（如 18*100 可能得 1800.0000000000002 → 否则会多进 1 分）。
const formatMoney = (val: number) => {
    if (!val) return '0.00';
    // 直接对带符号值向上取整（负数向零）：−1.17545284 → −1.17，与官网一致。
    // 不能取绝对值再补负号，那会得到 −1.18。
    const cents = Math.ceil(val * 100 - 1e-6) / 100;
    return cents.toLocaleString(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
    });
};

// signedMoney 带符号金额（0 时不显示 +0.00/-0.00）
const signedMoney = (val: number, sign: '+' | '-') => (val > 0 ? `${sign}${formatMoney(val)}` : formatMoney(0));

// M月D日 HH:mm
const formatDateTime = (val: string | null) => {
    if (!val) return '';
    const d = new Date(val);
    if (Number.isNaN(d.getTime())) return val;
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getMonth() + 1}月${d.getDate()}日 ${pad(d.getHours())}:${pad(d.getMinutes())}`;
};

// YYYY-MM-DD
const formatDate = (val: string | null) => {
    if (!val) return '';
    const d = new Date(val);
    if (Number.isNaN(d.getTime())) return val;
    const pad = (n: number) => String(n).padStart(2, '0');
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

// 明细列宽（桌面端 7 列）：时间 | 类型 | 到账金额 | 已消费 | 余额 | 状态 | 过期时间
const GRID_COLS = 'grid-cols-[132px_1fr_84px_76px_88px_60px_96px]';

/**
 * TokenRhythmWalletDialog 基元律动「资金明细」弹窗。
 *
 * 数据源：后端 /api/v1/plan-provider/wallet/transactions/:id（代理基元律动
 * wallet/summary + wallet/transactions + usage-summary）。
 * 只在弹窗打开时拉取，展示上游首屏 20 条（不分页）。
 *
 * 口径（2026-09-24 与用户确认）：
 *   - 顶部汇总：累计到账 = 官方赠送总额；累计消费 = 累计成本（与卡片「已用额度」同源）
 *   - 明细只列到账/赠送类记录（CREDIT），模型调用扣费等消费记录不进本弹窗
 *   - 每行金额三项：到账金额（amountCny）｜已消费（该笔到账中抵扣欠费的部分 |debtDeltaCny|）
 *     ｜实际入账（giftDeltaCny + rechargeDeltaCny，抵扣欠费后的净入账）
 */
export function TokenRhythmWalletDialog({
    provider,
    open,
    onOpenChange,
}: {
    provider: PlanProvider | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}) {
    const t = useTranslations('hub');
    // 未打开 / 无 provider 时传 null → hook 不发请求
    const { data, isLoading, error } = useTokenRhythmWallet(open && provider ? provider.id : null);

    // 类型映射；未知枚举回退 description（上游中文说明）再回退原文，避免新增类型被吞掉
    const typeLabel = (type: string, description: string) => {
        switch (type) {
            case 'INVITE_REWARD':
                return t('plan.wallet.typeInviteReward') || '邀请奖励';
            case 'MODEL_USAGE':
                return t('plan.wallet.typeModelUsage') || '模型调用扣费';
            case 'RECHARGE':
                return t('plan.wallet.typeRecharge') || '充值';
            case 'GIFT':
                return t('plan.wallet.typeGift') || '赠送';
            case 'REFUND':
                return t('plan.wallet.typeRefund') || '退款';
            case 'OTHER':
                return description || t('plan.wallet.typeOther') || '其他';
            default:
                return description || type || '-';
        }
    };

    // 状态映射；未知枚举回退原文
    const statusLabel = (status: string) => {
        switch (status) {
            case 'POSTED':
                return t('plan.wallet.statusPosted') || '已到账';
            case 'PENDING':
                return t('plan.wallet.statusPending') || '处理中';
            case 'EXPIRED':
                return t('plan.wallet.statusExpired') || '已过期';
            case 'FAILED':
                return t('plan.wallet.statusFailed') || '失败';
            case 'VOIDED':
                return t('plan.wallet.statusVoided') || '已作废';
            default:
                return status || '-';
        }
    };

    // 只保留到账/赠送类记录：模型调用扣费等消费记录（direction=DEBIT）不混进资金明细
    const transactions = (data?.transactions ?? []).filter((tx) => tx.direction !== 'DEBIT');

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="sm:max-w-3xl">
                <DialogHeader>
                    <DialogTitle>{t('plan.wallet.title') || '资金明细'}</DialogTitle>
                    <DialogDescription>
                        {provider?.name ? `${provider.name} · ` : ''}
                        {t('plan.wallet.desc') || '基元律动控制台钱包余额变动'}
                    </DialogDescription>
                </DialogHeader>

                {isLoading ? (
                    <div className="flex items-center justify-center gap-2 py-10 text-sm text-muted-foreground">
                        <Loader2 className="size-4 animate-spin" />
                        {t('plan.wallet.loading') || '加载中…'}
                    </div>
                ) : error ? (
                    <div className="py-8 text-center text-sm text-destructive">
                        {t('plan.wallet.error') || '获取资金明细失败'}
                        <p className="mt-1 break-all text-xs text-muted-foreground">
                            {String((error as Error)?.message || '')}
                        </p>
                    </div>
                ) : (
                    <>
                        {/* 汇总：累计到账（官方赠送总额）/ 累计消费（累计成本） */}
                        <div className="grid grid-cols-2 gap-3">
                            <div className="rounded-lg bg-muted/50 p-3">
                                <p className="mb-1 text-xs text-muted-foreground">
                                    {t('plan.wallet.totalReceived') || '累计到账'}
                                </p>
                                <p className="text-lg font-bold tabular-nums text-emerald-600">
                                    {signedMoney(data?.total_received_cny ?? 0, '+')}
                                </p>
                            </div>
                            <div className="rounded-lg bg-muted/50 p-3">
                                <p className="mb-1 text-xs text-muted-foreground">
                                    {t('plan.wallet.totalConsumed') || '累计消费'}
                                </p>
                                <p className="text-lg font-bold tabular-nums text-destructive">
                                    {signedMoney(data?.total_consumed_cny ?? 0, '-')}
                                </p>
                            </div>
                        </div>

                        {/* 明细列表：桌面 7 列 / 移动端堆叠 */}
                        <div className="overflow-hidden rounded-lg border border-border">
                            <div
                                className={cn(
                                    'hidden border-b border-border bg-muted/40 px-3 py-2 text-xs text-muted-foreground sm:grid sm:gap-2',
                                    GRID_COLS,
                                )}
                            >
                                <span>{t('plan.wallet.colTime') || '时间'}</span>
                                <span>{t('plan.wallet.colType') || '类型'}</span>
                                <span className="text-right">{t('plan.wallet.colReceived') || '到账金额'}</span>
                                <span className="text-right">{t('plan.wallet.colConsumed') || '已消费'}</span>
                                <span className="text-right">{t('plan.wallet.colNetCredit') || '实际入账'}</span>
                                <span>{t('plan.wallet.colStatus') || '状态'}</span>
                                <span>{t('plan.wallet.colExpires') || '过期时间'}</span>
                            </div>
                            <div className="max-h-[46vh] divide-y divide-border overflow-y-auto">
                                {transactions.length === 0 ? (
                                    <p className="py-8 text-center text-sm text-muted-foreground">
                                        {t('plan.wallet.empty') || '暂无资金明细'}
                                    </p>
                                ) : (
                                    transactions.map((tx) => {
                                        // 已消费 = 该笔到账中用于抵扣欠费的部分（上游为负值）
                                        const debtUsed = Math.abs(tx.debt_delta_cny ?? 0);
                                        // 实际入账 = 赠送余额变动 + 充值余额变动（抵扣欠费后的净额，如 68 − 1.18 = 66.83）
                                        const netCredited =
                                            (tx.gift_delta_cny ?? 0) + (tx.recharge_delta_cny ?? 0);
                                        return (
                                            <div key={tx.transaction_id} className="px-3 py-2 hover:bg-muted/30">
                                                <div
                                                    className={cn(
                                                        'hidden items-center gap-2 text-xs sm:grid',
                                                        GRID_COLS,
                                                    )}
                                                >
                                                    <span className="tabular-nums text-muted-foreground">
                                                        {formatDateTime(tx.occurred_at)}
                                                    </span>
                                                    <span className="truncate">{typeLabel(tx.type, tx.description)}</span>
                                                    <span className="text-right font-medium tabular-nums text-emerald-600">
                                                        {signedMoney(tx.amount_cny, '+')}
                                                    </span>
                                                    <span className="text-right tabular-nums text-destructive">
                                                        {debtUsed > 0 ? formatMoney(debtUsed) : '—'}
                                                    </span>
                                                    <span className="text-right font-medium tabular-nums">
                                                        {formatMoney(netCredited)}
                                                    </span>
                                                    <span>{statusLabel(tx.status)}</span>
                                                    <span className="tabular-nums text-muted-foreground">
                                                        {tx.expires_at ? formatDate(tx.expires_at) : '—'}
                                                    </span>
                                                </div>
                                                <div className="space-y-1 sm:hidden">
                                                    <div className="flex items-center justify-between gap-2">
                                                        <span className="truncate text-sm">
                                                            {typeLabel(tx.type, tx.description)}
                                                        </span>
                                                        <span className="shrink-0 text-sm font-semibold tabular-nums text-emerald-600">
                                                            {signedMoney(tx.amount_cny, '+')}
                                                        </span>
                                                    </div>
                                                    <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-[11px] text-muted-foreground">
                                                        <span>{formatDateTime(tx.occurred_at)}</span>
                                                        <span>{statusLabel(tx.status)}</span>
                                                        {debtUsed > 0 && (
                                                            <span>
                                                                {t('plan.wallet.colConsumed') || '已消费'}{' '}
                                                                {formatMoney(debtUsed)}
                                                            </span>
                                                        )}
                                                        <span>
                                                            {t('plan.wallet.colNetCredit') || '实际入账'}{' '}
                                                            {formatMoney(netCredited)}
                                                        </span>
                                                        {tx.expires_at && (
                                                            <span>
                                                                {t('plan.wallet.expiresShort') || '过期'}{' '}
                                                                {formatDate(tx.expires_at)}
                                                            </span>
                                                        )}
                                                    </div>
                                                </div>
                                            </div>
                                        );
                                    })
                                )}
                            </div>
                        </div>
                    </>
                )}
            </DialogContent>
        </Dialog>
    );
}
