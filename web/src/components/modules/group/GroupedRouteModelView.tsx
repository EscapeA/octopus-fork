'use client';

import { useMemo, useState } from 'react';
import { ChevronDown, Layers3, Waves } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { cn } from '@/lib/utils';
import { getModelIcon } from '@/lib/model-icons';
import type { GroupedRouteModelBucket, GroupedRouteModelRow } from './grouped-view';
import { UNGROUPED_BUCKET_ID } from './grouped-view';

function RouteModelRow({ model, index }: { model: GroupedRouteModelRow; index: number }) {
    const { Avatar: ModelAvatar } = getModelIcon(model.name);

    return (
        <div
            className={cn(
                'flex min-w-0 items-center gap-2 rounded-lg border border-border/30 bg-card px-3 py-2 transition-colors hover:border-primary/16 hover:bg-muted/30',
                !model.enabled && 'opacity-60 grayscale',
            )}
        >
            <span className="grid size-7 shrink-0 place-items-center rounded-md bg-primary/10 text-xs font-semibold text-primary">
                {index + 1}
            </span>
            <span className="shrink-0">
                <ModelAvatar size={18} />
            </span>
            <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-medium text-foreground">{model.name}</div>
                <div className="truncate text-xs text-muted-foreground">{model.channel_name}</div>
            </div>
            {typeof model.weight === 'number' ? (
                <span className="shrink-0 rounded-md border border-border/30 bg-muted/30 px-2 py-1 text-[11px] font-medium text-muted-foreground">
                    {model.weight}
                </span>
            ) : null}
        </div>
    );
}

function BucketHeader({ bucket, expanded, onToggle }: { bucket: GroupedRouteModelBucket; expanded: boolean; onToggle: () => void }) {
    const t = useTranslations('group');
    const title = bucket.kind === 'ungrouped' ? t('groupedView.ungrouped') : bucket.name;
    const description = bucket.kind === 'ungrouped'
        ? t('groupedView.ungroupedDescription')
        : [bucket.category, bucket.endpoint_type ? t('card.endpointType', { value: bucket.endpoint_type }) : null]
            .filter(Boolean)
            .join(' · ');

    return (
        <button
            type="button"
            onClick={onToggle}
            aria-expanded={expanded}
            className="flex w-full min-w-0 items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/30 md:px-5 md:py-4"
        >
            <span className="grid size-9 shrink-0 place-items-center rounded-lg border border-border/40 bg-muted/30 text-muted-foreground">
                {bucket.kind === 'ungrouped' ? <Layers3 className="size-4" /> : <Waves className="size-4" />}
            </span>
            <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-semibold text-foreground md:text-base">{title}</span>
                {description ? <span className="mt-0.5 block truncate text-xs text-muted-foreground">{description}</span> : null}
            </span>
            <span className="shrink-0 rounded-full border border-border/40 bg-card px-2 py-1 text-[11px] font-medium text-muted-foreground">
                {t('groupedView.modelCount', { count: bucket.models.length })}
            </span>
            <ChevronDown className={cn('size-4 shrink-0 text-muted-foreground transition-transform', expanded && 'rotate-180')} />
        </button>
    );
}

export function GroupedRouteModelView({ buckets }: { buckets: GroupedRouteModelBucket[] }) {
    const t = useTranslations('group');
    const initialExpanded = useMemo(() => new Set(buckets.slice(0, 3).map((bucket) => bucket.key)), [buckets]);
    const [expandedKeys, setExpandedKeys] = useState<Set<string>>(initialExpanded);

    if (buckets.length === 0) {
        return (
            <div className="flex h-full min-h-0 items-center justify-center p-6">
                <div className="max-w-md rounded-xl border border-border bg-card p-6 text-center text-card-foreground">
                    <div className="text-sm font-semibold">{t('groupedView.emptyTitle')}</div>
                    <div className="mt-2 text-sm text-muted-foreground">{t('groupedView.emptyDescription')}</div>
                </div>
            </div>
        );
    }

    return (
        <div className="h-full min-h-0 overflow-y-auto overscroll-contain rounded-t-xl px-3 pb-3 md:px-4 md:pb-4">
            <div className="grid gap-3 py-3 md:py-4">
                {buckets.map((bucket) => {
                    const expanded = expandedKeys.has(bucket.key);
                    return (
                        <article
                            key={bucket.key}
                            className={cn(
                                'overflow-hidden rounded-xl border border-border bg-card text-card-foreground shadow-sm',
                                bucket.id === UNGROUPED_BUCKET_ID && 'border-dashed',
                            )}
                        >
                            <BucketHeader
                                bucket={bucket}
                                expanded={expanded}
                                onToggle={() => {
                                    setExpandedKeys((current) => {
                                        const next = new Set(current);
                                        if (next.has(bucket.key)) {
                                            next.delete(bucket.key);
                                        } else {
                                            next.add(bucket.key);
                                        }
                                        return next;
                                    });
                                }}
                            />
                            {expanded ? (
                                <div className="border-t border-border/40 bg-muted/10 px-3 py-3 md:px-4">
                                    {bucket.models.length > 0 ? (
                                        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                                            {bucket.models.map((model, index) => (
                                                <RouteModelRow key={model.id} model={model} index={index} />
                                            ))}
                                        </div>
                                    ) : (
                                        <div className="rounded-lg border border-border/30 bg-card px-3 py-4 text-sm text-muted-foreground">
                                            {t('groupedView.emptyGroup')}
                                        </div>
                                    )}
                                </div>
                            ) : null}
                        </article>
                    );
                })}
            </div>
        </div>
    );
}
