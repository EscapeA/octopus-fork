'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { Share2, Download, X } from 'lucide-react';
import { toPng } from 'html-to-image';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { toast } from '@/components/common/Toast';
import { useTranslations } from 'next-intl';

export interface SnapshotMetric {
  id: string;
  label: string;
  value: string;
  change?: string;
  trend?: 'up' | 'down' | 'neutral';
}

export interface SnapshotListRow {
  label: string;
  value: string;
  meta?: string;
}

export interface SnapshotMetricSection {
  id: string;
  label: string;
  type: 'metrics';
  defaultSelected: boolean;
  items: SnapshotMetric[];
}

export interface SnapshotListSection {
  id: string;
  label: string;
  type: 'list';
  defaultSelected: boolean;
  rows: SnapshotListRow[];
}

export type SnapshotSection = SnapshotMetricSection | SnapshotListSection;

interface ShareSnapshotData {
  title: string;
  subtitle?: string;
  timestamp: string;
  sections: SnapshotSection[];
}

interface ShareSnapshotProps {
  data: ShareSnapshotData;
}

interface Choice {
  id: string;
  label: string;
  value: string;
  sectionId: string;
}

interface SectionGroup {
  section: SnapshotSection;
  choices: Choice[];
}

const STORAGE_KEY = 'octopus:snapshot:selected-v1';

function loadSelection(defaults: Set<string>): Set<string> {
  if (typeof window === 'undefined') return new Set(defaults);
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return new Set(defaults);
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return new Set(defaults);
    return new Set(parsed.filter((x): x is string => typeof x === 'string'));
  } catch {
    return new Set(defaults);
  }
}

