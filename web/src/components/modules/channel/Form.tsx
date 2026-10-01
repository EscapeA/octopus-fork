import {
    AutoGroupType,
    ChannelType,
    RequestRewriteProfile,
    RequestRewriteHeaderProfile,
    SystemMessageStrategy,
    ToolRoleStrategy,
    type Channel,
    useChannelGroupList,
    type RequestRewriteConfig,
    useFetchModel,
    useFetchModelsPerKey,
    type KeyModelResult,
    useTestChannel,
    type TestChannelSummary,
    type ChannelProxyMode,
} from '@/api/endpoints/channel';
import { ProxySelector } from '@/components/modules/proxy-pool/ProxySelector';
import { useAlertNotifChannelList } from '@/api/endpoints/alert';
import { useSettingList, SettingKey } from '@/api/endpoints/setting';
import { usePoolList } from '@/api/endpoints/pool';
import { channelTemplates } from './templates';
import { CHANNEL_TYPE_OPTIONS } from './type-options';
import { isOpenAICompatBaseUrlSuffixMode } from './base-url-suffix';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Hint } from '@/components/ui/hint';
import {
    MorphingDialog,
    MorphingDialogClose,
    MorphingDialogContainer,
    MorphingDialogContent,
    MorphingDialogDescription,
    MorphingDialogTitle,
    MorphingDialogTrigger,
    useMorphingDialog,
} from '@/components/ui/morphing-dialog';
import { toast } from '@/components/common/Toast';
import { cn } from '@/lib/utils';
import { useIsMobile } from '@/hooks/use-mobile';
import { useTranslations } from 'next-intl';
import { useEffect, useMemo, useRef, useState } from 'react';
import { RefreshCw, X, Plus, FlaskConical, CheckCircle2, AlertTriangle, Trash2, Sparkles, Orbit, Layers3, KeyRound, Cable, Search, Check, ListFilter, ChevronRight } from 'lucide-react';
import { getModelIcon } from '@/lib/model-icons';

export interface ChannelKeyFormItem {
    id?: number;
    enabled: boolean;
    channel_key: string;
    priority?: number;
    status_code?: number;
    last_use_time_stamp?: number;
    total_cost?: number;
    remark?: string;
    supported_models?: string;
    /** 系统自动禁用标记（上游连续 402 达阈值时后端写入）；有值时展示「已自动禁用」徽标 */
    auto_disabled_at?: string | null;
    auto_disable_reason?: string;
}

export interface ChannelFormData {
    name: string;
    group_id: number;
    type: ChannelType;
    base_urls: Channel['base_urls'];
    custom_header: Channel['custom_header'];
    channel_proxy: string;
    param_override: string;
    param_override_force: boolean;
    request_rewrite: RequestRewriteConfig;
    keys: ChannelKeyFormItem[];
    model: string;
    custom_model: string;
    enabled: boolean;
    proxy_mode: ChannelProxyMode;
    proxy_config_id: number | null;
    auto_sync: boolean;
    auto_sync_key_models: boolean;
    auto_group: AutoGroupType;
    skip_model_test: boolean;
    disposable: boolean;
    expire_at: string;
    notif_channel_id: number | null;
    key_selection_strategy: string;
    match_regex: string;
    pool_id: number;
}

/**
 * 从渠道数据推导表单展示用的代理模式，镜像后端 resolveChannelProxy 的兼容语义
 *（issue #195）：历史数据可能 proxy_mode=direct 但旧 proxy 布尔仍为 true，
 * 此时按 legacy 字段回退展示，避免"开关开着但地址为空"的误导状态。
 */
export function deriveChannelProxyMode(channel: Pick<Channel, 'proxy_mode' | 'proxy' | 'channel_proxy'>): ChannelProxyMode {
    const mode = channel.proxy_mode;
    if (mode && mode !== 'direct') return mode;
    if (!channel.proxy) return 'direct';
    return channel.channel_proxy?.trim() ? 'pool' : 'system';
}

export function createDefaultRequestRewriteFormData(): RequestRewriteConfig {
    return {
        enabled: false,
        profile: RequestRewriteProfile.Preserve,
        tool_role_strategy: ToolRoleStrategy.Keep,
        system_message_strategy: SystemMessageStrategy.Keep,
        header_profile: RequestRewriteHeaderProfile.None,
    };
}

export function normalizeRequestRewriteFormData(config?: RequestRewriteConfig | null): RequestRewriteConfig {
    return {
        enabled: config?.enabled ?? false,
        profile: config?.profile ?? RequestRewriteProfile.Preserve,
        tool_role_strategy: config?.tool_role_strategy ?? ToolRoleStrategy.Keep,
        system_message_strategy: config?.system_message_strategy ?? SystemMessageStrategy.Keep,
        header_profile: config?.header_profile ?? RequestRewriteHeaderProfile.None,
    };
}

export function isRequestRewriteSupportedChannelType(channelType: ChannelType): boolean {
    return channelType === ChannelType.OpenAIChat || channelType === ChannelType.MiMoChat || channelType === ChannelType.OpenAIResponse;
}

export function getEffectiveRequestRewriteFormData(channelType: ChannelType, config?: RequestRewriteConfig | null): RequestRewriteConfig {
    const normalized = normalizeRequestRewriteFormData(config);
    if (isRequestRewriteSupportedChannelType(channelType)) {
        return normalized;
    }

    return {
        ...normalized,
        enabled: false,
    };
}

