'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useChannelList } from '@/api/endpoints/channel';
import { useAPIKeyList } from '@/api/endpoints/apikey';
import { useLogs, type LogFilter } from '@/api/endpoints/log';
import { useModelList } from '@/api/endpoints/model';
import { LogCard } from './Item';
import { Loader2, X, Columns3, Check, ChevronsUpDown, Search, SlidersHorizontal } from 'lucide-react';
import { Popover, PopoverTrigger, PopoverContent } from '@/components/ui/popover';
import { useLogFieldVisibilityStore, useLogModelSearchStore, type LogFieldName } from './ui-store';
import { useTranslations } from 'next-intl';
import { VirtualizedGrid } from '@/components/common/VirtualizedGrid';
import { useNavHandoff } from '@/lib/nav-handoff';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { useIsMobile } from '@/hooks/use-mobile';
import { cn } from '@/lib/utils';
import { ENDPOINT_TYPE_OPTIONS } from '@/components/modules/group/utils';

const EMPTY_FILTER: LogFilter = {};

/**
 * 日志筛选栏
 */
function LogFilterBar({
    filter,
    onChange,
}: {
    filter: LogFilter;
    onChange: (f: LogFilter) => void;
}) {
    const t = useTranslations('log.filter');
    const tGroup = useTranslations('group');
    const tView = useTranslations('log.viewOptions');
    const isMobile = useIsMobile();
    const { data: channels = [] } = useChannelList();
    const { data: apiKeys = [] } = useAPIKeyList();
    const { data: models = [] } = useModelList();
    const visibility = useLogFieldVisibilityStore((s) => s.visibility);
    const [mobileOpen, setMobileOpen] = useState(false);

    const hasFilter = !!(
        filter.channel_id != null ||
        filter.api_key_id != null ||
        filter.endpoint_type ||
        filter.status ||
        filter.is_test != null ||
        (filter.models && filter.models.length > 0)
    );
    const activeFilterCount = [
        filter.channel_id != null,
        filter.api_key_id != null,
        !!filter.endpoint_type,
        !!filter.status,
        filter.is_test != null,
        !!(filter.models && filter.models.length > 0),
    ].filter(Boolean).length;

    const selectedModels = useMemo(() => new Set(filter.models ?? []), [filter.models]);
    const [modelSearchTerm, setModelSearchTerm] = useState('');
    const modelSearchInputRef = useRef<HTMLInputElement>(null);

    const filteredModelOptions = useMemo(() => {
        const term = modelSearchTerm.trim().toLowerCase();
        const sorted = [...models].sort((a, b) => a.name.localeCompare(b.name));
        if (!term) return sorted;
        return sorted.filter((m) => m.name.toLowerCase().includes(term));
    }, [models, modelSearchTerm]);

    const toggleModel = useCallback((name: string) => {
        const next = { ...filter };
        const set = new Set(next.models ?? []);
        if (set.has(name)) {
            set.delete(name);
        } else {
            set.add(name);
        }
        if (set.size > 0) {
            next.models = Array.from(set);
        } else {
            delete next.models;
        }
        onChange(next);
    }, [filter, onChange]);

    const setModelSearch = useLogModelSearchStore((s) => s.setModelSearch);
    const handleClear = useCallback(() => {
        onChange(EMPTY_FILTER);
        setModelSearch('');
    }, [onChange, setModelSearch]);

    const selectTriggerClass = isMobile
        ? 'h-10 w-full text-sm min-w-0'
        : 'h-7 text-xs min-w-[7rem]';
    const compactSelectTriggerClass = isMobile
        ? 'h-10 w-full text-sm min-w-0'
        : 'h-7 text-xs min-w-[6rem]';
    const modelPickerClass = isMobile
        ? 'flex h-10 w-full items-center justify-between gap-1 rounded-md border border-border/50 bg-background px-3 text-sm hover:bg-muted transition-colors'
        : 'flex h-7 items-center gap-1 rounded-md border border-border/50 bg-background px-2 text-xs hover:bg-muted transition-colors min-w-[7rem]';

    const filtersBody = (
        <div className={cn(isMobile ? 'grid gap-3' : 'flex flex-nowrap items-center gap-2')}>
            <Select
                value={filter.channel_id != null ? String(filter.channel_id) : ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.channel_id = Number(v);
                    } else {
                        delete next.channel_id;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className={selectTriggerClass}>
                    <SelectValue placeholder={t('allChannels')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allChannels')}</SelectItem>
                    {channels.map((item) => (
                        <SelectItem key={item.raw.id} value={String(item.raw.id)}>
                            {item.raw.name}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filter.api_key_id != null ? String(filter.api_key_id) : ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.api_key_id = Number(v);
                    } else {
                        delete next.api_key_id;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className={selectTriggerClass}>
                    <SelectValue placeholder={t('allKeys')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allKeys')}</SelectItem>
                    {apiKeys.map((key) => (
                        <SelectItem key={key.id} value={String(key.id)}>
                            {key.name}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filter.endpoint_type ?? ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.endpoint_type = v;
                    } else {
                        delete next.endpoint_type;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className={selectTriggerClass}>
                    <SelectValue placeholder={t('allTypes')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allTypes')}</SelectItem>
                    {ENDPOINT_TYPE_OPTIONS.filter((o) => o.value !== '*').map((opt) => (
                        <SelectItem key={opt.value} value={opt.value}>
                            {tGroup(opt.labelKey)}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Select
                value={filter.status ?? ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v && v !== '' && v !== '__all__') {
                        next.status = v as 'success' | 'error';
                    } else {
                        delete next.status;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className={compactSelectTriggerClass}>
                    <SelectValue placeholder={t('allStatuses')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allStatuses')}</SelectItem>
                    <SelectItem value="success">{t('statusSuccess')}</SelectItem>
                    <SelectItem value="error">{t('statusError')}</SelectItem>
                </SelectContent>
            </Select>

            <Select
                value={filter.is_test != null ? String(filter.is_test) : ''}
                onValueChange={(v) => {
                    const next = { ...filter };
                    if (v === 'true') {
                        next.is_test = true;
                    } else if (v === 'false') {
                        next.is_test = false;
                    } else {
                        delete next.is_test;
                    }
                    onChange(next);
                }}
            >
                <SelectTrigger size="sm" className={compactSelectTriggerClass}>
                    <SelectValue placeholder={t('allLogs')} />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="__all__">{t('allLogs')}</SelectItem>
                    <SelectItem value="true">{t('testOnly')}</SelectItem>
                    <SelectItem value="false">{t('nonTest')}</SelectItem>
                </SelectContent>
            </Select>

            <Popover
                onOpenChange={(open) => {
                    if (open) {
                        setModelSearchTerm('');
                        requestAnimationFrame(() => modelSearchInputRef.current?.focus());
                    }
                }}
            >
                <PopoverTrigger asChild>
                    <button type="button" className={modelPickerClass}>
                        <span className="truncate">
                            {selectedModels.size > 0
                                ? `${t('modelsSelected', { count: selectedModels.size })}`
                                : t('allModels')}
                        </span>
                        <ChevronsUpDown className="size-3 shrink-0 opacity-50" />
                    </button>
                </PopoverTrigger>
                <PopoverContent align={isMobile ? 'center' : 'start'} className="w-72 p-2">
                    <div className="mb-2 flex items-center gap-2 rounded-md border border-border/50 px-2">
                        <Search className="size-3.5 text-muted-foreground" />
                        <input
                            ref={modelSearchInputRef}
                            value={modelSearchTerm}
                            onChange={(e) => setModelSearchTerm(e.target.value)}
                            placeholder={t('searchModel')}
                            className="h-8 w-full bg-transparent text-sm outline-none"
                        />
                    </div>
                    <div className="max-h-56 space-y-1 overflow-y-auto">
                        {filteredModelOptions.length === 0 ? (
                            <p className="px-2 py-3 text-xs text-muted-foreground">{t('noModels')}</p>
                        ) : (
                            filteredModelOptions.map((model) => {
                                const checked = selectedModels.has(model.name);
                                return (
                                    <button
                                        key={model.name}
                                        type="button"
                                        onClick={() => toggleModel(model.name)}
                                        className="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted"
                                    >
                                        <span className={cn(
                                            'grid size-4 place-items-center rounded border',
                                            checked ? 'border-primary bg-primary text-primary-foreground' : 'border-border'
                                        )}>
                                            {checked ? <Check className="size-3" /> : null}
                                        </span>
                                        <span className="min-w-0 truncate">{model.name}</span>
                                    </button>
                                );
                            })
                        )}
                    </div>
                    {selectedModels.size > 0 ? (
                        <button
                            type="button"
                            onClick={() => {
                                const next = { ...filter };
                                delete next.models;
                                onChange(next);
                            }}
                            className="mt-2 w-full rounded-md border border-border/50 px-2 py-1.5 text-xs text-muted-foreground hover:bg-muted"
                        >
                            {t('clearModels')}
                        </button>
                    ) : null}
                </PopoverContent>
            </Popover>

            {hasFilter ? (
                <button
                    type="button"
                    onClick={handleClear}
                    className={cn(
                        'inline-flex items-center justify-center gap-1 rounded-md border border-border/50 text-muted-foreground hover:bg-muted hover:text-foreground',
                        isMobile ? 'h-10 px-3 text-sm' : 'h-7 px-2 text-xs'
                    )}
                >
                    <X className="size-3.5" />
                    {t('clear')}
                </button>
            ) : null}

            <Popover>
                <PopoverTrigger asChild>
                    <button
                        type="button"
                        className={cn(
                            'inline-flex items-center justify-center gap-1 rounded-md border border-border/50 text-muted-foreground hover:bg-muted hover:text-foreground',
                            isMobile ? 'h-10 px-3 text-sm' : 'h-7 px-2 text-xs'
                        )}
                    >
                        <Columns3 className="size-3.5" />
                        {tView('title')}
                    </button>
                </PopoverTrigger>
                <PopoverContent align="end" className="w-64 p-2">
                    <div className="mb-2 flex items-center justify-between px-1">
                        <p className="text-xs font-semibold text-muted-foreground">{tView('title')}</p>
                        <button
                            type="button"
                            onClick={() => useLogFieldVisibilityStore.getState().resetFields()}
                            className="text-[11px] text-muted-foreground hover:bg-muted hover:text-foreground rounded px-1.5 py-0.5 transition-colors"
                        >
                            {tView('reset')}
                        </button>
                    </div>
                    <div className="grid gap-1">
                        {(Object.keys(visibility) as LogFieldName[]).map((field) => (
                            <button
                                key={field}
                                type="button"
                                onClick={() => useLogFieldVisibilityStore.getState().toggleField(field)}
                                className="flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm hover:bg-muted"
                            >
                                <span className={cn(
                                    'grid size-4 place-items-center rounded border',
                                    visibility[field] ? 'border-primary bg-primary text-primary-foreground' : 'border-border'
                                )}>
                                    {visibility[field] ? <Check className="size-3" /> : null}
                                </span>
                                {tView(field)}
                            </button>
                        ))}
                    </div>
                </PopoverContent>
            </Popover>
        </div>
    );

    if (!isMobile) {
        return (
            <div className="overflow-x-auto py-1">
                {filtersBody}
            </div>
        );
    }

    return (
        <>
            <div className="flex items-center gap-2 py-1">
                <button
                    type="button"
                    onClick={() => setMobileOpen(true)}
                    className={cn(
                        'inline-flex h-10 min-h-10 flex-1 items-center justify-center gap-2 rounded-xl border border-border bg-card px-3 text-sm font-medium text-foreground',
                        hasFilter && 'border-primary/40 bg-primary/5 text-primary'
                    )}
                >
                    <SlidersHorizontal className="size-4" />
                    <span>{t('open')}</span>
                    {activeFilterCount > 0 ? (
                        <span className="rounded-full bg-primary/15 px-2 py-0.5 text-xs tabular-nums">
                            {t('activeCount', { count: activeFilterCount })}
                        </span>
                    ) : null}
                </button>
                {hasFilter ? (
                    <button
                        type="button"
                        onClick={handleClear}
                        className="inline-flex h-10 min-h-10 items-center justify-center gap-1 rounded-xl border border-border bg-card px-3 text-sm text-muted-foreground"
                    >
                        <X className="size-4" />
                        {t('clear')}
                    </button>
                ) : null}
            </div>

            <Dialog open={mobileOpen} onOpenChange={setMobileOpen}>
                <DialogContent
                    showCloseButton={false}
                    className="top-auto bottom-0 left-0 right-0 max-h-[min(88dvh,40rem)] w-full max-w-none translate-x-0 translate-y-0 rounded-t-2xl rounded-b-none border-border p-0 sm:max-w-none"
                >
                    <div className="flex items-center justify-between border-b border-border px-4 py-3">
                        <DialogTitle className="text-base font-semibold">{t('title')}</DialogTitle>
                        <button
                            type="button"
                            onClick={() => setMobileOpen(false)}
                            className="rounded-lg px-3 py-1.5 text-sm font-medium text-primary"
                        >
                            {t('apply')}
                        </button>
                    </div>
                    <div className="max-h-[calc(min(88dvh,40rem)-3.5rem)] overflow-y-auto px-4 py-4 pb-[calc(1rem+env(safe-area-inset-bottom,0px))]">
                        {filtersBody}
                    </div>
                </DialogContent>
            </Dialog>
        </>
    );
}

/**
 * 日志页面组件
 * - 初始加载 pageSize 条历史日志
 * - SSE 实时推送新日志
 * - 滚动自动加载更多
 * - 筛选（模型、渠道、密钥、端点类型、状态）
 */
export function Log() {
    const t = useTranslations('log');
    const [filter, setFilter] = useState<LogFilter>(EMPTY_FILTER);
    const modelSearch = useLogModelSearchStore((s) => s.modelSearch);
    const setModelSearch = useLogModelSearchStore((s) => s.setModelSearch);
    const combinedFilter = useMemo<LogFilter>(
        () => (modelSearch ? { ...filter, model: modelSearch } : filter),
        [filter, modelSearch],
    );
    const { logs, hasMore, isLoading, isLoadingMore, loadMore } = useLogs({ filter: combinedFilter });
    const { data: channels = [] } = useChannelList();

    // 消费来自其它模块（分析/分组健康）的待处理筛选，实现"点击失败渠道 → 跳转日志并预填"。
    const pendingLogFilter = useNavHandoff((s) => s.pendingLogFilter);
    const consumePendingLogFilter = useNavHandoff((s) => s.consumePendingLogFilter);
    useEffect(() => {
        const pending = consumePendingLogFilter();
        if (pending) {
            const { model, ...rest } = pending;
            setFilter(rest);
            if (model) setModelSearch(model);
        }
    }, [pendingLogFilter, consumePendingLogFilter, setModelSearch]);

    const channelNameById = useMemo(() => {
        const map = new Map<number, string>();
        for (const item of channels) {
            map.set(item.raw.id, item.raw.name);
        }
        return map;
    }, [channels]);

    const canLoadMore = hasMore && !isLoading && !isLoadingMore && logs.length > 0;
    const handleReachEnd = useCallback(() => {
        if (!canLoadMore) return;
        void loadMore();
    }, [canLoadMore, loadMore]);

    const footer = useMemo(() => {
        if (hasMore && (isLoading || isLoadingMore)) {
            return (
                <div className="flex justify-center py-6">
                    <div className="flex items-center gap-2 rounded-full border border-border/50 bg-card/80 px-4 py-2 shadow-sm backdrop-blur">
                        <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                        <span className="text-xs text-muted-foreground">{t('list.loadingMore')}</span>
                    </div>
                </div>
            );
        }
        if (!hasMore && logs.length > 0) {
            return (
                <div className="flex justify-center py-6">
                    <span className="text-xs text-muted-foreground/60">{t('list.noMore')}</span>
                </div>
            );
        }
        return null;
    }, [hasMore, isLoading, isLoadingMore, logs.length, t]);

    return (
        <div className="flex h-full min-h-0 flex-col gap-2 overflow-hidden">
            <LogFilterBar filter={filter} onChange={setFilter} />
            {isLoading && logs.length === 0 ? (
                <div className="flex min-h-[18rem] items-center justify-center rounded-xl border border-border/35 bg-card">
                    <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
                </div>
            ) : logs.length === 0 ? (
                <div className="flex min-h-[18rem] items-center justify-center rounded-xl border border-dashed border-border/35 bg-card px-6 py-6 text-center">
                    <p className="text-sm text-muted-foreground">{t('list.empty')}</p>
                </div>
            ) : (
                <div className="min-h-0 flex-1">
                    <VirtualizedGrid
                        items={logs}
                        layout="list"
                        columns={{ default: 1 }}
                        estimateItemHeight={148}
                        overscan={8}
                        getItemKey={(log) => `log-${log.id}`}
                        renderItem={(log) => <LogCard log={log} channelNameById={channelNameById} />}
                        footer={footer}
                        onReachEnd={handleReachEnd}
                        reachEndEnabled={canLoadMore}
                        reachEndOffset={2}
                        bottomPaddingClassName="pb-16 md:pb-4"
                    />
                </div>
            )}
        </div>
    );
}
