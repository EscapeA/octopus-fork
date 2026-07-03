'use client';

import { useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { Plus } from 'lucide-react';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContents, TabsContent, TabsList, TabsTrigger } from '@/components/animate-ui/components/animate/tabs';
import { Site } from '@/components/modules/site';
import { SiteChannelSection } from '@/components/modules/site-channel';
import { SettingSiteAutomation } from '@/components/modules/setting/SiteAutomation';
import { BalanceSection, TokenPlanSection } from '@/components/modules/plan-provider';
import { useHubTabStore } from './hub-tab-store';
import { useSiteUIStore } from '@/components/modules/site/ui-store';
import { useSubTabStore, type HubTab } from '@/components/modules/navbar/sub-tab-store';

const TAB_LABEL_KEY: Record<HubTab, string> = {
    sites: 'tabs.sites',
    'site-channels': 'tabs.siteChannels',
    automation: 'tabs.automation',
    balance: 'plan.balance',
    tokenplan: 'plan.tokenPlan',
};

export function RemoteSite() {
    const t = useTranslations('hub');
    const { activeTab, setActiveTab } = useHubTabStore();
    const { orderedTabs, visibleTabs } = useSubTabStore((s) => s.hub);
    const requestOpenCreateDialog = useSiteUIStore((state) => state.requestOpenCreateDialog);

    useEffect(() => {
        if (visibleTabs.length > 0 && !visibleTabs.includes(activeTab)) {
            setActiveTab(visibleTabs[0] as HubTab);
        }
    }, [visibleTabs, activeTab, setActiveTab]);

    const visibleSet = new Set(visibleTabs);
    const orderedVisible = orderedTabs.filter((tab) => visibleSet.has(tab));

    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-4 sm:space-y-6 rounded-t-xl pb-3 md:pb-4">
            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as HubTab)}>
                <section className="rounded-xl border border-border bg-card p-3 sm:p-5 text-card-foreground">
                    <div className="flex items-center justify-between gap-3">
                        <div className="overflow-x-auto -mx-1 px-1 scrollbar-none min-w-0">
                            <TabsList className="w-max min-w-full xl:min-w-0">
                                {orderedVisible.map((tab) => (
                                    <TabsTrigger key={tab} value={tab}>
                                        {tab === 'balance' ? (t('plan.balance') || '额度') : tab === 'tokenplan' ? (t('plan.tokenPlan') || 'TokenPlan') : t(TAB_LABEL_KEY[tab as HubTab])}
                                    </TabsTrigger>
                                ))}
                            </TabsList>
                        </div>
                        {activeTab === 'sites' && (
                            <Button
                                size="sm"
                                className="shrink-0 rounded-xl"
                                onClick={requestOpenCreateDialog}
                            >
                                <Plus className="size-4" />
                                <span className="hidden sm:inline">{t('addSite')}</span>
                            </Button>
                        )}
                    </div>
                </section>

                <TabsContents>
                    <TabsContent value="sites">
                        <Site />
                    </TabsContent>
                    <TabsContent value="site-channels">
                        {activeTab === 'site-channels' ? <SiteChannelSection /> : <div />}
                    </TabsContent>
                    <TabsContent value="automation">
                        {activeTab === 'automation' ? (
                            <div className="mx-auto max-w-2xl">
                                <SettingSiteAutomation />
                            </div>
                        ) : <div />}
                    </TabsContent>
                    <TabsContent value="balance">
                        <BalanceSection />
                    </TabsContent>
                    <TabsContent value="tokenplan">
                        <TokenPlanSection />
                    </TabsContent>
                </TabsContents>
            </Tabs>
        </PageWrapper>
    );
}
