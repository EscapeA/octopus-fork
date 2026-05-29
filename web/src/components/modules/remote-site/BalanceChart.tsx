'use client';

import { useMemo } from 'react';
import { useBalanceChart } from '@/api/endpoints/balance-history';
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts';
import { useTranslations } from 'next-intl';

interface BalanceChartProps {
    siteId: number;
    days?: number;
}

export function BalanceChart({ siteId, days = 30 }: BalanceChartProps) {
    const t = useTranslations('hub');
    const startDate = useMemo(() => {
        const d = new Date();
        d.setDate(d.getDate() - days);
        return d.toISOString().slice(0, 10);
    }, [days]);

    const { data: chartData, isLoading } = useBalanceChart(siteId, startDate);

    if (isLoading) {
        return <div className="h-48 flex items-center justify-center text-muted-foreground text-sm">{t('chart.loading')}</div>;
    }

    if (!chartData || chartData.length === 0) {
        return <div className="h-48 flex items-center justify-center text-muted-foreground text-sm">{t('chart.noData')}</div>;
    }

    return (
        <ResponsiveContainer width="100%" height={200}>
            <LineChart data={chartData} margin={{ top: 5, right: 10, left: 10, bottom: 5 }}>
                <CartesianGrid strokeDasharray="3 3" className="stroke-border" />
                <XAxis
                    dataKey="day_key"
                    tick={{ fontSize: 11 }}
                    tickFormatter={(v: string) => v.slice(5)}
                    className="text-muted-foreground"
                />
                <YAxis
                    tick={{ fontSize: 11 }}
                    className="text-muted-foreground"
                    width={50}
                />
                <Tooltip
                    contentStyle={{
                        fontSize: 12,
                        borderRadius: 8,
                        border: '1px solid hsl(var(--border))',
                        background: 'hsl(var(--card))',
                    }}
                />
                <Line
                    type="monotone"
                    dataKey="quota"
                    stroke="hsl(var(--primary))"
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4 }}
                />
            </LineChart>
        </ResponsiveContainer>
    );
}