export function ShareSnapshot({ data }: ShareSnapshotProps) {
  const t = useTranslations('analytics.share');
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const snapshotRef = useRef<HTMLDivElement>(null);

  const { choices, choicesBySection, defaultSelected } = useMemo<{ choices: Choice[]; choicesBySection: SectionGroup[]; defaultSelected: Set<string> }>(() => {
    const all: Choice[] = [];
    const bySection: SectionGroup[] = [];
    const defaults = new Set<string>();
    for (const section of data.sections) {
      const sectionChoices: Choice[] = [];
      if (section.type === 'metrics') {
        for (const item of section.items) {
          const choice: Choice = { id: item.id, label: item.label, value: item.value, sectionId: section.id };
          sectionChoices.push(choice);
          all.push(choice);
          if (section.defaultSelected) defaults.add(item.id);
        }
      } else {
        const choice: Choice = {
          id: section.id,
          label: section.label,
          value: t('items', { count: section.rows.length }),
          sectionId: section.id,
        };
        sectionChoices.push(choice);
        all.push(choice);
        if (section.defaultSelected) defaults.add(section.id);
      }
      bySection.push({ section, choices: sectionChoices });
    }
    return { choices: all, choicesBySection: bySection, defaultSelected: defaults };
  }, [data.sections, t]);

  const [selected, setSelected] = useState<Set<string>>(() => loadSelection(defaultSelected));

  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify([...selected]));
    } catch {
      // ignore quota / private mode errors
    }
  }, [selected]);

  const toggle = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const toggleSection = (sectionChoices: Choice[]) => {
    const allOn = sectionChoices.every((c) => selected.has(c.id));
    setSelected((prev) => {
      const next = new Set(prev);
      if (allOn) sectionChoices.forEach((c) => next.delete(c.id));
      else sectionChoices.forEach((c) => next.add(c.id));
      return next;
    });
  };

  const selectedMetrics = useMemo(() => {
    const result: SnapshotMetric[] = [];
    for (const section of data.sections) {
      if (section.type !== 'metrics') continue;
      for (const item of section.items) {
        if (selected.has(item.id)) result.push(item);
      }
    }
    return result;
  }, [data.sections, selected]);

  const selectedLists = useMemo(
    () => data.sections.filter((s): s is SnapshotListSection => s.type === 'list' && selected.has(s.id)),
    [data.sections, selected],
  );

  const handleExport = async () => {
    if (!snapshotRef.current) return;
    setLoading(true);
    try {
      const dataUrl = await toPng(snapshotRef.current, {
        backgroundColor: '#ffffff',
        pixelRatio: 2,
        quality: 0.95,
      });
      const link = document.createElement('a');
      link.download = `analytics-${data.title.toLowerCase().replace(/\s+/g, '-')}-${Date.now()}.png`;
      link.href = dataUrl;
      link.click();
      toast.success(t('exportSuccess'));
      setOpen(false);
    } catch (error) {
      console.error('Failed to export snapshot:', error);
      toast.error(t('exportError'));
    } finally {
      setLoading(false);
    }
  };

  const handleCopyToClipboard = async () => {
    if (!snapshotRef.current) return;
    setLoading(true);
    try {
      const dataUrl = await toPng(snapshotRef.current, {
        backgroundColor: '#ffffff',
        pixelRatio: 2,
        quality: 0.95,
      });
      const response = await fetch(dataUrl);
      const blob = await response.blob();
      await navigator.clipboard.write([
        new ClipboardItem({ 'image/png': blob }),
      ]);
      toast.success(t('copySuccess'));
    } catch (error) {
      console.error('Failed to copy to clipboard:', error);
      toast.error(t('copyError'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        onClick={() => setOpen(true)}
        className="gap-2"
      >
        <Share2 className="h-4 w-4" />
        {t('button')}
      </Button>

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent aria-describedby={undefined} className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Share2 className="h-5 w-5" />
              {t('title')}
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-4">
            {/* Snapshot Preview — captured element */}
            <div className="border rounded-lg overflow-hidden">
              <div
                ref={snapshotRef}
                className="p-6 bg-gradient-to-br from-primary/5 to-primary/10 space-y-6"
              >
                {/* Header */}
                <div className="space-y-1">
                  <h2 className="text-2xl font-bold">{data.title}</h2>
                  {data.subtitle && (
                    <p className="text-sm text-muted-foreground">{data.subtitle}</p>
                  )}
                  <p className="text-xs text-muted-foreground">{data.timestamp}</p>
                </div>

                {/* Stats Grid */}
                {selectedMetrics.length > 0 && (
                  <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                    {selectedMetrics.map((stat) => (
                      <div
                        key={stat.id}
                        className="bg-white dark:bg-gray-900 rounded-lg p-4 shadow-sm border border-border"
                      >
                        <div className="text-xs text-muted-foreground mb-1">
                          {stat.label}
                        </div>
                        <div className="text-2xl font-bold">{stat.value}</div>
                        {stat.change && (
                          <div
                            className={`text-xs mt-1 flex items-center gap-1 ${
                              stat.trend === 'up'
                                ? 'text-green-600'
                                : stat.trend === 'down'
                                ? 'text-red-600'
                                : 'text-muted-foreground'
                            }`}
                          >
                            {stat.change}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                )}

                {/* List Sections */}
                {selectedLists.map((section) => (
                  <div key={section.id} className="space-y-2">
                    <div className="text-sm font-semibold border-b pb-1">{section.label}</div>
                    <div className="space-y-1.5">
                      {section.rows.map((row, i) => (
                        <div key={i} className="flex items-center justify-between gap-3 text-xs">
                          <span className="truncate text-muted-foreground">{row.label}</span>
                          <span className="flex items-baseline gap-1.5 shrink-0">
                            <span className="font-medium tabular-nums">{row.value}</span>
                            {row.meta && <span className="text-muted-foreground">{row.meta}</span>}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}

                {selectedMetrics.length === 0 && selectedLists.length === 0 && (
                  <div className="text-center text-sm text-muted-foreground py-8">
                    {t('emptySelection')}
                  </div>
                )}

                {/* Footer */}
                <div className="text-xs text-muted-foreground text-center pt-4 border-t">
                  Generated by Octopus Analytics
                </div>
              </div>
            </div>

            {/* Selection Panel — not captured */}
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">{t('selectData')}</span>
                <div className="flex gap-1">
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-7 px-2 text-xs"
                    onClick={() => setSelected(new Set(choices.map((c) => c.id)))}
                  >
                    {t('selectAll')}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-7 px-2 text-xs"
                    onClick={() => setSelected(new Set())}
                  >
                    {t('clearAll')}
                  </Button>
                </div>
              </div>

              <div className="max-h-72 overflow-y-auto space-y-4 pr-1">
                {choicesBySection.map(({ section, choices: sectionChoices }) => {
                  const allOn = sectionChoices.every((c) => selected.has(c.id));
                  return (
                    <div key={section.id} className="space-y-2">
                      <div className="flex items-center justify-between">
                        <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                          {section.label}
                        </span>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-6 px-2 text-xs"
                          onClick={() => toggleSection(sectionChoices)}
                        >
                          {allOn ? t('clearSection') : t('selectSection')}
                        </Button>
                      </div>
                      <div className="grid gap-1.5">
                        {sectionChoices.map((choice) => (
                          <div
                            key={choice.id}
                            className="flex items-center justify-between gap-3 rounded-lg border border-border/30 bg-card px-3 py-2"
                          >
                            <div className="min-w-0">
                              <div className="text-sm font-medium truncate">{choice.label}</div>
                              <div className="text-xs text-muted-foreground truncate">{choice.value}</div>
                            </div>
                            <Switch
                              checked={selected.has(choice.id)}
                              onCheckedChange={() => toggle(choice.id)}
                            />
                          </div>
                        ))}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>

            {/* Actions */}
            <div className="flex gap-2">
              <Button
                onClick={handleExport}
                disabled={loading || (selectedMetrics.length === 0 && selectedLists.length === 0)}
                className="flex-1 gap-2"
              >
                <Download className="h-4 w-4" />
                {t('download')}
              </Button>
              <Button
                onClick={handleCopyToClipboard}
                disabled={loading || (selectedMetrics.length === 0 && selectedLists.length === 0)}
                variant="outline"
                className="flex-1 gap-2"
              >
                <Share2 className="h-4 w-4" />
                {t('copy')}
              </Button>
              <Button
                onClick={() => setOpen(false)}
                variant="ghost"
                disabled={loading}
                size="icon"
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
