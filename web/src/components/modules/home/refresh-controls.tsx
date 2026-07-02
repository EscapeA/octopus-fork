'use client';

import { useState } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useTranslations } from 'next-intl';
import { Loader2, RefreshCw } from 'lucide-react';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import {
    STATS_REFRESH_INTERVAL_OPTIONS,
    useHomeViewStore,
    type StatsRefreshInterval,
} from './store';

// 首页统计涉及的查询前缀，手动刷新时一并 refetch。
const HOME_STATS_QUERY_PREFIXES = ['stats', 'analytics'] as const;

export function StatsRefreshControls() {
    const t = useTranslations('home.refresh');
    const queryClient = useQueryClient();
    const statsRefreshInterval = useHomeViewStore((state) => state.statsRefreshInterval);
    const setStatsRefreshInterval = useHomeViewStore((state) => state.setStatsRefreshInterval);
    const [isRefreshing, setIsRefreshing] = useState(false);

    const handleRefresh = async () => {
        if (isRefreshing) return;
        setIsRefreshing(true);
        try {
            await Promise.all(
                HOME_STATS_QUERY_PREFIXES.map((prefix) =>
                    queryClient.refetchQueries({ queryKey: [prefix] }),
                ),
            );
        } finally {
            setIsRefreshing(false);
        }
    };

    return (
        <div className="flex items-center gap-2">
            <button
                type="button"
                onClick={handleRefresh}
                disabled={isRefreshing}
                className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-card px-3 py-2 text-xs font-medium text-muted-foreground transition-colors hover:border-border/80 hover:text-foreground disabled:cursor-not-allowed disabled:opacity-60"
                title={t('manual')}
            >
                {isRefreshing ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" strokeWidth={1.5} />
                ) : (
                    <RefreshCw className="h-3.5 w-3.5" strokeWidth={1.5} />
                )}
                <span className="hidden sm:inline">{t('manual')}</span>
            </button>

            <Select
                value={statsRefreshInterval}
                onValueChange={(value) => setStatsRefreshInterval(value as StatsRefreshInterval)}
            >
                <SelectTrigger size="sm" className="h-8 w-auto gap-1.5 rounded-lg border-border bg-card px-3 text-xs font-medium text-muted-foreground">
                    <RefreshCw className="h-3.5 w-3.5" strokeWidth={1.5} />
                    <SelectValue />
                </SelectTrigger>
                <SelectContent align="end">
                    {STATS_REFRESH_INTERVAL_OPTIONS.map((option) => (
                        <SelectItem key={option} value={option}>
                            {t(`interval.${option}`)}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
        </div>
    );
}
