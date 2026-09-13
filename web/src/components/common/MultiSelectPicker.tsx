'use client';

import { useMemo, useState } from 'react';
import { Check, ListFilter, Search } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
import { cn } from '@/lib/utils';

export interface MultiSelectOption {
    /** 稳定标识（写入设置 API 时使用，通常是字符串化的 ID） */
    value: string;
    /** 主标签 */
    label: string;
    /** 次要说明（如 #13），可省略 */
    description?: string;
}

export interface MultiSelectPickerLabels {
    /** 打开弹窗的按钮文案 */
    dialogTitle: string;
    /** 弹窗副标题（可选） */
    dialogDescription?: string;
    searchPlaceholder: string;
    /** 含 {count} 占位符 */
    selectedCount: string;
    /** 含 {count} 占位符 */
    filteredCount: string;
    clear: string;
    selectFiltered: string;
    unselectFiltered: string;
    /** 含 {count} 占位符 */
    apply: string;
    cancel: string;
    /** 无可选项时的空态 */
    empty: string;
    /** 尚未选择任何项时 chip 区的占位文案 */
    emptySelection: string;
    /** 搜索无结果 */
    noResult: string;
    /** 已选项 chip 的移除提示 */
    removeHint: string;
}

export interface MultiSelectPickerProps {
    options: MultiSelectOption[];
    /** 当前已应用的值（首屏空态显示「未选择 = 不限制」由调用方通过 labels 说明） */
    selected: string[];
    /** 应用后的回调（弹窗点「应用」与 chip 删除都会触发） */
    onApply: (next: string[]) => void;
    labels: MultiSelectPickerLabels;
    /** 弹窗标题旁的图标 */
    icon?: React.ReactNode;
    /** 触发器额外文案，例如「选择分组」 */
    triggerLabel: string;
    disabled?: boolean;
    className?: string;
}

function formatTemplate(template: string, count: number): string {
    return template.replaceAll('{count}', String(count));
}

function MultiSelectPickerPanel({
    options,
    draftSelected,
    onDraftChange,
    labels,
    onApply,
}: {
    options: MultiSelectOption[];
    draftSelected: string[];
    onDraftChange: (next: string[]) => void;
    labels: MultiSelectPickerLabels;
    onApply: () => void;
}) {
    const { setIsOpen } = useMorphingDialog();
    const [searchTerm, setSearchTerm] = useState('');
    const normalizedSearch = searchTerm.trim().toLowerCase();
    const isSearching = normalizedSearch.length > 0;

    const filteredOptions = useMemo(() => {
        if (!isSearching) return options;
        return options.filter((option) =>
            option.label.toLowerCase().includes(normalizedSearch)
            || option.value.toLowerCase().includes(normalizedSearch)
            || (option.description ?? '').toLowerCase().includes(normalizedSearch)
        );
    }, [options, normalizedSearch, isSearching]);

    const selectedSet = useMemo(() => new Set(draftSelected), [draftSelected]);
    const allFilteredSelected = filteredOptions.length > 0 && filteredOptions.every((option) => selectedSet.has(option.value));

    const toggleOption = (value: string) => {
        onDraftChange(
            draftSelected.includes(value)
                ? draftSelected.filter((item) => item !== value)
                : [...draftSelected, value]
        );
    };

    const toggleFiltered = () => {
        if (filteredOptions.length === 0) return;
        const filteredValues = filteredOptions.map((option) => option.value);
        if (allFilteredSelected) {
            const removeSet = new Set(filteredValues);
            onDraftChange(draftSelected.filter((value) => !removeSet.has(value)));
        } else {
            onDraftChange(Array.from(new Set([...draftSelected, ...filteredValues])));
        }
    };

    const handleApply = () => {
        onApply();
        setIsOpen(false);
    };

    return (
        <div className="relative flex h-full min-h-0 flex-col overflow-hidden rounded-xl border border-border/35 bg-card text-card-foreground shadow-md">
            <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_16%_14%,color-mix(in_oklch,var(--primary)_18%,transparent)_0%,transparent_30%),linear-gradient(180deg,color-mix(in_oklch,white_18%,transparent),transparent_28%)]" />
            <MorphingDialogTitle className="shrink-0">
                <header className="relative flex items-center justify-between gap-4 border-b border-border/20 px-5 py-4 md:px-6">
                    <div className="min-w-0 space-y-1">
                        <h3 className="truncate text-lg font-semibold tracking-tight text-card-foreground md:text-xl">
                            {labels.dialogTitle}
                        </h3>
                        {labels.dialogDescription ? (
                            <p className="text-xs text-muted-foreground">{labels.dialogDescription}</p>
                        ) : null}
                    </div>
                    <MorphingDialogClose className="relative right-0 top-0" />
                </header>
            </MorphingDialogTitle>

            <MorphingDialogDescription disableLayoutAnimation className="relative flex min-h-0 flex-1 flex-col gap-4 px-4 py-4 md:px-6">
                <div className="relative shrink-0">
                    <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
                    <Input
                        value={searchTerm}
                        onChange={(event) => setSearchTerm(event.target.value)}
                        placeholder={labels.searchPlaceholder}
                        className="h-11 rounded-lg pl-9"
                    />
                </div>

                <div className="flex shrink-0 flex-wrap items-center justify-between gap-2">
                    <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                        <Badge variant="secondary" className="rounded-full">
                            {formatTemplate(labels.selectedCount, draftSelected.length)}
                        </Badge>
                        <span>{formatTemplate(labels.filteredCount, filteredOptions.length)}</span>
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
                            {labels.clear}
                        </Button>
                        <Button
                            type="button"
                            variant="secondary"
                            size="sm"
                            onClick={toggleFiltered}
                            disabled={filteredOptions.length === 0}
                            className="h-8 rounded-lg px-2 text-xs"
                        >
                            <ListFilter className="size-3.5" />
                            {allFilteredSelected ? labels.unselectFiltered : labels.selectFiltered}
                        </Button>
                    </div>
                </div>

                <div className="min-h-0 flex-1 overflow-y-auto rounded-lg border border-border/25 bg-card p-2 shadow-sm">
                    {options.length === 0 ? (
                        <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
                            {labels.empty}
                        </div>
                    ) : filteredOptions.length > 0 ? (
                        <div className="grid gap-2 sm:grid-cols-2">
                            {filteredOptions.map((option) => {
                                const selected = selectedSet.has(option.value);
                                return (
                                    <button
                                        key={option.value}
                                        type="button"
                                        onClick={() => toggleOption(option.value)}
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
                                        <span className="min-w-0 flex-1">
                                            <span className="block truncate" title={option.label}>{option.label}</span>
                                            {option.description ? (
                                                <span className="block truncate text-xs text-muted-foreground">{option.description}</span>
                                            ) : null}
                                        </span>
                                    </button>
                                );
                            })}
                        </div>
                    ) : (
                        <div className="flex h-40 items-center justify-center text-sm text-muted-foreground">
                            {labels.noResult}
                        </div>
                    )}
                </div>

                <div className="flex shrink-0 flex-col gap-2 border-t border-border/20 pt-4 sm:flex-row">
                    <Button
                        type="button"
                        variant="secondary"
                        onClick={() => setIsOpen(false)}
                        className="h-11 rounded-lg sm:flex-1"
                    >
                        {labels.cancel}
                    </Button>
                    <Button
                        type="button"
                        onClick={handleApply}
                        className="h-11 rounded-lg sm:flex-1"
                    >
                        {formatTemplate(labels.apply, draftSelected.length)}
                    </Button>
                </div>
            </MorphingDialogDescription>
        </div>
    );
}

