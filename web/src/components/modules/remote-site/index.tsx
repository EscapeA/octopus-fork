'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Tabs, TabsContents, TabsContent, TabsList, TabsTrigger } from '@/components/animate-ui/components/animate/tabs';
import { SitesPanel } from './SitesPanel';
import { CheckInPanel } from './CheckInPanel';
import { AnnouncementPanel } from './AnnouncementPanel';
import { RedemptionPanel } from './RedemptionPanel';
import { UsageHistoryPanel } from './UsageHistoryPanel';
import { CredentialPanel } from './CredentialPanel';

type HubTab = 'sites' | 'checkin' | 'announcement' | 'redemption' | 'usage' | 'credential';

export function RemoteSite() {
    const t = useTranslations('hub');
    const [activeTab, setActiveTab] = useState<HubTab>('sites');

    return (
        <PageWrapper className="h-full min-h-0 overflow-y-auto overscroll-contain space-y-4 sm:space-y-6 rounded-t-xl pb-3 md:pb-4">
            <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as HubTab)}>
                <section className="rounded-xl border border-border bg-card p-3 sm:p-5 text-card-foreground">
                    <div className="overflow-x-auto -mx-1 px-1 scrollbar-none">
                        <TabsList className="w-max min-w-full xl:min-w-0">
                            <TabsTrigger value="sites">{t('tabs.sites')}</TabsTrigger>
                            <TabsTrigger value="checkin">{t('tabs.checkin')}</TabsTrigger>
                            <TabsTrigger value="announcement">{t('tabs.announcement')}</TabsTrigger>
                            <TabsTrigger value="redemption">{t('tabs.redemption')}</TabsTrigger>
                            <TabsTrigger value="usage">{t('tabs.usage')}</TabsTrigger>
                            <TabsTrigger value="credential">{t('tabs.credential')}</TabsTrigger>
                        </TabsList>
                    </div>
                </section>

                <TabsContents>
                    <TabsContent value="sites">
                        <SitesPanel />
                    </TabsContent>
                    <TabsContent value="checkin">
                        <CheckInPanel />
                    </TabsContent>
                    <TabsContent value="announcement">
                        <AnnouncementPanel />
                    </TabsContent>
                    <TabsContent value="redemption">
                        <RedemptionPanel />
                    </TabsContent>
                    <TabsContent value="usage">
                        <UsageHistoryPanel />
                    </TabsContent>
                    <TabsContent value="credential">
                        <CredentialPanel />
                    </TabsContent>
                </TabsContents>
            </Tabs>
        </PageWrapper>
    );
}
