'use client';

import { Check, Loader, Trash2, X } from 'lucide-react';
import { motion } from 'motion/react';
import { useTranslations } from 'next-intl';
import { Input } from '@/components/ui/input';

type EditValues = {
    input: string;
    output: string;
    cache_read: string;
    cache_write: string;
};

type ModelDeleteOverlayProps = {
    layoutId: string;
    isPending: boolean;
    onCancel: () => void;
    onConfirm: () => void;
};

export function ModelDeleteOverlay({
    layoutId,
    isPending,
    onCancel,
    onConfirm,
}: ModelDeleteOverlayProps) {
    const t = useTranslations('model.overlay');
    return (
        <motion.div
            layoutId={layoutId}
            className="absolute inset-0 flex items-center justify-center gap-3 rounded-lg border border-destructive/20 bg-destructive p-4"
            transition={{ type: 'spring', stiffness: 400, damping: 30 }}
        >
            <button
                type="button"
                onClick={onCancel}
                className="flex h-10 items-center justify-center gap-1.5 rounded-lg border border-white/15 bg-destructive-foreground/16 px-4 text-sm font-medium text-destructive-foreground transition-all hover:bg-destructive-foreground/24 active:scale-[0.98]"
            >
                <X className="size-4" />
                {t('cancel')}
            </button>
            <button
                type="button"
                onClick={onConfirm}
                disabled={isPending}
                className="flex h-10 items-center justify-center gap-1.5 rounded-lg bg-destructive-foreground px-4 text-sm font-medium text-destructive transition-all hover:bg-destructive-foreground/92 active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-50"
            >
                {isPending ? (
                    <Loader className="size-4 animate-spin" />
                ) : (
                    <Trash2 className="size-4" />
                )}
                {isPending ? t('deleting') : t('confirmDelete')}
            </button>
        </motion.div>
    );
}

type ModelEditOverlayProps = {
    layoutId: string;
    modelName: string;
    brandColor: string;
    editValues: EditValues;
    isPending: boolean;
    onChange: (next: EditValues) => void;
    onCancel: () => void;
    onSave: () => void;
};

export function ModelEditOverlay({
    layoutId,
    modelName,
    brandColor,
    editValues,
    isPending,
    onChange,
    onCancel,
    onSave,
}: ModelEditOverlayProps) {
    const t = useTranslations('model.overlay');
    return (
        <motion.div
            layoutId={layoutId}
            className="absolute inset-x-0 top-0 z-20 flex flex-col overflow-hidden rounded-xl border border-border/35 bg-card p-5 text-card-foreground"
            transition={{ type: 'spring', stiffness: 400, damping: 30 }}
        >
            <div className="relative">
                <div className="mb-3 inline-flex items-center rounded-full border border-primary/12 bg-card px-3 py-1 text-[0.68rem] font-semibold text-primary">
                    {t('save')}
                </div>
                <h3 className="mb-4 line-clamp-1 text-base font-semibold text-card-foreground">
                    {modelName}
                </h3>

                <div className="grid grid-cols-2 gap-2">
                    <label className="grid gap-1.5 text-xs text-muted-foreground">
                        {t('input')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.input}
                            onChange={(e) => onChange({ ...editValues, input: e.target.value })}
                            className="h-10 rounded-lg border-border/25 bg-card text-sm"
                        />
                    </label>
                    <label className="grid gap-1.5 text-xs text-muted-foreground">
                        {t('output')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.output}
                            onChange={(e) => onChange({ ...editValues, output: e.target.value })}
                            className="h-10 rounded-lg border-border/25 bg-card text-sm"
                        />
                    </label>
                    <label className="grid gap-1.5 text-xs text-muted-foreground">
                        {t('cacheRead')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.cache_read}
                            onChange={(e) => onChange({ ...editValues, cache_read: e.target.value })}
                            className="h-10 rounded-lg border-border/25 bg-card text-sm"
                        />
                    </label>
                    <label className="grid gap-1.5 text-xs text-muted-foreground">
                        {t('cacheWrite')}
                        <Input
                            type="number"
                            step="any"
                            value={editValues.cache_write}
                            onChange={(e) => onChange({ ...editValues, cache_write: e.target.value })}
                            className="h-10 rounded-lg border-border/25 bg-card text-sm"
                        />
                    </label>
                </div>

                <div className="mt-4 flex gap-2">
                    <button
                        type="button"
                        onClick={onCancel}
                        disabled={isPending}
                        className="flex h-10 flex-1 items-center justify-center gap-1.5 rounded-lg border border-border/25 bg-card text-sm font-medium text-muted-foreground transition-all hover:bg-card active:scale-[0.98] disabled:opacity-50"
                    >
                        <X className="size-4" />
                        {t('cancel')}
                    </button>
                    <button
                        type="button"
                        onClick={onSave}
                        disabled={isPending}
                        className="flex h-10 flex-1 items-center justify-center gap-1.5 rounded-lg text-sm font-medium transition-all active:scale-[0.98] disabled:opacity-50"
                        style={{ backgroundColor: brandColor, color: '#fff' }}
                    >
                        {isPending ? <Loader className="size-4 animate-spin" /> : <Check className="size-4" />}
                        {t('save')}
                    </button>
                </div>
            </div>
        </motion.div>
    );
}
