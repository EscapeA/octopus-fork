'use client';

import { useState } from 'react';
import { BarChart3, Orbit } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Tabs, TabsContents, TabsContent, TabsList, TabsTrigger } from '@/components/animate-ui/components/animate/tabs';
import type { AnalyticsRange } from '@/api/endpoints/analytics';
import { Utilization } from './Utilization';
import { GroupHealth } from './GroupHealth';
import { Evaluation } from './Evaluation';

type AnalyticsTab = 'utilization' | 'route-health' | 'evaluation';

const RANGE_OPTIONS: AnalyticsRange[] = ['1d', '7d', '30d', '90d', 'ytd', 'all'];

export function Analytics() {
    const t = useTranslations('analytics');
    const [activeTab, setActiveTab] = useState<AnalyticsTab>('utilization');
    const [range, setRange] = useState<AnalyticsRange>('7d');
    const subtitle = t('subtitle');

    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-6 rounded-t-xl pb-24 md:pb-4">
            <section className="relative overflow-hidden rounded-xl border border-border/35 bg-card p-5 text-card-foreground md:p-6">
                <div className="relative flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
                    <div className="flex items-start gap-4">
                        <div className="grid h-14 w-14 shrink-0 place-items-center rounded-lg border border-border/35 bg-card text-primary">
                            <BarChart3 className="h-5 w-5" />
                        </div>
                        {subtitle ? (
                            <p className="min-w-0 max-w-3xl text-sm leading-6 text-muted-foreground">
                                {subtitle}
                            </p>
                        ) : null}
                    </div>
                    <div className="flex items-center gap-2 self-start rounded-lg border border-border/25 bg-card px-3 py-2 text-sm text-muted-foreground">
                        <Orbit className="h-4 w-4 text-primary" />
                        {t(`range.${range}`)}
                    </div>
                </div>
            </section>

            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as AnalyticsTab)}>
                <section className="relative overflow-hidden rounded-xl border border-border/35 bg-card p-4 text-card-foreground md:p-5">
                    <div className="relative flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
                        <div className="overflow-x-auto">
                            <TabsList className="flex w-max min-w-full flex-wrap rounded-lg border-border/30 bg-card p-1 xl:min-w-0">
                                <TabsTrigger value="utilization">{t('cards.utilization.title')}</TabsTrigger>
                                <TabsTrigger value="route-health">{t('cards.routeHealth.title')}</TabsTrigger>
                                <TabsTrigger value="evaluation">{t('evaluation.title')}</TabsTrigger>
                            </TabsList>
                        </div>

                        <Tabs value={range} onValueChange={(value) => setRange(value as AnalyticsRange)}>
                            <div className="overflow-x-auto">
                                <TabsList className="flex w-max min-w-full flex-wrap rounded-lg border-border/30 bg-card p-1 xl:min-w-0">
                                    {RANGE_OPTIONS.map((option) => (
                                        <TabsTrigger key={option} value={option}>
                                            {t(`range.${option}`)}
                                        </TabsTrigger>
                                    ))}
                                </TabsList>
                            </div>
                        </Tabs>
                    </div>
                </section>

                <TabsContents>
                    <TabsContent value="utilization">
                        <Utilization range={range} />
                    </TabsContent>
                    <TabsContent value="route-health">
                        <GroupHealth />
                    </TabsContent>
                    <TabsContent value="evaluation">
                        <Evaluation />
                    </TabsContent>
                </TabsContents>
            </Tabs>
        </PageWrapper>
    );
}