/**
 * 「已选 chip + 弹窗勾选」多选控件，视觉与交互对齐渠道编辑页的模型选择弹窗
 * （搜索 / 已选与匹配计数 / 全选匹配项 / 清空 / 应用）。
 */
export function MultiSelectPicker({
    options,
    selected,
    onApply,
    labels,
    icon,
    triggerLabel,
    disabled,
    className,
}: MultiSelectPickerProps) {
    const [draftSelected, setDraftSelected] = useState<string[]>(selected);

    // 选项列表变化（如新增/删除 Key）后，丢弃已不存在的选中项，避免脏值写回设置。
    const availableValues = useMemo(() => new Set(options.map((option) => option.value)), [options]);
    const appliedValues = useMemo(
        () => selected.filter((value) => availableValues.size === 0 || availableValues.has(value)),
        [selected, availableValues]
    );

    const selectedOptions = useMemo(() => {
        const selectedSet = new Set(appliedValues);
        return options.filter((option) => selectedSet.has(option.value));
    }, [options, appliedValues]);

    return (
        <div className={cn('flex flex-col gap-3', className)}>
            <div className="flex flex-wrap items-center gap-2">
                {selectedOptions.length > 0 ? (
                    selectedOptions.map((option) => (
                        <Badge
                            key={option.value}
                            variant="default"
                            className="max-w-full gap-1.5 rounded-lg px-2.5 py-1 text-xs"
                        >
                            <span className="truncate">{option.label}</span>
                            <button
                                type="button"
                                onClick={() => onApply(appliedValues.filter((value) => value !== option.value))}
                                disabled={disabled}
                                title={labels.removeHint}
                                className="text-muted-foreground transition-colors hover:text-foreground disabled:opacity-50"
                            >
                                ×
                            </button>
                        </Badge>
                    ))
                ) : (
                    <span className="text-xs text-muted-foreground">{labels.emptySelection}</span>
                )}

                <MorphingDialog onOpen={() => setDraftSelected(appliedValues)}>
                    <MorphingDialogTrigger
                        disabled={disabled || options.length === 0}
                        className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-border/35 px-3 text-xs font-medium text-muted-foreground transition-colors hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
                    >
                        {icon ?? <ListFilter className="size-3.5" />}
                        {triggerLabel}
                    </MorphingDialogTrigger>
                    <MorphingDialogContainer>
                        <MorphingDialogContent className="h-[calc(100dvh-2rem)] w-[min(100vw-2rem,42rem)] max-w-full rounded-xl border border-border/35 bg-card p-0 md:h-[min(38rem,calc(100dvh-3rem))]">
                            <MultiSelectPickerPanel
                                options={options}
                                draftSelected={draftSelected}
                                onDraftChange={setDraftSelected}
                                labels={labels}
                                onApply={() => onApply(draftSelected)}
                            />
                        </MorphingDialogContent>
                    </MorphingDialogContainer>
                </MorphingDialog>
            </div>
        </div>
    );
}