function hasManualVersionSuffix(rawUrl: string): boolean {
    const normalized = rawUrl.trim().split(/[?#]/)[0].replace(/\/+$/, '').toLowerCase();
    return /\/(v\d+(?:beta)?|api\/v\d+)$/.test(normalized);
}

export interface ChannelFormProps {
    formData: ChannelFormData;
    onFormDataChange: (data: ChannelFormData) => void;
    onSubmit: (event: React.FormEvent<HTMLFormElement>) => void;
    isPending: boolean;
    submitText: string;
    pendingText: string;
    onCancel?: () => void;
    cancelText?: string;
    idPrefix?: string;
    showTemplatePicker?: boolean;
    onShowTemplatePicker?: () => void;
    layout?: 'default' | 'create';
}

import {
    Accordion,
    AccordionContent,
    AccordionItem,
    AccordionTrigger,
} from "@/components/ui/accordion";

function SectionHeader({
    icon: Icon,
    title,
    hint,
}: {
    icon: typeof Sparkles;
    title: string;
    hint?: string;
}) {
    return (
        <h3 className="flex min-w-0 items-center gap-2 text-sm font-semibold text-foreground">
            <Icon className="size-4 shrink-0 text-muted-foreground" />
            <span>{title}</span>
            {hint ? <Hint text={hint} /> : null}
        </h3>
    );
}

interface PerKeyModelItem {
    /** formData.keys 下标（写回目标）；-1 表示探测结果与当前表单 key 对不上，不提供写回 */
    keyIndex: number;
    keyMasked: string;
    keyRemark?: string;
    passed: boolean;
    message?: string;
    /** 该 key 探测到的模型 */
    models: string[];
    /** 该 key 当前 supported_models（默认勾选；空数组 = 不限） */
    currentModels: string[];
}

interface ModelPickerDialogPanelProps {
    models: string[];
    draftSelected: string[];
    onDraftChange: (models: string[]) => void;
    isLoading: boolean;
    onApply: () => void;
    perKeyItems?: PerKeyModelItem[] | null;
    perKeyLoading?: boolean;
    onApplyPerKey?: (selection: Record<number, string[]>) => void;
}

interface ModelProviderGroup {
    label: string;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    Avatar: React.ComponentType<any>;
    color: string;
    models: string[];
}

function groupModelsByProvider(models: string[]): ModelProviderGroup[] {
    const groupMap = new Map<string, { label: string; Avatar: ModelProviderGroup['Avatar']; color: string; models: string[] }>();

    for (const model of models) {
        const { label, Avatar, color } = getModelIcon(model);
        const existing = groupMap.get(label);
        if (existing) {
            existing.models.push(model);
        } else {
            groupMap.set(label, { label, Avatar, color, models: [model] });
        }
    }

    return Array.from(groupMap.values()).sort((a, b) => {
        if (a.label === 'Model') return 1;
        if (b.label === 'Model') return -1;
        return a.label.localeCompare(b.label);
    });
}

function ModelPickerDialogPanel({ models, draftSelected, onDraftChange, isLoading, onApply, perKeyItems, perKeyLoading, onApplyPerKey }: ModelPickerDialogPanelProps) {
    const t = useTranslations('channel.form.modelPicker');
    const { setIsOpen } = useMorphingDialog();
    const [searchTerm, setSearchTerm] = useState('');
    const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
    const [viewMode, setViewMode] = useState<'all' | 'perKey'>('all');
    // 每 key 独立勾选（键 = formData.keys 下标，值 = 该 key 限定的模型；[] 表示不限）
    const [perKeyDraft, setPerKeyDraft] = useState<Record<number, string[]>>({});
    const hasPerKeyData = !!perKeyItems && perKeyItems.length > 0;
    const showPerKey = perKeyLoading || hasPerKeyData;

    // 探测结果异步到达：只给「用户还没动过」的 key 做默认勾选（= 该 key 现有 supported_models）
    useEffect(() => {
        if (!perKeyItems || perKeyItems.length === 0) return;
        setPerKeyDraft((prev) => {
            let changed = false;
            const next = { ...prev };
            for (const item of perKeyItems) {
                if (item.keyIndex < 0) continue;
                if (next[item.keyIndex] === undefined) {
                    next[item.keyIndex] = item.currentModels;
                    changed = true;
                }
            }
            return changed ? next : prev;
        });
    }, [perKeyItems]);

    const perKeySelectionOf = (item: PerKeyModelItem): string[] => perKeyDraft[item.keyIndex] ?? item.currentModels;

    const togglePerKeyModel = (item: PerKeyModelItem, model: string) => {
        if (item.keyIndex < 0) return;
        setPerKeyDraft((prev) => {
            const current = prev[item.keyIndex] ?? item.currentModels;
            const next = current.includes(model) ? current.filter((m) => m !== model) : [...current, model];
            return { ...prev, [item.keyIndex]: next };
        });
    };

    const limitedKeyCount = (perKeyItems ?? []).filter(
        (item) => item.keyIndex >= 0 && perKeySelectionOf(item).length > 0
    ).length;

    const normalizedSearch = searchTerm.trim().toLowerCase();
    const isSearching = normalizedSearch.length > 0;

    const groups = useMemo(() => groupModelsByProvider(models), [models]);

    const filteredGroups = useMemo(() => {
        if (!isSearching) return groups;
        return groups
            .map((group) => ({
                ...group,
                models: group.models.filter((m) => m.toLowerCase().includes(normalizedSearch)),
            }))
            .filter((group) => group.models.length > 0);
    }, [groups, normalizedSearch, isSearching]);

    const filteredModels = useMemo(
        () => filteredGroups.flatMap((g) => g.models),
        [filteredGroups]
    );

    const selectedSet = new Set(draftSelected);
    const allFilteredSelected = filteredModels.length > 0 && filteredModels.every((m) => selectedSet.has(m));

    const toggleModel = (model: string) => {
        onDraftChange(
            draftSelected.includes(model)
                ? draftSelected.filter((item) => item !== model)
                : [...draftSelected, model]
        );
    };

    const toggleGroupSelection = (groupModels: string[]) => {
        const currentSet = new Set(draftSelected);
        const allSelected = groupModels.every((m) => currentSet.has(m));
        if (allSelected) {
            const removeSet = new Set(groupModels);
            onDraftChange(draftSelected.filter((m) => !removeSet.has(m)));
        } else {
            onDraftChange(Array.from(new Set([...draftSelected, ...groupModels])));
        }
    };

    const toggleGroupCollapsed = (label: string) => {
        setCollapsedGroups((prev) => ({ ...prev, [label]: !prev[label] }));
    };

    const handleSelectFiltered = () => {
        if (filteredModels.length === 0) return;
        const currentSet = new Set(draftSelected);
        if (filteredModels.every((model) => currentSet.has(model))) {
            onDraftChange(draftSelected.filter((model) => !filteredModels.includes(model)));
        } else {
            onDraftChange(Array.from(new Set([...draftSelected, ...filteredModels])));
        }
    };

    const handleApply = () => {
        if (viewMode === 'perKey' && onApplyPerKey) {
            // 「按 Key 查看」：写回每个 key 的 supported_models（[] = 不限）
            const selection: Record<number, string[]> = {};
            for (const item of perKeyItems ?? []) {
                if (item.keyIndex < 0) continue;
                selection[item.keyIndex] = perKeySelectionOf(item);
            }
            onApplyPerKey(selection);
        } else {
            onApply();
        }
        setIsOpen(false);
    };

    return (
        <div className="relative flex h-full min-h-0 flex-col overflow-hidden rounded-xl border border-border/35 bg-card text-card-foreground shadow-md">
            <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_16%_14%,color-mix(in_oklch,var(--primary)_18%,transparent)_0%,transparent_30%),linear-gradient(180deg,color-mix(in_oklch,white_18%,transparent),transparent_28%)]" />
            <MorphingDialogTitle className="shrink-0">
                <header className="relative flex items-center justify-between gap-4 border-b border-border/20 px-5 py-4 md:px-6">
                    <div className="min-w-0 space-y-2">
                        <div className="flex items-center gap-2">
                            <span className="h-2.5 w-10 rounded-full bg-primary/18 shadow-sm" />
                            <span className="h-2.5 w-20 rounded-full bg-card shadow-inner" />
                        </div>
                        <div className="space-y-1">
                            <h3 className="truncate text-lg font-semibold tracking-tight text-card-foreground md:text-xl">
                                {t('title')}
                            </h3>
                            <p className="text-xs text-muted-foreground">
                                {t('description', { count: models.length })}
                            </p>
                        </div>
                    </div>
                    <MorphingDialogClose className="relative right-0 top-0" />
                </header>
            </MorphingDialogTitle>

            <MorphingDialogDescription disableLayoutAnimation className="relative flex min-h-0 flex-1 flex-col gap-4 px-4 py-4 md:px-6">
                {showPerKey && (
                    <div className="flex shrink-0 items-center gap-1 rounded-lg border border-border/25 bg-muted/40 p-1">
                        <button
                            type="button"
                            onClick={() => setViewMode('all')}
                            className={`flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${viewMode === 'all' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                        >
                            {t('allModels')}
                        </button>
                        <button
                            type="button"
                            onClick={() => setViewMode('perKey')}
                            className={`flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${viewMode === 'perKey' ? 'bg-card text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'}`}
                        >
                            {t('perKey')}
                        </button>
                    </div>
                )}
                {viewMode === 'all' && (
                    <>
                <div className="relative shrink-0">
                    <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        value={searchTerm}
                        onChange={(event) => setSearchTerm(event.target.value)}
                        placeholder={t('searchPlaceholder')}
                        className="h-11 rounded-lg pl-9"
                    />
                </div>

                <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <Badge variant="secondary" className="rounded-full">
                            {t('selectedCount', { count: draftSelected.length })}
                        </Badge>
                        <span>{t('filteredCount', { count: filteredModels.length })}</span>
                    </div>
                    <div className="flex items-center gap-2">
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => onDraftChange([])}
                            disabled={draftSelected.length === 0}
                            className="h-8 rounded-lg px-2 text-xs text-muted-foreground hover:text-foreground"
                        >
                            {t('clear')}
                        </Button>
                        <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            onClick={handleSelectFiltered}
                            disabled={filteredModels.length === 0}
                            className="h-8 rounded-lg px-2 text-xs"
                        >
                            <ListFilter className="size-3.5" />
                            {allFilteredSelected ? t('unselectFiltered') : t('selectFiltered')}
                        </Button>
                    </div>
                </div>

                <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border border-border/25 bg-card p-2 shadow-sm">
                    {isLoading ? (
                        <div className="flex h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
                            <RefreshCw className="size-4 animate-spin" />
                            {t('loading')}
                        </div>
                    ) : filteredGroups.length > 0 ? (
                        <div className="flex flex-col gap-1">
                            {filteredGroups.map((group) => {
                                const isCollapsed = !isSearching && collapsedGroups[group.label];
                                const groupSelectedCount = group.models.filter((m) => selectedSet.has(m)).length;
                                const allGroupSelected = group.models.length > 0 && groupSelectedCount === group.models.length;
                                const Avatar = group.Avatar;

                                return (
                                    <div key={group.label} className="rounded-lg">
                                        <div className="flex items-center gap-2 px-1 py-1.5">
                                            <button
                                                type="button"
                                                onClick={() => toggleGroupCollapsed(group.label)}
                                                className="flex size-5 shrink-0 items-center justify-center rounded text-muted-foreground/60 hover:text-foreground transition-colors"
                                            >
                                                <ChevronRight
                                                    className={`size-3.5 transition-transform duration-150 ${isCollapsed ? '' : 'rotate-90'}`}
                                                />
                                            </button>
                                            <Avatar className="size-4 shrink-0" />
                                            <span className="text-xs font-medium text-foreground">{group.label}</span>
                                            <Badge variant="secondary" className="h-4 rounded-full px-1.5 text-[0.625rem] tabular-nums">
                                                {groupSelectedCount}/{group.models.length}
                                            </Badge>
                                            <button
                                                type="button"
                                                onClick={() => toggleGroupSelection(group.models)}
                                                className={`ml-auto shrink-0 rounded px-2 py-0.5 text-[0.625rem] font-medium transition-colors ${
                                                    allGroupSelected
                                                        ? 'text-primary hover:text-primary/70'
                                                        : 'text-muted-foreground/60 hover:text-foreground'
                                                }`}
                                            >
                                                {allGroupSelected ? t('unselectFiltered') : t('selectFiltered')}
                                            </button>
                                        </div>
                                        {!isCollapsed && (
                                            <div className="grid gap-2 sm:grid-cols-2 pl-7 pb-1">
                                                {group.models.map((model) => {
                                                    const selected = selectedSet.has(model);
                                                    return (
                                                        <button
                                                            key={model}
                                                            type="button"
                                                            onClick={() => toggleModel(model)}
                                                            className={`flex min-w-0 items-center gap-3 rounded-lg border px-3 py-2.5 text-left text-sm transition-colors ${
                                                                selected
                                                                    ? 'border-primary/30 bg-primary/10 text-foreground'
                                                                    : 'border-border/25 bg-background/40 text-muted-foreground hover:border-border/60 hover:text-foreground'
                                                            }`}
                                                        >
                                                            <span className={`flex size-5 shrink-0 items-center justify-center rounded-md border ${
                                                                selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border bg-card'
                                                            }`}>
                                                                {selected ? <Check className="size-3.5" /> : null}
                                                            </span>
                                                            <span className="min-w-0 flex-1 truncate font-mono" title={model}>
                                                                {model}
                                                            </span>
                                                        </button>
                                                    );
                                                })}
                                            </div>
                                        )}
                                    </div>
                                );
                            })}
                        </div>
                    ) : (
                        <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
                            {models.length === 0 ? t('empty') : t('noSearchResult')}
                        </div>
                    )}
                </div>
                    </>
                )}
                {viewMode === 'perKey' && (
                    <div className="min-h-0 flex-1 space-y-2 overflow-y-auto rounded-lg border border-border/25 bg-card p-2 shadow-sm">
                        {perKeyLoading ? (
                            <div className="flex h-40 items-center justify-center gap-2 text-sm text-muted-foreground">
                                <RefreshCw className="size-4 animate-spin" />
                                {t('loading')}
                            </div>
                        ) : hasPerKeyData ? (
                            <>
                                <p className="px-1 pt-1 text-[0.68rem] leading-relaxed text-muted-foreground">{t('perKeyHint')}</p>
                                <div className="flex flex-col gap-3">
                                    {perKeyItems!.map((item, idx) => {
                                        const selection = perKeySelectionOf(item);
                                        const selectionSet = new Set(selection);
                                        // 已被限定但本次探测未返回的模型也展示，避免无声丢失
                                        const chipModels = [...item.models, ...selection.filter((m) => !item.models.includes(m))];
                                        const limited = selection.length > 0;
                                        const unmappable = item.keyIndex < 0;
                                        return (
                                            <div
                                                key={`${item.keyMasked}-${idx}`}
                                                className={`rounded-lg border p-3 ${item.passed ? 'border-border/25 bg-background/40' : 'border-red-500/20 bg-red-500/5'}`}
                                            >
                                                <div className="mb-2 flex flex-wrap items-center gap-2">
                                                    {item.passed ? (
                                                        <CheckCircle2 className="size-4 shrink-0 text-green-500" />
                                                    ) : (
                                                        <AlertTriangle className="size-4 shrink-0 text-red-400" />
                                                    )}
                                                    <span className="truncate text-xs font-mono text-foreground">
                                                        {item.keyMasked}
                                                    </span>
                                                    {item.keyRemark && (
                                                        <Badge variant="secondary" className="h-4 rounded-full px-1.5 text-[0.625rem]">
                                                            {item.keyRemark}
                                                        </Badge>
                                                    )}
                                                    <Badge variant="secondary" className="h-4 rounded-full px-1.5 text-[0.625rem]">
                                                        {limited ? t('perKeyLimited', { count: selection.length }) : t('perKeyUnlimited')}
                                                    </Badge>
                                                    {!unmappable && (
                                                        <div className="ml-auto flex items-center gap-1">
                                                            <button
                                                                type="button"
                                                                onClick={() => setPerKeyDraft((prev) => ({ ...prev, [item.keyIndex]: chipModels }))}
                                                                className="rounded px-2 py-0.5 text-[0.625rem] font-medium text-muted-foreground/60 transition-colors hover:text-foreground"
                                                            >
                                                                {t('perKeySelectAll')}
                                                            </button>
                                                            <button
                                                                type="button"
                                                                onClick={() => setPerKeyDraft((prev) => ({ ...prev, [item.keyIndex]: [] }))}
                                                                className="rounded px-2 py-0.5 text-[0.625rem] font-medium text-muted-foreground/60 transition-colors hover:text-foreground"
                                                            >
                                                                {t('perKeyClear')}
                                                            </button>
                                                        </div>
                                                    )}
                                                </div>
                                                {chipModels.length > 0 ? (
                                                    <div className="flex flex-wrap gap-1.5">
                                                        {chipModels.map((model) => {
                                                            const selected = selectionSet.has(model);
                                                            const extra = !item.models.includes(model);
                                                            return (
                                                                <button
                                                                    key={model}
                                                                    type="button"
                                                                    disabled={unmappable}
                                                                    title={extra ? t('perKeyNotProbed') : model}
                                                                    onClick={() => togglePerKeyModel(item, model)}
                                                                    className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-mono transition-colors ${
                                                                        selected
                                                                            ? 'border-primary/30 bg-primary/10 text-foreground'
                                                                            : 'border-border/25 bg-background/40 text-muted-foreground hover:border-border/60 hover:text-foreground'
                                                                    } ${extra ? 'border-dashed' : ''} ${unmappable ? 'cursor-not-allowed opacity-60' : ''}`}
                                                                >
                                                                    <span className={`flex size-3.5 shrink-0 items-center justify-center rounded-full ${
                                                                        selected ? 'bg-primary text-primary-foreground' : 'border border-border'
                                                                    }`}>
                                                                        {selected ? <Check className="size-2.5" /> : null}
                                                                    </span>
                                                                    {model}
                                                                </button>
                                                            );
                                                        })}
                                                    </div>
                                                ) : !item.passed && item.message ? (
                                                    <p className="text-xs text-red-400">{item.message}</p>
                                                ) : (
                                                    <p className="text-xs text-muted-foreground">{t('noModels')}</p>
                                                )}
                                                {unmappable && (
                                                    <p className="mt-2 text-xs text-orange-400">{t('perKeyUnmappable')}</p>
                                                )}
                                            </div>
                                        );
                                    })}
                                </div>
                            </>
                        ) : (
                            <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
                                {t('empty')}
                            </div>
                        )}
                    </div>
                )}

                <div className="flex shrink-0 flex-col gap-2 border-t border-border/20 pt-4 sm:flex-row">
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={() => setIsOpen(false)}
                        className="h-11 rounded-lg sm:flex-1"
                    >
                        {t('cancel')}
                    </Button>
                    <Button
                        type="button"
                        onClick={handleApply}
                        className="h-11 rounded-lg sm:flex-1"
                    >
                        {viewMode === 'perKey'
                            ? t('applyPerKey', { count: limitedKeyCount })
                            : t('apply', { count: draftSelected.length })}
                    </Button>
                </div>
            </MorphingDialogDescription>
        </div>
    );
}

