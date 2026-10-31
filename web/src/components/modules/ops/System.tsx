'use client';

import { Boxes, KeyRound, Radio, Server, Settings2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { useOpsSystemSummary } from '@/api/endpoints/ops';
import { useNavStore } from '@/components/modules/navbar';
import { Button } from '@/components/ui/button';
import { MetricCard, QueryState } from '@/components/modules/analytics/shared';

function InfoRow({ label, value }: { label: string; value: string }) {
    return (
        <div className="flex flex-col gap-1 border-b border-border/40 py-3 last:border-b-0 last:pb-0 first:pt-0 sm:flex-row sm:items-start sm:justify-between sm:gap-4">
            <span className="text-sm text-muted-foreground">{label}</span>
            <span className="break-all text-sm font-medium sm:max-w-[60%] sm:text-right">{value || '-'}</span>
        </div>
    );
}

export function System() {
    const t = useTranslations('ops');
    const { setActiveItem } = useNavStore();
    const { data, isLoading, error } = useOpsSystemSummary();

    return (
        <section className="rounded-xl border border-border/35 bg-card p-5 text-card-foreground">
            <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div className="space-y-1">
                    <h3 className="text-base font-semibold">{t('tabs.system')}</h3>
                    <p className="text-sm leading-6 text-muted-foreground">{t('system.description')}</p>
                </div>
                <Button
                    variant="outline"
                    size="sm"
                    className="rounded-xl"
                    onClick={() => setActiveItem('setting')}
                >
                    {t('actions.openSettings')}
                </Button>
            </div>

            <QueryState
                loading={isLoading}
                error={error}
                empty={!data}
                emptyLabel={t('states.loading')}
            >
                <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-3 sm:gap-4 xl:grid-cols-4">
                        <MetricCard
                            title={t('system.metrics.channels')}
                            value={data?.channel_count ?? 0}
                            icon={Radio}
                        />
                        <MetricCard
                            title={t('system.metrics.groups')}
                            value={data?.group_count ?? 0}
                            icon={Boxes}
                        />
                        <MetricCard
                            title={t('system.metrics.apiKeys')}
                            value={data?.api_key_count ?? 0}
                            icon={KeyRound}
                        />
                    </div>

                    <div className="grid grid-cols-1 gap-4 xl:grid-cols-[1.2fr_0.8fr]">
                        <article className="rounded-xl border border-border/60 bg-card p-4">
                            <div className="mb-2 flex items-center gap-2 text-sm font-semibold">
                                <Server className="h-4 w-4" />
                                {t('system.sections.runtime')}
                            </div>
                            <InfoRow label={t('system.fields.version')} value={data?.version || '-'} />
                            <InfoRow label={t('system.fields.commit')} value={data?.commit || '-'} />
                            <InfoRow label={t('system.fields.buildTime')} value={data?.build_time || '-'} />
                            <InfoRow label={t('system.fields.databaseType')} value={data?.database_type || '-'} />
                            <InfoRow label={t('system.fields.publicApiBaseUrl')} value={data?.public_api_base_url || '-'} />
                            <InfoRow label={t('system.fields.proxyUrl')} value={data?.proxy_url || '-'} />
                            <InfoRow
                                label={t('system.fields.relayLogRetention')}
                                value={data?.relay_log_keep_enabled
                                    ? (data?.relay_log_keep_count > 0
                                        ? `${data.relay_log_keep_count} ${t('system.fields.logCountUnit')}`
                                        : `${data?.relay_log_keep_days}d`)
                                    : t('system.fields.disabled')}
                            />
                            <InfoRow
                                label={t('system.fields.statsSaveInterval')}
                                value={`${data?.stats_save_interval_minutes ?? 0}m`}
                            />
                            <InfoRow
                                label={t('system.fields.syncLlmInterval')}
                                value={`${data?.sync_llm_interval_hours ?? 0}h`}
                            />
                            <InfoRow
                                label={t('system.fields.modelInfoInterval')}
                                value={`${data?.model_info_update_interval_hours ?? 0}h`}
                            />
                        </article>

                        <article className="rounded-xl border border-border/60 bg-card p-4">
                            <div className="mb-2 flex items-center gap-2 text-sm font-semibold">
                                <Settings2 className="h-4 w-4" />
                                {t('system.sections.operations')}
                            </div>
                            <InfoRow
                                label={t('system.fields.importExport')}
                                value={`${data?.import_enabled ? t('system.fields.enabled') : t('system.fields.disabled')} / ${data?.export_enabled ? t('system.fields.enabled') : t('system.fields.disabled')}`}
                            />
                        </article>
                    </div>
                </div>
            </QueryState>
        </section>
    );
}

