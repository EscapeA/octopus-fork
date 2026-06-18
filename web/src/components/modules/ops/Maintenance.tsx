'use client';

import { useTranslations } from 'next-intl';
import { SettingCircuitBreaker } from '@/components/modules/setting/CircuitBreaker';
import { SettingRetry } from '@/components/modules/setting/Retry';
import { SettingResponseFilter } from '@/components/modules/setting/ResponseFilter';

type MaintenanceSectionId = 'circuit-breaker' | 'retry' | 'response-filter';

const SECTIONS: MaintenanceSectionId[] = ['circuit-breaker', 'retry', 'response-filter'];

export function Maintenance() {
    const t = useTranslations('ops');

    return (
        <section className="space-y-4">
            <div className="space-y-1 px-1">
                <h3 className="text-base font-semibold">{t('tabs.maintenance')}</h3>
                <p className="text-sm leading-6 text-muted-foreground">{t('maintenance.description')}</p>
            </div>

            <div className="space-y-4">
                {SECTIONS.map((id) => (
                    <article
                        key={id}
                        className="rounded-xl border border-border/35 bg-card p-1 text-card-foreground shadow-sm"
                    >
                        {id === 'circuit-breaker' && <SettingCircuitBreaker />}
                        {id === 'retry' && <SettingRetry />}
                        {id === 'response-filter' && <SettingResponseFilter />}
                    </article>
                ))}
            </div>
        </section>
    );
}