export function TemplatePickerGrid({
    onApplyTemplate,
}: {
    onApplyTemplate: (templateKey: string) => void;
}) {
    const t = useTranslations('channel.form');

    return (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
            {channelTemplates.map((template) => (
                <Button
                    key={template.key}
                    type="button"
                    variant="outline"
                    onClick={() => onApplyTemplate(template.key)}
                    className="h-auto min-h-20 flex-col items-start gap-1 rounded-lg border-border/30 bg-card px-3.5 py-3 text-left whitespace-normal hover:bg-card md:min-h-24 md:rounded-lg md:px-4"
                >
                    <span className="text-sm font-semibold">{template.name}</span>
                    <span className="text-xs text-muted-foreground">{t(template.descriptionKey)}</span>
                </Button>
            ))}
        </div>
    );
}

/**
 * 移动端高密度版模板选择器：下拉 Select 替代卡片网格，
 * 选择即应用，触发区显示当前已选模板名。
 */
export function TemplatePickerSelect({
    onApplyTemplate,
}: {
    onApplyTemplate: (templateKey: string) => void;
}) {
    const t = useTranslations('channel.form');
    const [selected, setSelected] = useState('');

    return (
        <Select
            value={selected}
            onValueChange={(value) => {
                setSelected(value);
                onApplyTemplate(value);
            }}
        >
            <SelectTrigger
                id="channel-template-select"
                className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
                <SelectValue placeholder={t('template.selectPlaceholder')} />
            </SelectTrigger>
            <SelectContent className="max-h-80 min-w-0 rounded-lg" style={{ width: 'var(--radix-select-trigger-width)' }}>
                {channelTemplates.map((template) => (
                    <SelectItem key={template.key} className="rounded-xl py-2.5" value={template.key}>
                        <span className="flex min-w-0 items-center gap-2">
                            <span className="shrink-0 font-medium">{template.name}</span>
                            <span className="truncate text-xs text-muted-foreground">{t(template.descriptionKey)}</span>
                        </span>
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    );
}

export function ChannelForm({
    formData,
    onFormDataChange,
    onSubmit,
    isPending,
    submitText,
    pendingText,
    onCancel,
    cancelText,
    idPrefix = 'channel',
    showTemplatePicker = true,
    onShowTemplatePicker,
    layout = 'default',
}: ChannelFormProps) {
    const t = useTranslations('channel.form');
    const isMobile = useIsMobile();
    const { data: settings } = useSettingList();
    const { data: channelGroups = [] } = useChannelGroupList();
    const { data: notifChannels = [] } = useAlertNotifChannelList();
    const { data: pools = [] } = usePoolList();
    const requestRewriteSupported = isRequestRewriteSupportedChannelType(formData.type);
    const isCreateLayout = layout === 'create';
    const sectionClassName = isCreateLayout
        ? 'min-w-0 space-y-3 border-b border-border pb-5'
        : 'space-y-4 rounded-lg bg-card/70 p-4 md:p-5';
    const labelClassName = 'text-sm font-medium text-card-foreground';
    const fieldGroupClassName = 'space-y-2';

    const globalKeyStrategy = settings?.find((s) => s.key === SettingKey.KeySelectionStrategy)?.value ?? 'cost';
    const effectiveKeyStrategy = formData.key_selection_strategy || globalKeyStrategy;
    const showPriorityInput = effectiveKeyStrategy === 'priority';

    // Ensure the form always shows at least 1 row for base_urls / keys / custom_header.
    // This avoids "empty list" UI and also keeps URL + APIKEY layout consistent.
    useEffect(() => {
        if (!formData.base_urls || formData.base_urls.length === 0) {
            onFormDataChange({ ...formData, base_urls: [{ url: '', delay: 0, suffix_mode: 'openai_compat' }] });
            return;
        }
        if (!formData.keys || formData.keys.length === 0) {
            onFormDataChange({ ...formData, keys: [{ enabled: true, channel_key: '', priority: 0 }] });
            return;
        }
        if (!formData.custom_header || formData.custom_header.length === 0) {
            onFormDataChange({ ...formData, custom_header: [{ header_key: '', header_value: '' }] });
        }
    }, [formData, onFormDataChange]);

    useEffect(() => {
        if (formData.group_id !== 0 || channelGroups.length === 0) {
            return;
        }
        const defaultGroup = channelGroups.find((item) => item.is_default) ?? channelGroups[0];
        if (!defaultGroup) {
            return;
        }
        onFormDataChange({ ...formData, group_id: defaultGroup.id });
    }, [channelGroups, formData, onFormDataChange]);

    const autoModels = formData.model
        ? formData.model.split(',').map((m) => m.trim()).filter(Boolean)
        : [];
    const customModels = formData.custom_model
        ? formData.custom_model.split(',').map((m) => m.trim()).filter(Boolean)
        : [];
    const [inputValue, setInputValue] = useState('');
    const [fetchedModels, setFetchedModels] = useState<string[]>([]);
    const inputRef = useRef<HTMLInputElement>(null);

    const fetchModel = useFetchModel();
    const fetchModelsPerKey = useFetchModelsPerKey();
    const testChannel = useTestChannel();
    const [testSummary, setTestSummary] = useState<TestChannelSummary | null>(null);
    const [modelPickerDraft, setModelPickerDraft] = useState<string[]>([]);
    const [perKeyResults, setPerKeyResults] = useState<KeyModelResult[] | null>(null);
    // 与 perKeyResults 同序：每个结果对应 formData.keys 的下标
    const [perKeyKeyIndexes, setPerKeyKeyIndexes] = useState<number[]>([]);

    const effectiveKey =
        formData.keys.find((k) => k.enabled && k.channel_key.trim())?.channel_key.trim() || '';

    const updateModels = (nextAuto: string[], nextCustom: string[]) => {
        const model = nextAuto.join(',');
        const custom_model = nextCustom.join(',');
        if (formData.model === model && formData.custom_model === custom_model) return;
        onFormDataChange({ ...formData, model, custom_model });
    };

    const applyFetchedModelSelection = () => {
        const customModelSet = new Set(customModels);
        const nextAutoModels = Array.from(new Set(modelPickerDraft)).filter((model) => !customModelSet.has(model));
        updateModels(nextAutoModels, customModels);
        toast.success(t('modelPicker.applySuccess', { count: nextAutoModels.length }));
    };

    const normalizeFetchedModels = (data: unknown): string[] => {
        if (!Array.isArray(data)) return [];

        return Array.from(new Set(
            data
                .map((item) => {
                    if (typeof item === 'string') return item.trim();
                    if (!item || typeof item !== 'object') return '';

                    const candidate =
                        ('id' in item && typeof item.id === 'string' && item.id) ||
                        ('name' in item && typeof item.name === 'string' && item.name) ||
                        ('display_name' in item && typeof item.display_name === 'string' && item.display_name) ||
                        ('displayName' in item && typeof item.displayName === 'string' && item.displayName) ||
                        '';

                    return candidate.trim();
                })
                .filter(Boolean)
        ));
    };

    const normalizedHeaders = useMemo(() =>
        (formData.custom_header ?? [])
            .map((h) => ({ header_key: h.header_key.trim(), header_value: h.header_value }))
            .filter((h) => h.header_key && h.header_value !== ''),
        [formData.custom_header]
    );

    const buildTestPayload = () => ({
        type: formData.type,
        base_urls: (formData.base_urls ?? []).filter((u) => u.url.trim()).map((u) => ({
            url: u.url.trim(),
            delay: Number(u.delay || 0),
            suffix_mode: u.suffix_mode && u.suffix_mode !== 'auto' ? u.suffix_mode : undefined,
        })),
        keys: formData.keys
            .filter((k) => k.channel_key.trim())
            .map((k) => ({ enabled: k.enabled, channel_key: k.channel_key.trim(), remark: k.remark ?? '' })),
        proxy_mode: formData.proxy_mode,
        proxy_config_id: formData.proxy_mode === 'pool' ? formData.proxy_config_id : null,
        proxy: formData.proxy_mode !== 'direct',
        channel_proxy: formData.channel_proxy?.trim() || '',
        match_regex: formData.match_regex.trim() || '',
        custom_header: normalizedHeaders,
        model: formData.model,
        custom_model: formData.custom_model,
        name: formData.name,
        enabled: formData.enabled,
        auto_sync: formData.auto_sync,
        auto_group: formData.auto_group,
        param_override: formData.param_override.trim() || '',
        param_override_force: formData.param_override_force,
    });

    const handleTestChannel = () => {
        setTestSummary(null);
        testChannel.mutate(buildTestPayload(), {
            onSuccess: (data) => {
                setTestSummary(data);
                toast.success(data.passed ? t('test.success') : t('test.partialSuccess'));
            },
            onError: (error) => {
                const errorMessage = error instanceof Error ? error.message : String(error);
                toast.error(t('test.failed'), { description: errorMessage });
            },
        });
    };

    const maskKey = (secret: string): string => {
        const trimmed = secret.trim();
        if (!trimmed) return '';
        if (trimmed.length <= 8) return trimmed;
        return trimmed.slice(0, 4) + '...' + trimmed.slice(-4);
    };

    // 「按 Key 查看」的可写回项：把探测结果与表单里的 key 行对应起来。
    // 后端 helper.FetchModelsPerKey 按请求顺序返回，请求 keys 与 perKeyKeyIndexes 同序，
    // 这里再用脱敏值复核一次，对不上（key 变动/顺序错位）就标 -1，不提供写回。
    const perKeyItems = useMemo<PerKeyModelItem[] | null>(() => {
        if (!perKeyResults || perKeyResults.length === 0) return null;
        return perKeyResults.map((result, idx) => {
            const keyIndex = perKeyKeyIndexes[idx] ?? -1;
            const key = keyIndex >= 0 ? formData.keys[keyIndex] : undefined;
            const matched = !!key && maskKey(key.channel_key) === (result.key_masked ?? '');
            return {
                keyIndex: matched ? keyIndex : -1,
                keyMasked: result.key_masked ?? '',
                keyRemark: result.key_remark,
                passed: result.passed,
                message: result.message,
                models: result.models ?? [],
                currentModels: key?.supported_models
                    ? key.supported_models.split(',').map((m) => m.trim()).filter(Boolean)
                    : [],
            };
        });
    }, [perKeyResults, perKeyKeyIndexes, formData.keys]);

    // 写回：每个 key 的 supported_models = 该 key 卡片上勾选的模型（空 = 不限）；
    // 并把各 key 限定模型的并集并入渠道模型列表（否则这些模型不会被路由到该渠道）。
    const applyPerKeySelection = (selection: Record<number, string[]>) => {
        const nextKeys = formData.keys.map((k, i) =>
            selection[i] === undefined ? k : { ...k, supported_models: selection[i].join(',') }
        );
        const union = Array.from(new Set(Object.values(selection).flat()));
        const customSet = new Set(customModels);
        const nextAutoModels = Array.from(new Set([...autoModels, ...union])).filter((m) => !customSet.has(m));
        onFormDataChange({
            ...formData,
            keys: nextKeys,
            model: nextAutoModels.join(','),
            custom_model: formData.custom_model,
        });
        const limited = Object.values(selection).filter((models) => models.length > 0).length;
        toast.success(t('modelPicker.perKeyApplySuccess', { keys: limited, models: union.length }));
    };

    const handleRemoveFailedKeys = () => {
        if (!testSummary) return;
        const failedIds = new Set<string>();
        for (const result of testSummary.results) {
            if (!result.passed) {
                failedIds.add(`${result.key_masked ?? ''}||${result.key_remark ?? ''}`);
            }
        }
        if (failedIds.size === 0) return;
        const nextKeys = formData.keys.filter((k) => {
            const id = `${maskKey(k.channel_key)}||${k.remark ?? ''}`;
            return !failedIds.has(id);
        });
        if (nextKeys.length === 0) {
            nextKeys.push({ enabled: true, channel_key: '' });
        }
        onFormDataChange({ ...formData, keys: nextKeys });
        setTestSummary(null);
    };

    const handleRefreshModels = async () => {
        if (!formData.base_urls?.[0]?.url || !effectiveKey) return;
        setModelPickerDraft(autoModels);
        setFetchedModels([]);
        setPerKeyResults(null);
        setPerKeyKeyIndexes([]);

        const payload = {
            type: formData.type,
            base_urls: formData.base_urls,
            keys: formData.keys
                .filter((k) => k.channel_key.trim())
                .map((k) => ({ enabled: k.enabled, channel_key: k.channel_key.trim() })),
            proxy_mode: formData.proxy_mode,
            proxy_config_id: formData.proxy_mode === 'pool' ? formData.proxy_config_id : null,
            proxy: formData.proxy_mode !== 'direct',
            channel_proxy: formData.channel_proxy?.trim() || '',
            match_regex: formData.match_regex.trim() || '',
            custom_header: normalizedHeaders,
        };

        // 主模型列表（单 key）
        fetchModel.mutate(
            payload,
            {
                onSuccess: (data) => {
                    const normalizedModels = normalizeFetchedModels(data);
                    if (normalizedModels.length > 0) {
                        setFetchedModels(normalizedModels);
                        toast.success(t('modelRefreshSuccess', { count: normalizedModels.length }));
                    } else {
                        setFetchedModels([]);
                        toast.warning(t('modelRefreshEmpty'));
                    }
                },
                onError: (error) => {
                    setFetchedModels([]);
                    const errorMessage = error instanceof Error ? error.message : String(error);
                    toast.error(t('modelRefreshFailed'), { description: errorMessage });
                },
            }
        );

        // 如果有多个 key，同时拉取每个 key 的模型列表。
        // 结果与请求 keys 同序返回；这里记下每个结果对应 formData.keys 的下标，
        // 「按 Key 查看」写回 supported_models 时按该下标定位 key 行。
        const enabledKeyTargets = formData.keys
            .map((k, i) => ({ k, i }))
            .filter(({ k }) => k.enabled && k.channel_key.trim());
        if (enabledKeyTargets.length > 1) {
            fetchModelsPerKey.mutate(
                {
                    ...payload,
                    // 带上 id / remark：后端在结果的 key_id / key_remark 里原样回传（上游 v2.6.1 起），
                    // 便于「按 Key 查看」写回时精确定位到具体的 key 行。
                    keys: enabledKeyTargets.map(({ k }) => ({
                        id: k.id,
                        enabled: true,
                        channel_key: k.channel_key.trim(),
                        remark: k.remark ?? '',
                    })),
                },
                {
                    onSuccess: (data) => {
                        setPerKeyResults(data.results);
                        setPerKeyKeyIndexes(enabledKeyTargets.map(({ i }) => i));
                    },
                    onError: () => {
                        setPerKeyResults(null);
                        setPerKeyKeyIndexes([]);
                    },
                }
            );
        }
    };

    const handleAddModel = (model: string) => {
        const trimmedModel = model.trim();
        if (trimmedModel && !customModels.includes(trimmedModel) && !autoModels.includes(trimmedModel)) {
            updateModels(autoModels, [...customModels, trimmedModel]);
        }
        setInputValue('');
    };

    const handleRemoveAutoModel = (model: string) => {
        updateModels(autoModels.filter(m => m !== model), customModels);
    };

    const handleRemoveCustomModel = (model: string) => {
        updateModels(autoModels, customModels.filter(m => m !== model));
    };

    const handleInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
        if (e.key === 'Enter') {
            e.preventDefault();
            if (inputValue.trim()) handleAddModel(inputValue);
        }
    };

    const handleAddKey = () => {
        onFormDataChange({
            ...formData,
            keys: [...formData.keys, { enabled: true, channel_key: '', priority: 0 }],
        });
    };

    const handleUpdateKey = (idx: number, patch: Partial<ChannelKeyFormItem>) => {
        const next = formData.keys.map((k, i) => (i === idx ? { ...k, ...patch } : k));
        onFormDataChange({ ...formData, keys: next });
    };

    const handleRemoveKey = (idx: number) => {
        const curr = formData.keys ?? [];
        if (curr.length <= 1) return;
        const next = curr.filter((_, i) => i !== idx);
        onFormDataChange({ ...formData, keys: next });
    };

    const handleAddBaseUrl = () => {
        onFormDataChange({
            ...formData,
            base_urls: [...(formData.base_urls ?? []), { url: '', delay: 0, suffix_mode: 'openai_compat' }],
        });
    };

    const handleUpdateBaseUrl = (idx: number, patch: Partial<Channel['base_urls'][number]>) => {
        const next = (formData.base_urls ?? []).map((u, i) => (i === idx ? { ...u, ...patch } : u));
        onFormDataChange({ ...formData, base_urls: next });
    };

    const handleRemoveBaseUrl = (idx: number) => {
        const curr = formData.base_urls ?? [];
        if (curr.length <= 1) return;
        onFormDataChange({ ...formData, base_urls: curr.filter((_, i) => i !== idx) });
    };

    const handleAddHeader = () => {
        onFormDataChange({
            ...formData,
            custom_header: [...(formData.custom_header ?? []), { header_key: '', header_value: '' }],
        });
    };

    const handleUpdateHeader = (idx: number, patch: Partial<Channel['custom_header'][number]>) => {
        const next = (formData.custom_header ?? []).map((h, i) => (i === idx ? { ...h, ...patch } : h));
        onFormDataChange({ ...formData, custom_header: next });
    };

    const handleRemoveHeader = (idx: number) => {
        const curr = formData.custom_header ?? [];
        if (curr.length <= 1) return;
        onFormDataChange({ ...formData, custom_header: curr.filter((_, i) => i !== idx) });
    };

    const handleApplyTemplate = (templateKey: string) => {
        const template = channelTemplates.find((item) => item.key === templateKey);
        if (!template) return;
        onFormDataChange(template.apply(formData));
        setTestSummary(null);
    };

    return (
        <form onSubmit={onSubmit} className="flex h-full min-h-0 flex-col">
            <div className={cn(
                'min-h-0 flex-1 overflow-y-auto overscroll-contain',
                isCreateLayout
                    ? 'grid content-start gap-5 px-4 py-5 sm:px-6 md:grid-cols-2 md:gap-x-6 [&_input]:h-11 [&_[data-slot=select-trigger]]:h-11 [&_[data-slot=select-trigger]]:min-w-0'
                    : 'space-y-4 pb-2',
            )}>
            {showTemplatePicker ? (
                <section className={sectionClassName}>
                    <SectionHeader icon={Sparkles} title={t('template.label')} hint={t('template.hint')} />
                    {isMobile ? (
                        <TemplatePickerSelect onApplyTemplate={handleApplyTemplate} />
                    ) : (
                        <TemplatePickerGrid onApplyTemplate={handleApplyTemplate} />
                    )}
                </section>
            ) : onShowTemplatePicker ? (
                <div className="flex justify-end">
                    <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={onShowTemplatePicker}
                        className="h-8 rounded-lg text-xs transition-[color,background-color,border-color,box-shadow,opacity,transform] duration-100 ease-out active:scale-[0.98]"
                    >
                        <Sparkles className="size-3.5" />
                        {t('template.open')}
                    </Button>
                </div>
            ) : null}

            <section className={cn(sectionClassName, isCreateLayout && 'md:col-span-2')}>
                <SectionHeader icon={Orbit} title={t('basicInfo')} />
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                    <div className={fieldGroupClassName}>
                        <label htmlFor={`${idPrefix}-name`} className={labelClassName}>
                        {t('name')}
                        </label>
                        <Input
                            className="rounded-lg"
                            id={`${idPrefix}-name`}
                            type="text"
                            value={formData.name}
                            onChange={(event) => onFormDataChange({ ...formData, name: event.target.value })}
                            required
                        />
                    </div>

                    <div className={fieldGroupClassName}>
                        <label htmlFor={`${idPrefix}-type`} className={labelClassName}>
                        {t('type')}
                        </label>
                        <Select
                            value={String(formData.type)}
                            onValueChange={(value) => onFormDataChange({ ...formData, type: Number(value) as ChannelType })}
                        >
                            <SelectTrigger id={`${idPrefix}-type`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent className="rounded-lg">
                                {CHANNEL_TYPE_OPTIONS.map(option => (
                                    <SelectItem key={option.value} className="rounded-xl" value={String(option.value)}>{t(option.labelKey)}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    <div className={fieldGroupClassName}>
                        <label htmlFor={`${idPrefix}-group`} className={labelClassName}>
                            {t('group')}
                        </label>
                        <Select
                            value={String(formData.group_id || 0)}
                            onValueChange={(value) => onFormDataChange({ ...formData, group_id: Number(value) })}
                        >
                            <SelectTrigger id={`${idPrefix}-group`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                <SelectValue placeholder={t('groupLoading')} />
                            </SelectTrigger>
                            <SelectContent className="rounded-lg">
                                {channelGroups.length > 0 ? (
                                    channelGroups.map((group) => (
                                        <SelectItem key={group.id} className="rounded-xl" value={String(group.id)}>
                                            {group.name}
                                        </SelectItem>
                                    ))
                                ) : (
                                    <SelectItem className="rounded-xl" value="0">
                                        {t('groupLoading')}
                                    </SelectItem>
                                )}
                            </SelectContent>
                        </Select>
                    </div>
                </div>
            </section>

            <section className={cn(sectionClassName, isCreateLayout && 'md:col-span-2')}>
                <SectionHeader icon={Cable} title={t('baseUrlConfig')} hint={t('baseUrlHint')} />
                <div className="flex items-center justify-end gap-2">
                    <Badge variant="secondary" className="rounded-full">
                        {formData.base_urls.length}
                    </Badge>
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={handleAddBaseUrl}
                        className="h-10 rounded-lg px-3 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                    >
                        <Plus className="h-3 w-3 mr-1" />
                        {t('add')}
                    </Button>
                </div>
                <div className="space-y-2">
                    {(formData.base_urls ?? []).map((u, idx) => (
                        <div key={`baseurl-${idx}`} className="space-y-1.5 rounded-lg border border-border/25 bg-card p-2">
                            <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
                                <Input
                                    id={`${idPrefix}-base-${idx}`}
                                    type="url"
                                    value={u.url}
                                    onChange={(e) => handleUpdateBaseUrl(idx, { url: e.target.value })}
                                    placeholder={t('baseUrlUrl')}
                                    aria-label={`${t('baseUrlUrl')} ${idx + 1}`}
                                    required={idx === 0}
                                    className="w-full flex-1 rounded-lg"
                                />
                                <div className="flex items-center gap-2 sm:shrink-0">
                                    <Select
                                        value={isOpenAICompatBaseUrlSuffixMode(u.suffix_mode) ? 'openai_compat' : 'custom'}
                                        onValueChange={(value) => handleUpdateBaseUrl(idx, { suffix_mode: value as Channel['base_urls'][number]['suffix_mode'] })}
                                    >
                                        <SelectTrigger className="h-10 min-w-0 flex-1 rounded-lg sm:w-36 lg:w-44 sm:flex-none">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value="openai_compat">{t('baseUrlSuffixOpenAI')}</SelectItem>
                                            <SelectItem className="rounded-xl" value="custom">{t('baseUrlSuffixCustom')}</SelectItem>
                                        </SelectContent>
                                    </Select>
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        onClick={() => handleRemoveBaseUrl(idx)}
                                        disabled={(formData.base_urls ?? []).length <= 1}
                                        className="size-11 shrink-0 rounded-lg p-0 text-muted-foreground hover:bg-muted hover:text-destructive disabled:opacity-40"
                                        title={t('remove')}
                                    >
                                        <X className="h-4 w-4" />
                                    </Button>
                                </div>
                            </div>
                            {isOpenAICompatBaseUrlSuffixMode(u.suffix_mode) && hasManualVersionSuffix(u.url) ? (
                                <p className="px-1 text-xs leading-5 text-destructive">{t('baseUrlOpenAIWarning')}</p>
                            ) : (
                                <Hint
                                    className="sm:shrink-0"
                                    text={
                                        isOpenAICompatBaseUrlSuffixMode(u.suffix_mode)
                                            ? t('baseUrlOpenAIHint')
                                            : t('baseUrlCustomHint')
                                    }
                                />
                            )}
                        </div>
                    ))}
                </div>
            </section>


            {formData.pool_id === 0 && (
            <section className={cn(sectionClassName, isCreateLayout && 'md:col-span-2')}>
                <SectionHeader icon={KeyRound} title={t('apiKeyConfig')} />
                <div className="flex items-center justify-end gap-2">
                    <Badge variant="secondary" className="rounded-full">
                        {formData.keys.length}
                    </Badge>
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={handleTestChannel}
                        disabled={testChannel.isPending || !(formData.base_urls?.some((u) => u.url.trim()) && formData.keys?.some((k) => k.channel_key.trim()))}
                        className="h-10 rounded-lg px-3 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                    >
                        {testChannel.isPending ? (
                            <RefreshCw className="h-3 w-3 mr-1 animate-spin" />
                        ) : (
                            <FlaskConical className="h-3 w-3 mr-1" />
                        )}
                        {t('test.button')}
                    </Button>
                    <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={handleAddKey}
                        className="h-10 rounded-lg px-3 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                    >
                        <Plus className="h-3 w-3 mr-1" />
                        {t('add')}
                    </Button>
                </div>
                <div className="space-y-2">
                    {(formData.keys ?? []).map((k, idx) => (
                        <div key={k.id ?? `new-${idx}`} className="rounded-lg border border-border/25 bg-card p-2 space-y-2">
                        <div className={cn(
                            "grid grid-cols-[minmax(0,1fr)_auto] gap-2 lg:items-center",
                            showPriorityInput ? "lg:grid-cols-[minmax(0,1fr)_6rem_8rem_6.5rem_2.75rem]" : "lg:grid-cols-[minmax(0,1fr)_8rem_6.5rem_2.75rem]"
                        )}>
                            <Input
                                type="text"
                                value={k.channel_key}
                                onChange={(e) => handleUpdateKey(idx, { channel_key: e.target.value })}
                                placeholder={t('apiKey')}
                                aria-label={`${t('apiKey')} ${idx + 1}`}
                                required={idx === 0}
                                className="col-span-2 rounded-lg lg:col-span-1"
                            />
                            {showPriorityInput && (
                                <Hint text={t('priorityHint')} side="top">
                                    <Input
                                        type="number"
                                        value={k.priority ?? 0}
                                        onChange={(e) => handleUpdateKey(idx, { priority: Number(e.target.value || 0) })}
                                        placeholder={t('priority')}
                                        className="rounded-lg"
                                    />
                                </Hint>
                            )}
                            <Input
                                type="text"
                                value={k.remark ?? ''}
                                onChange={(e) => handleUpdateKey(idx, { remark: e.target.value })}
                                placeholder={t('remark')}
                                className="col-span-2 min-w-0 rounded-lg lg:col-span-1"
                            />
                            <label className="flex items-center gap-2 rounded-lg border border-border/20 bg-card px-3 py-2 text-sm text-card-foreground">
                                <Switch
                                    checked={k.enabled}
                                    onCheckedChange={(checked) => handleUpdateKey(idx, { enabled: checked })}
                                />
                                <span>{t('enabled')}</span>
                            </label>
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={() => handleRemoveKey(idx)}
                                disabled={(formData.keys ?? []).length <= 1}
                                className="size-11 shrink-0 rounded-lg p-0 text-muted-foreground hover:bg-muted hover:text-destructive disabled:opacity-40"
                                title={t('remove')}
                            >
                                <X className="h-4 w-4" />
                            </Button>
                        </div>
                        <div className="flex items-center gap-2">
                            <div className="min-w-0 flex-1">
                                <Hint text={t('supportedModelsHint')} side="top">
                                    <Input
                                        type="text"
                                        value={k.supported_models ?? ''}
                                        onChange={(e) => handleUpdateKey(idx, { supported_models: e.target.value })}
                                        placeholder={t('supportedModels')}
                                        className="rounded-lg text-xs text-muted-foreground"
                                    />
                                </Hint>
                            </div>
                            {k.auto_disabled_at ? (
                                <div className="flex shrink-0 items-center gap-1.5">
                                    <Hint text={k.auto_disable_reason || t('autoDisabledHint')} side="top">
                                        <Badge variant="destructive" className="whitespace-nowrap rounded-full text-[10px] font-normal">
                                            {t('autoDisabled')}
                                        </Badge>
                                    </Hint>
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="sm"
                                        className="h-6 px-2 text-xs"
                                        onClick={() => handleUpdateKey(idx, { enabled: true, auto_disabled_at: null, auto_disable_reason: '' })}
                                    >
                                        {t('autoDisabledEnable')}
                                    </Button>
                                </div>
                            ) : null}
                        </div>
                        </div>
                    ))}
                </div>

                {testSummary && (
                    <div className="space-y-2 rounded-lg border border-border/25 bg-card p-3">
                        <div className="flex items-center justify-between gap-2">
                            <div className="flex items-center gap-2 text-sm font-medium text-card-foreground">
                                {testSummary.passed ? (
                                    <CheckCircle2 className="h-4 w-4 text-green-500" />
                                ) : (
                                    <AlertTriangle className="h-4 w-4 text-orange-500" />
                                )}
                                <span>
                                    {testSummary.passed ? t('test.success') : t('test.partialSuccess')}
                                    <Hint text={t('test.hint')} />
                                </span>
                            </div>
                            <div className="flex items-center gap-2">
                                {testSummary.results.some((r) => !r.passed) && (
                                    <Button
                                        type="button"
                                        variant="ghost"
                                        size="sm"
                                        onClick={handleRemoveFailedKeys}
                                        className="h-6 px-2 text-xs text-destructive/70 hover:text-destructive hover:bg-transparent"
                                    >
                                        <Trash2 className="h-3 w-3 mr-1" />
                                        {t('test.removeFailedKeys')}
                                    </Button>
                                )}
                                <Badge variant="secondary">{testSummary.results.length} {t('test.results')}</Badge>
                            </div>
                        </div>
                        <div className="space-y-2 max-h-48 overflow-y-auto">
                            {testSummary.results.map((result, idx) => (
                                <div key={`${result.base_url}-${result.key_masked}-${idx}`} className="rounded-lg border border-border/30 bg-card p-2.5 text-xs space-y-1">
                                    <div className="flex flex-wrap items-center justify-between gap-2">
                                        <span className="font-mono truncate">{result.base_url}</span>
                                        <div className="flex items-center gap-1 shrink-0">
                                            <Badge variant="secondary">{result.key_masked || '-'}</Badge>
                                            <Badge variant="secondary">{result.status_code}</Badge>
                                        </div>
                                    </div>
                                    <div className="flex items-center justify-between gap-2 text-muted-foreground">
                                        <span>{result.key_remark || t('test.noRemark')}</span>
                                        <span>{result.latency_ms}ms · {result.passed ? t('test.pass') : t('test.fail')}</span>
                                    </div>
                                    {result.message && <p className="break-all text-muted-foreground">{result.message}</p>}
                                </div>
                            ))}
                        </div>
                    </div>
                )}
            </section>
            )}

            <section className={sectionClassName}>
                <SectionHeader icon={Layers3} title={t('modelConfig')} />
                <div className="flex items-center justify-end gap-2">
                    <MorphingDialog onOpen={handleRefreshModels}>
                        <MorphingDialogTrigger
                            disabled={!formData.base_urls?.[0]?.url || !effectiveKey || fetchModel.isPending}
                            className="inline-flex h-6 items-center justify-center gap-1.5 rounded-md px-2 text-xs font-medium text-muted-foreground/50 transition-colors hover:bg-transparent hover:text-muted-foreground"
                        >
                            <RefreshCw className={`h-3 w-3 ${fetchModel.isPending ? 'animate-spin' : ''}`} />
                            {t('modelRefresh')}
                        </MorphingDialogTrigger>
                        <MorphingDialogContainer>
                            <MorphingDialogContent className="h-[calc(100dvh-2rem)] w-[min(100vw-2rem,54rem)] max-w-full rounded-xl border border-border/35 bg-card p-0 md:h-[min(44rem,calc(100dvh-3rem))]">
                                <ModelPickerDialogPanel
                                    models={fetchedModels}
                                    draftSelected={modelPickerDraft}
                                    onDraftChange={setModelPickerDraft}
                                    isLoading={fetchModel.isPending}
                                    onApply={applyFetchedModelSelection}
                                    perKeyItems={perKeyItems}
                                    perKeyLoading={fetchModelsPerKey.isPending}
                                    onApplyPerKey={applyPerKeySelection}
                                />
                            </MorphingDialogContent>
                        </MorphingDialogContainer>
                    </MorphingDialog>
                </div>
                <input type="hidden" value={formData.model} required />

                <div className="relative">
                    <Input
                        ref={inputRef}
                        id={`${idPrefix}-model-custom`}
                        type="text"
                        value={inputValue}
                        onChange={(e) => setInputValue(e.target.value)}
                        onKeyDown={handleInputKeyDown}
                        placeholder={t('modelCustomPlaceholder')}
                        className="rounded-lg pr-10"
                    />
                    {inputValue.trim() && !customModels.includes(inputValue.trim()) && !autoModels.includes(inputValue.trim()) && (
                        <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={() => handleAddModel(inputValue)}
                            className="absolute rounded-lg right-1 top-1/2 -translate-y-1/2 h-7 w-7 p-0 text-muted-foreground hover:bg-accent hover:text-accent-foreground transition-colors"
                            title={t('modelAdd')}
                        >
                            <Plus className="size-4" />
                        </Button>
                    )}
                </div>

                <div className="space-y-2">
                    <div className="flex items-center justify-between">
                        <label className="text-xs font-medium text-card-foreground">
                            {t('modelSelected')} {(autoModels.length + customModels.length) > 0 && `(${autoModels.length + customModels.length})`}
                        </label>
                        {(autoModels.length + customModels.length) > 0 && (
                            <Button
                                type="button"
                                variant="ghost"
                                size="sm"
                                onClick={() => {
                                    updateModels([], []);
                                }}
                                className="h-6 px-2 text-xs text-muted-foreground/50 hover:text-muted-foreground hover:bg-transparent"
                            >
                                {t('modelClearAll')}
                            </Button>
                        )}
                    </div>
                    <div className="max-h-40 min-h-12 overflow-y-auto rounded-lg border border-border/25 bg-card p-2.5 shadow-sm">
                        {(autoModels.length + customModels.length) > 0 ? (
                            <div className="flex flex-wrap gap-1.5">
                                {autoModels.map((model) => (
                                    <Badge key={model} variant="secondary" className="max-w-full gap-1 bg-muted hover:bg-muted/80">
                                        <span className="min-w-0 break-all whitespace-normal">{model}</span>
                                        <button
                                            type="button"
                                            onClick={() => handleRemoveAutoModel(model)}
                                            className="ml-1 rounded-sm opacity-70 hover:opacity-100 focus:outline-none focus:ring-1 focus:ring-ring"
                                        >
                                            <X className="h-3 w-3" />
                                        </button>
                                    </Badge>
                                ))}
                                {customModels.map((model) => (
                                    <Badge key={model} className="max-w-full gap-1 bg-primary hover:bg-primary/90">
                                        <span className="min-w-0 break-all whitespace-normal">{model}</span>
                                        <button
                                            type="button"
                                            onClick={() => handleRemoveCustomModel(model)}
                                            className="ml-1 rounded-sm opacity-70 hover:opacity-100 focus:outline-none focus:ring-1 focus:ring-ring"
                                        >
                                            <X className="h-3 w-3" />
                                        </button>
                                    </Badge>
                                ))}
                            </div>
                        ) : (
                            <div className="flex items-center justify-center h-8 text-xs text-muted-foreground">
                                {t('modelNoSelected')}
                            </div>
                        )}
                    </div>
                </div>
            </section>

            <Accordion type="single" collapsible className={cn('w-full min-w-0', isCreateLayout && 'md:col-span-2')}>
                <AccordionItem value="advanced" className="border-none">
                    <AccordionTrigger className="rounded-lg bg-card/70 px-4 py-4 text-sm font-medium text-card-foreground transition-colors hover:bg-card hover:no-underline">
                        <span className="flex items-center gap-2">
                            <span className="h-2 w-2 rounded-full bg-primary/70" />
                            {t('advanced')}
                        </span>
                    </AccordionTrigger>
                    <AccordionContent className="pt-4">
                        <div className="space-y-4">
                        <div className="grid min-w-0 items-start gap-4 md:grid-cols-2">
                            <ProxySelector
                                layout="stacked"
                                value={{ proxy_mode: formData.proxy_mode, proxy_config_id: formData.proxy_config_id }}
                                onChange={(next) => onFormDataChange({
                                    ...formData,
                                    proxy_mode: next.proxy_mode as ChannelProxyMode,
                                    proxy_config_id: next.proxy_config_id ?? null,
                                })}
                            />
                            <div className={fieldGroupClassName}>
                                <label htmlFor={`${idPrefix}-key-strategy`} className={labelClassName}>{t('keySelectionStrategy')}</label>
                                <Select
                                    value={formData.key_selection_strategy || '__inherit__'}
                                    onValueChange={(value) => onFormDataChange({ ...formData, key_selection_strategy: value === '__inherit__' ? '' : value })}
                                >
                                    <SelectTrigger id={`${idPrefix}-key-strategy`} className="h-11 w-full rounded-lg">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent className="rounded-lg">
                                        <SelectItem className="rounded-xl" value="__inherit__">{t('keySelectionStrategyInherit')}</SelectItem>
                                        <SelectItem className="rounded-xl" value="cost">{t('keySelectionStrategyCost')}</SelectItem>
                                        <SelectItem className="rounded-xl" value="availability">{t('keySelectionStrategyAvailability')}</SelectItem>
                                        <SelectItem className="rounded-xl" value="speed">{t('keySelectionStrategySpeed')}</SelectItem>
                                        <SelectItem className="rounded-xl" value="priority">{t('keySelectionStrategyPriority')}</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                        </div>
                        <div className={fieldGroupClassName}>
                            <label htmlFor={`${idPrefix}-pool`} className={labelClassName}>
                                {t('poolBinding')}
                                {formData.pool_id > 0 ? <Hint text={t('poolHint')} /> : null}
                            </label>
                            <Select
                                value={String(formData.pool_id || 0)}
                                onValueChange={(value) => onFormDataChange({ ...formData, pool_id: Number(value) })}
                            >
                                <SelectTrigger id={`${idPrefix}-pool`} className="h-11 w-full rounded-lg">
                                    <SelectValue placeholder={t('poolPlaceholder')} />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="0">{t('poolNone')}</SelectItem>
                                    {pools.map((pool) => (
                                        <SelectItem key={pool.id} value={String(pool.id)}>{pool.name}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div className={fieldGroupClassName}>
                                <label htmlFor={`${idPrefix}-auto-group`} className={labelClassName}>
                                    {t('autoGroup')}
                                </label>
                                <Select
                                    value={String(formData.auto_group)}
                                    onValueChange={(value) => onFormDataChange({ ...formData, auto_group: Number(value) as AutoGroupType })}
                                >
                                    <SelectTrigger id={`${idPrefix}-auto-group`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent className="rounded-lg">
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.None)}>{t('autoGroupNone')}</SelectItem>
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.Fuzzy)}>{t('autoGroupFuzzy')}</SelectItem>
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.Exact)}>{t('autoGroupExact')}</SelectItem>
                                        <SelectItem className="rounded-xl" value={String(AutoGroupType.Regex)}>{t('autoGroupRegex')}</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>

                            <div className={fieldGroupClassName}>
                                <label htmlFor={`${idPrefix}-channel-proxy`} className={labelClassName}>
                                    {t('channelProxy')}
                                </label>
                                <Input
                                    id={`${idPrefix}-channel-proxy`}
                                    type="text"
                                    value={formData.channel_proxy}
                                    onChange={(e) => onFormDataChange({ ...formData, channel_proxy: e.target.value })}
                                    placeholder={t('channelProxyPlaceholder')}
                                    className="rounded-lg"
                                />
                            </div>
                        </div>

                        <div className={fieldGroupClassName}>
                            <div className="flex items-center justify-between">
                                <label className={labelClassName}>
                                    {t('customHeader')} {formData.custom_header.length > 0 ? `(${formData.custom_header.length})` : ''}
                                </label>
                                <Button
                                    type="button"
                                    variant="ghost"
                                    size="sm"
                                    onClick={handleAddHeader}
                                    className="h-10 rounded-lg px-3 text-xs text-muted-foreground hover:bg-muted hover:text-foreground"
                                >
                                    <Plus className="h-3 w-3 mr-1" />
                                    {t('customHeaderAdd')}
                                </Button>
                            </div>
                            <div className="space-y-2">
                                {(formData.custom_header ?? []).map((h, idx) => (
                                    <div key={`hdr-${idx}`} className="grid gap-2 rounded-lg border border-border/25 bg-card p-2 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] lg:items-center">
                                        <Input
                                            type="text"
                                            value={h.header_key}
                                            onChange={(e) => handleUpdateHeader(idx, { header_key: e.target.value })}
                                            placeholder={t('customHeaderKey')}
                                            className="rounded-lg"
                                        />
                                        <Input
                                            type="text"
                                            value={h.header_value}
                                            onChange={(e) => handleUpdateHeader(idx, { header_value: e.target.value })}
                                            placeholder={t('customHeaderValue')}
                                            className="rounded-lg"
                                        />
                                        <Button
                                            type="button"
                                            variant="ghost"
                                            size="sm"
                                            onClick={() => handleRemoveHeader(idx)}
                                            disabled={(formData.custom_header ?? []).length <= 1}
                                            className="size-11 shrink-0 rounded-lg p-0 text-muted-foreground hover:bg-muted hover:text-destructive disabled:opacity-40"
                                            title={t('remove')}
                                        >
                                            <X className="h-4 w-4" />
                                        </Button>
                                    </div>
                                ))}
                            </div>
                        </div>

                        <div className={fieldGroupClassName}>
                            <label htmlFor={`${idPrefix}-match-regex`} className={labelClassName}>
                                {t('matchRegex')}
                            </label>
                            <Input
                                id={`${idPrefix}-match-regex`}
                                type="text"
                                value={formData.match_regex}
                                onChange={(e) => onFormDataChange({ ...formData, match_regex: e.target.value })}
                                placeholder={t('matchRegexPlaceholder')}
                                className="rounded-lg"
                            />
                        </div>

                        <div className={fieldGroupClassName}>
                            <div className="flex items-center justify-between gap-3">
                                <label htmlFor={`${idPrefix}-param-override`} className={labelClassName}>
                                    {t('paramOverride')}
                                </label>
                                <label className="flex items-center gap-2 cursor-pointer">
                                    <Switch
                                        checked={formData.param_override_force}
                                        onCheckedChange={(checked) => onFormDataChange({ ...formData, param_override_force: checked })}
                                    />
                                    <span className="text-sm text-card-foreground whitespace-nowrap">{t('paramOverrideForce')}</span>
                                    <Hint text={t('paramOverrideForceHint')} />
                                </label>
                            </div>
                            <textarea
                                id={`${idPrefix}-param-override`}
                                value={formData.param_override}
                                onChange={(e) => onFormDataChange({ ...formData, param_override: e.target.value })}
                                placeholder={t('paramOverridePlaceholder')}
                                className="min-h-28 w-full rounded-lg border border-border/35 bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                            />
                        </div>

                        <div className="space-y-4 pt-2 border-t border-border/20">
                            <div className="flex flex-wrap items-center justify-between gap-3">
                                <div>
                                    <p className="text-sm font-medium text-card-foreground">
                                        {t('requestRewrite')}
                                        <Hint text={t('requestRewriteHint')} />
                                    </p>
                                </div>
                                <label className="flex items-center gap-2 cursor-pointer">
                                    <Switch
                                        checked={requestRewriteSupported && formData.request_rewrite.enabled}
                                        onCheckedChange={(checked) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                enabled: checked,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported}
                                    />
                                    <span className="text-sm text-card-foreground">{t('requestRewriteEnabled')}</span>
                                </label>
                            </div>

                            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                                <div className={fieldGroupClassName}>
                                    <label htmlFor={`${idPrefix}-request-rewrite-profile`} className={labelClassName}>
                                        {t('requestRewriteProfile')}
                                    </label>
                                    <Select
                                        value={formData.request_rewrite.profile ?? RequestRewriteProfile.OpenAIChatCompat}
                                        onValueChange={(value) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                profile: value as RequestRewriteProfile,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported || !formData.request_rewrite.enabled}
                                    >
                                        <SelectTrigger id={`${idPrefix}-request-rewrite-profile`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value={RequestRewriteProfile.Preserve}>保留</SelectItem>
                                            <SelectItem className="rounded-xl" value={RequestRewriteProfile.OpenAIChatCompat}>{t('requestRewriteProfileOpenAIChatCompat')}</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className={fieldGroupClassName}>
                                    <label htmlFor={`${idPrefix}-request-rewrite-tool-role`} className={labelClassName}>
                                        {t('requestRewriteToolRoleStrategy')}
                                    </label>
                                    <Select
                                        value={formData.request_rewrite.tool_role_strategy ?? ToolRoleStrategy.Keep}
                                        onValueChange={(value) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                tool_role_strategy: value as ToolRoleStrategy,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported || !formData.request_rewrite.enabled}
                                    >
                                        <SelectTrigger id={`${idPrefix}-request-rewrite-tool-role`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value={ToolRoleStrategy.Keep}>{t('requestRewriteStrategyKeep')}</SelectItem>
                                            <SelectItem className="rounded-xl" value={ToolRoleStrategy.StringifyToUser}>{t('requestRewriteStrategyStringifyToUser')}</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className={fieldGroupClassName}>
                                    <label htmlFor={`${idPrefix}-request-rewrite-header-profile`} className={labelClassName}>
                                        Headers 请求重写
                                    </label>
                                    <Select
                                        value={formData.request_rewrite.header_profile || '__none__'}
                                        onValueChange={(value) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                header_profile: value === '__none__' ? RequestRewriteHeaderProfile.None : value as RequestRewriteHeaderProfile,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported || !formData.request_rewrite.enabled}
                                    >
                                        <SelectTrigger id={`${idPrefix}-request-rewrite-header-profile`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value="__none__">无</SelectItem>
                                            <SelectItem className="rounded-xl" value={RequestRewriteHeaderProfile.Codex}>Codex</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>

                                <div className={fieldGroupClassName}>
                                    <label htmlFor={`${idPrefix}-request-rewrite-system`} className={labelClassName}>
                                        {t('requestRewriteSystemMessageStrategy')}
                                    </label>
                                    <Select
                                        value={formData.request_rewrite.system_message_strategy ?? SystemMessageStrategy.Keep}
                                        onValueChange={(value) => onFormDataChange({
                                            ...formData,
                                            request_rewrite: {
                                                ...formData.request_rewrite,
                                                system_message_strategy: value as SystemMessageStrategy,
                                            },
                                        })}
                                        disabled={!requestRewriteSupported || !formData.request_rewrite.enabled}
                                    >
                                        <SelectTrigger id={`${idPrefix}-request-rewrite-system`} className="w-full rounded-lg border border-border px-4 py-2 text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value={SystemMessageStrategy.Keep}>{t('requestRewriteStrategyKeep')}</SelectItem>
                                            <SelectItem className="rounded-xl" value={SystemMessageStrategy.Merge}>{t('requestRewriteStrategyMerge')}</SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>
                            </div>
                        </div>
                        </div>
                    </AccordionContent>
                </AccordionItem>
            </Accordion>
            <section className={cn(sectionClassName, 'flex flex-col gap-4', isCreateLayout && 'md:col-span-2')}>
                <label className="flex items-center gap-2 cursor-pointer">
                    <Switch
                        checked={formData.enabled}
                        onCheckedChange={(checked) => onFormDataChange({ ...formData, enabled: checked })}
                    />
                    <span className="text-sm font-medium text-card-foreground">{t('enabled')}</span>
                </label>
                <div className="grid grid-cols-1 items-start gap-4 border-t border-border/10 pt-4 sm:grid-cols-2 lg:grid-cols-3">
                    <label className="flex items-center gap-2 cursor-pointer">
                        <Switch
                            checked={formData.auto_sync}
                            onCheckedChange={(checked) => onFormDataChange({ ...formData, auto_sync: checked })}
                        />
                        <span className="text-sm text-card-foreground">{t('autoSync')}</span>
                    </label>
                    {/* 仅在自动同步开启时有意义：参照 request_rewrite 子项的 disabled 依赖处理方式。
                        Hint 放在 label 外侧，避免在 label 内嵌可交互控件导致点击图标误触发开关 */}
                    {/* 号池渠道（pool_id !== 0）不渲染该开关：其 relay 走 poolscheduler，按
                        PoolAccount.Models 过滤，完全不消费 ChannelKey.SupportedModels，开关打开也无效。
                        门禁写法与下方 key 编辑区的 formData.pool_id === 0 保持一致。 */}
                    {formData.pool_id === 0 && (
                    <div className="flex items-center gap-1">
                        <label className={cn('flex items-center gap-2', formData.auto_sync ? 'cursor-pointer' : 'cursor-not-allowed')}>
                            <Switch
                                checked={formData.auto_sync_key_models}
                                disabled={!formData.auto_sync || formData.pool_id !== 0}
                                onCheckedChange={(checked) => onFormDataChange({ ...formData, auto_sync_key_models: checked })}
                            />
                            <span className={cn('text-sm text-card-foreground', !formData.auto_sync && 'text-muted-foreground/60')}>
                                {t('autoSyncKeyModels')}
                            </span>
                        </label>
                        <Hint text={t('autoSyncKeyModelsHint')} />
                    </div>
                    )}
                    <label className="flex items-center gap-2 cursor-pointer">
                        <Switch
                            checked={formData.skip_model_test}
                            onCheckedChange={(checked) => onFormDataChange({ ...formData, skip_model_test: checked })}
                        />
                        <span className="text-sm text-card-foreground">{t('skipModelTest')}</span>
                    </label>
                    <label className="flex items-center gap-2 cursor-pointer">
                        <Switch
                            checked={formData.disposable}
                            onCheckedChange={(checked) => onFormDataChange({ ...formData, disposable: checked })}
                        />
                        <span className="text-sm text-card-foreground">{t('disposable')}</span>
                    </label>
                    {formData.disposable && (
                        <div className="col-span-full grid min-w-0 gap-4 sm:grid-cols-2">
                            <div className="min-w-0 space-y-2">
                                <span className="text-sm text-card-foreground">{t('expireAt')}</span>
                                <Input
                                    type="datetime-local"
                                    className="h-11 w-full min-w-0 rounded-lg"
                                    value={formData.expire_at || ''}
                                    onChange={(e) => onFormDataChange({ ...formData, expire_at: e.target.value })}
                                />
                            </div>
                            {notifChannels.length > 0 && (
                                <div className="flex items-center gap-2">
                                    <span className="text-sm text-card-foreground whitespace-nowrap">{t('notifChannel')}</span>
                                    <Select
                                        value={formData.notif_channel_id != null ? String(formData.notif_channel_id) : '__none__'}
                                        onValueChange={(value) => onFormDataChange({ ...formData, notif_channel_id: value === '__none__' ? null : Number(value) })}
                                    >
                                        <SelectTrigger className="h-8 w-36 rounded-lg">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent className="rounded-lg">
                                            <SelectItem className="rounded-xl" value="__none__">{t('notifChannelNone')}</SelectItem>
                                            {notifChannels.map((nc) => (
                                                <SelectItem key={nc.id} className="rounded-xl" value={String(nc.id)}>
                                                    {nc.name}
                                                </SelectItem>
                                            ))}
                                        </SelectContent>
                                    </Select>
                                </div>
                            )}
                        </div>
                    )}
                </div>
            </section>
            </div>

            <div className={cn(
                'flex shrink-0 gap-3',
                isCreateLayout
                    ? 'justify-end border-t border-border bg-card px-4 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] sm:px-6'
                    : `flex-col pt-4 ${onCancel ? 'sm:flex-row' : ''}`,
            )}>
                {onCancel && cancelText && (
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={onCancel}
                        disabled={isPending}
                        className={cn('h-11 rounded-lg', isCreateLayout ? 'flex-1 sm:w-auto sm:flex-none sm:min-w-28' : 'w-full sm:flex-1')}
                    >
                        {cancelText}
                    </Button>
                )}
                <Button
                    type="submit"
                    disabled={isPending}
                    className={cn('h-11 rounded-lg', isCreateLayout ? 'flex-1 sm:w-auto sm:flex-none sm:min-w-28' : 'w-full sm:flex-1')}
                >
                    {isPending ? pendingText : submitText}
                </Button>
            </div>
        </form>
    );
}
