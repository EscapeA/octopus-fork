'use client';

import { useMemo, useState } from 'react';
import { Archive, Bell, Check, Inbox, Pencil, RefreshCw, Save, Search, Trash2, X } from 'lucide-react';
import { useTranslations } from 'next-intl';
import { PageWrapper } from '@/components/common/PageWrapper';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import {
    useArchiveNotification,
    useDeleteNotification,
    useMarkAllNotificationsRead,
    useMarkNotificationRead,
    useMarkNotificationUnread,
    useNotificationDetail,
    useNotificationPolicies,
    useNotificationPreferences,
    useSaveNotificationPreference,
    useCreateNotificationPolicy,
    useUpdateNotificationPolicy,
    useDeleteNotificationPolicy,
    useNotifications,
    useUnreadNotificationCount,
    type NotificationFilter,
    type NotificationItem,
    type NotificationPolicy,
    type NotificationPreference,
} from '@/api/endpoints/notification';

const TYPES = ['', 'alert', 'report', 'channel_expire', 'system', 'site', 'backup', 'usage'];
const SEVERITIES = ['', 'info', 'success', 'warning', 'error', 'critical'];

function formatTime(ts?: number) {
    if (!ts) return '-';
    return new Date(ts).toLocaleString();
}

function severityClass(severity: string) {
    switch (severity) {
        case 'success': return 'bg-emerald-500/10 text-emerald-600 border-emerald-500/30';
        case 'warning': return 'bg-amber-500/10 text-amber-600 border-amber-500/30';
        case 'error':
        case 'critical': return 'bg-destructive/10 text-destructive border-destructive/30';
        default: return 'bg-muted text-muted-foreground';
    }
}

function NotificationCard({ item, selected, onSelect }: { item: NotificationItem; selected: boolean; onSelect: () => void }) {
    const t = useTranslations('notification');
    return (
        <button onClick={onSelect} className={`w-full rounded-2xl border p-4 text-left transition hover:bg-muted/60 ${selected ? 'border-primary bg-primary/5' : 'border-border bg-card'}`}>
            <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                        {!item.read_at && <span className="h-2 w-2 rounded-full bg-primary" />}
                        <Badge variant="outline">{t(`type.${item.type}`)}</Badge>
                        <Badge variant="outline" className={severityClass(item.severity)}>{t(`severity.${item.severity}`)}</Badge>
                    </div>
                    <h3 className="mt-2 truncate text-base font-semibold">{item.title}</h3>
                    <p className="mt-1 line-clamp-2 text-sm text-muted-foreground">{item.content}</p>
                </div>
                <div className="shrink-0 text-xs text-muted-foreground">{formatTime(item.created_at)}</div>
            </div>
        </button>
    );
}

export function Notification() {
    const t = useTranslations('notification');
    const [tab, setTab] = useState<'inbox' | 'archived' | 'policies' | 'preferences'>('inbox');
    const [type, setType] = useState('');
    const [severity, setSeverity] = useState('');
    const [search, setSearch] = useState('');
    const [selectedID, setSelectedID] = useState<number | undefined>();
    const [policyDraft, setPolicyDraft] = useState<Partial<NotificationPolicy>>({ name: '', enabled: true, type: '', min_severity: 'info', source: '', channel_ids: '[]' });
    const [editingPolicyID, setEditingPolicyID] = useState<number | null>(null);
    const [preferenceDraft, setPreferenceDraft] = useState<Partial<NotificationPreference>>({ user_id: 0, type: 'alert', enabled: true, in_app_enabled: true, external_enabled: true, min_severity: 'info', channel_ids: '[]' });

    const filter: NotificationFilter = useMemo(() => ({
        page: 1,
        page_size: 50,
        archived: tab === 'archived',
        type: type || undefined,
        severity: severity || undefined,
        search: search || undefined,
    }), [search, severity, tab, type]);

    const { data: items = [], isLoading, refetch } = useNotifications(filter);
    const { data: unread } = useUnreadNotificationCount();
    const detail = useNotificationDetail(selectedID);
    const markRead = useMarkNotificationRead();
    const markUnread = useMarkNotificationUnread();
    const archive = useArchiveNotification();
    const deleteNotification = useDeleteNotification();
    const markAllRead = useMarkAllNotificationsRead();
    const { data: policies = [] } = useNotificationPolicies();
    const { data: preferences = [] } = useNotificationPreferences();
    const createPolicy = useCreateNotificationPolicy();
    const updatePolicy = useUpdateNotificationPolicy();
    const deletePolicy = useDeleteNotificationPolicy();
    const savePreference = useSaveNotificationPreference();

    const selected = detail.data?.notification;

    const resetPolicyDraft = () => {
        setEditingPolicyID(null);
        setPolicyDraft({ name: '', enabled: true, type: '', min_severity: 'info', source: '', channel_ids: '[]' });
    };
    const savePolicyDraft = () => {
        const payload = {
            id: editingPolicyID ?? 0,
            name: policyDraft.name || '',
            enabled: policyDraft.enabled ?? true,
            type: policyDraft.type || '',
            min_severity: policyDraft.min_severity || 'info',
            source: policyDraft.source || '',
            channel_ids: policyDraft.channel_ids || '[]',
        } as NotificationPolicy;
        if (editingPolicyID) updatePolicy.mutate(payload, { onSuccess: resetPolicyDraft });
        else createPolicy.mutate(payload, { onSuccess: resetPolicyDraft });
    };
    const editPolicy = (policy: NotificationPolicy) => {
        setEditingPolicyID(policy.id);
        setPolicyDraft(policy);
    };
    const savePreferenceDraft = () => {
        savePreference.mutate({
            id: preferenceDraft.id ?? 0,
            user_id: preferenceDraft.user_id ?? 0,
            type: preferenceDraft.type || 'alert',
            enabled: preferenceDraft.enabled ?? true,
            in_app_enabled: preferenceDraft.in_app_enabled ?? true,
            external_enabled: preferenceDraft.external_enabled ?? true,
            min_severity: preferenceDraft.min_severity || 'info',
            channel_ids: preferenceDraft.channel_ids || '[]',
            quiet_start: preferenceDraft.quiet_start || '',
            quiet_end: preferenceDraft.quiet_end || '',
        });
    };

    return (
        <PageWrapper>
            <div className="space-y-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                    <div>
                        <h1 className="text-2xl font-bold tracking-tight">{t('title')}</h1>
                        <p className="text-sm text-muted-foreground">{t('subtitle', { count: unread?.count ?? 0 })}</p>
                    </div>
                    <div className="flex gap-2">
                        <Button variant="outline" onClick={() => refetch()}><RefreshCw className="h-4 w-4" />{t('actions.refresh')}</Button>
                        <Button onClick={() => markAllRead.mutate()}><Check className="h-4 w-4" />{t('actions.markAllRead')}</Button>
                    </div>
                </div>

                <div className="flex flex-wrap gap-2">
                    <Button variant={tab === 'inbox' ? 'default' : 'outline'} onClick={() => setTab('inbox')}><Inbox className="h-4 w-4" />{t('tabs.inbox')}</Button>
                    <Button variant={tab === 'archived' ? 'default' : 'outline'} onClick={() => setTab('archived')}><Archive className="h-4 w-4" />{t('tabs.archived')}</Button>
                    <Button variant={tab === 'policies' ? 'default' : 'outline'} onClick={() => setTab('policies')}><Bell className="h-4 w-4" />{t('tabs.policies')}</Button>
                    <Button variant={tab === 'preferences' ? 'default' : 'outline'} onClick={() => setTab('preferences')}><Check className="h-4 w-4" />{t('tabs.preferences')}</Button>
                </div>

                {(tab === 'inbox' || tab === 'archived') ? (
                    <>
                        <div className="grid gap-3 rounded-2xl border bg-card p-3 md:grid-cols-[1fr_auto_auto]">
                            <div className="relative">
                                <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                                <Input value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t('filters.search')} className="pl-9" />
                            </div>
                            <select value={type} onChange={(e) => setType(e.target.value)} className="rounded-xl border bg-background px-3 py-2 text-sm">
                                {TYPES.map((v) => <option key={v} value={v}>{v ? t(`type.${v}`) : t('filters.allTypes')}</option>)}
                            </select>
                            <select value={severity} onChange={(e) => setSeverity(e.target.value)} className="rounded-xl border bg-background px-3 py-2 text-sm">
                                {SEVERITIES.map((v) => <option key={v} value={v}>{v ? t(`severity.${v}`) : t('filters.allSeverities')}</option>)}
                            </select>
                        </div>

                        <div className="grid min-h-[32rem] gap-4 lg:grid-cols-[minmax(0,1fr)_24rem]">
                            <div className="space-y-3">
                                {isLoading ? <div className="rounded-2xl border bg-card p-8 text-center text-muted-foreground">{t('loading')}</div> : null}
                                {!isLoading && items.length === 0 ? <div className="rounded-2xl border bg-card p-8 text-center text-muted-foreground">{t('empty')}</div> : null}
                                {items.map((item) => <NotificationCard key={item.id} item={item} selected={selectedID === item.id} onSelect={() => setSelectedID(item.id)} />)}
                            </div>

                            <aside className="rounded-2xl border bg-card p-4">
                                {!selected ? (
                                    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">{t('detail.placeholder')}</div>
                                ) : (
                                    <div className="space-y-4">
                                        <div>
                                            <div className="flex gap-2"><Badge>{t(`type.${selected.type}`)}</Badge><Badge variant="outline" className={severityClass(selected.severity)}>{t(`severity.${selected.severity}`)}</Badge></div>
                                            <h2 className="mt-3 text-lg font-semibold">{selected.title}</h2>
                                            <p className="mt-1 text-xs text-muted-foreground">{formatTime(selected.created_at)}</p>
                                        </div>
                                        <p className="whitespace-pre-wrap text-sm text-muted-foreground">{selected.content}</p>
                                        <div className="flex flex-wrap gap-2">
                                            {selected.read_at ? <Button size="sm" variant="outline" onClick={() => markUnread.mutate([selected.id])}>{t('actions.markUnread')}</Button> : <Button size="sm" onClick={() => markRead.mutate([selected.id])}>{t('actions.markRead')}</Button>}
                                            {!selected.archived_at && <Button size="sm" variant="outline" onClick={() => archive.mutate([selected.id])}>{t('actions.archive')}</Button>}
                                            <Button size="sm" variant="destructive" onClick={() => deleteNotification.mutate(selected.id)}><Trash2 className="h-4 w-4" />{t('actions.delete')}</Button>
                                        </div>
                                        <div className="border-t pt-3">
                                            <h3 className="text-sm font-semibold">{t('detail.deliveries')}</h3>
                                            <div className="mt-2 space-y-2">
                                                {(detail.data?.deliveries ?? []).length === 0 ? <p className="text-xs text-muted-foreground">{t('detail.noDeliveries')}</p> : null}
                                                {(detail.data?.deliveries ?? []).map((d) => <div key={d.id} className="rounded-xl bg-muted p-2 text-xs"><b>{d.channel_name || d.channel_id}</b> · {d.status}{d.last_error ? ` · ${d.last_error}` : ''}</div>)}
                                            </div>
                                        </div>
                                    </div>
                                )}
                            </aside>
                        </div>
                    </>
                ) : tab === 'policies' ? (
                    <div className="space-y-4 rounded-2xl border bg-card p-4">
                        <div>
                            <h2 className="text-lg font-semibold">{t('policies.title')}</h2>
                            <p className="text-sm text-muted-foreground">{t('policies.description')}</p>
                        </div>
                        <div className="grid gap-3 rounded-xl bg-muted/40 p-3 md:grid-cols-2">
                            <Input placeholder={t('policies.form.name')} value={policyDraft.name || ''} onChange={(e) => setPolicyDraft({ ...policyDraft, name: e.target.value })} />
                            <Input placeholder={t('policies.form.channelIds')} value={policyDraft.channel_ids || ''} onChange={(e) => setPolicyDraft({ ...policyDraft, channel_ids: e.target.value })} />
                            <select value={policyDraft.type || ''} onChange={(e) => setPolicyDraft({ ...policyDraft, type: e.target.value as NotificationPolicy['type'] })} className="rounded-xl border bg-background px-3 py-2 text-sm">
                                {TYPES.map((v) => <option key={v} value={v}>{v ? t(`type.${v}`) : t('filters.allTypes')}</option>)}
                            </select>
                            <select value={policyDraft.min_severity || 'info'} onChange={(e) => setPolicyDraft({ ...policyDraft, min_severity: e.target.value as NotificationPolicy['min_severity'] })} className="rounded-xl border bg-background px-3 py-2 text-sm">
                                {SEVERITIES.filter(Boolean).map((v) => <option key={v} value={v}>{t(`severity.${v}`)}</option>)}
                            </select>
                            <Input placeholder={t('policies.form.source')} value={policyDraft.source || ''} onChange={(e) => setPolicyDraft({ ...policyDraft, source: e.target.value })} />
                            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={policyDraft.enabled ?? true} onChange={(e) => setPolicyDraft({ ...policyDraft, enabled: e.target.checked })} />{t('policies.form.enabled')}</label>
                            <div className="flex gap-2 md:col-span-2">
                                <Button onClick={savePolicyDraft} disabled={!policyDraft.name}><Save className="h-4 w-4" />{editingPolicyID ? t('actions.save') : t('actions.create')}</Button>
                                {editingPolicyID && <Button variant="outline" onClick={resetPolicyDraft}><X className="h-4 w-4" />{t('actions.cancel')}</Button>}
                            </div>
                        </div>
                        <div className="space-y-3">
                            {policies.length === 0 ? <div className="rounded-xl bg-muted p-4 text-sm text-muted-foreground">{t('policies.empty')}</div> : null}
                            {policies.map((p) => <div key={p.id} className="flex items-center justify-between gap-3 rounded-xl border p-3"><div><div className="font-medium">{p.name}</div><div className="text-xs text-muted-foreground">{p.type || t('filters.allTypes')} · {p.min_severity} · {p.channel_ids}</div></div><div className="flex gap-2"><Button size="sm" variant="outline" onClick={() => editPolicy(p)}><Pencil className="h-4 w-4" />{t('actions.edit')}</Button><Button size="sm" variant="destructive" onClick={() => deletePolicy.mutate(p.id)}><Trash2 className="h-4 w-4" /></Button></div></div>)}
                        </div>
                    </div>
                ) : (
                    <div className="space-y-4 rounded-2xl border bg-card p-4">
                        <div>
                            <h2 className="text-lg font-semibold">{t('preferences.title')}</h2>
                            <p className="text-sm text-muted-foreground">{t('preferences.description')}</p>
                        </div>
                        <div className="grid gap-3 rounded-xl bg-muted/40 p-3 md:grid-cols-2">
                            <select value={preferenceDraft.type || 'alert'} onChange={(e) => setPreferenceDraft({ ...preferenceDraft, type: e.target.value as NotificationPreference['type'] })} className="rounded-xl border bg-background px-3 py-2 text-sm">
                                {TYPES.filter(Boolean).map((v) => <option key={v} value={v}>{t(`type.${v}`)}</option>)}
                            </select>
                            <select value={preferenceDraft.min_severity || 'info'} onChange={(e) => setPreferenceDraft({ ...preferenceDraft, min_severity: e.target.value as NotificationPreference['min_severity'] })} className="rounded-xl border bg-background px-3 py-2 text-sm">
                                {SEVERITIES.filter(Boolean).map((v) => <option key={v} value={v}>{t(`severity.${v}`)}</option>)}
                            </select>
                            <Input placeholder={t('preferences.form.channelIds')} value={preferenceDraft.channel_ids || ''} onChange={(e) => setPreferenceDraft({ ...preferenceDraft, channel_ids: e.target.value })} />
                            <div className="grid grid-cols-2 gap-2"><Input placeholder="HH:mm" value={preferenceDraft.quiet_start || ''} onChange={(e) => setPreferenceDraft({ ...preferenceDraft, quiet_start: e.target.value })} /><Input placeholder="HH:mm" value={preferenceDraft.quiet_end || ''} onChange={(e) => setPreferenceDraft({ ...preferenceDraft, quiet_end: e.target.value })} /></div>
                            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={preferenceDraft.in_app_enabled ?? true} onChange={(e) => setPreferenceDraft({ ...preferenceDraft, in_app_enabled: e.target.checked })} />{t('preferences.form.inApp')}</label>
                            <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={preferenceDraft.external_enabled ?? true} onChange={(e) => setPreferenceDraft({ ...preferenceDraft, external_enabled: e.target.checked })} />{t('preferences.form.external')}</label>
                            <Button onClick={savePreferenceDraft} className="md:col-span-2"><Save className="h-4 w-4" />{t('actions.save')}</Button>
                        </div>
                        <div className="space-y-3">
                            {preferences.length === 0 ? <div className="rounded-xl bg-muted p-4 text-sm text-muted-foreground">{t('preferences.empty')}</div> : null}
                            {preferences.map((p) => <button key={p.id} onClick={() => setPreferenceDraft(p)} className="w-full rounded-xl border p-3 text-left hover:bg-muted"><div className="font-medium">{t(`type.${p.type}`)} · {t(`severity.${p.min_severity}`)}</div><div className="text-xs text-muted-foreground">{p.in_app_enabled ? t('preferences.form.inApp') : ''} {p.external_enabled ? t('preferences.form.external') : ''} · {p.channel_ids}</div></button>)}
                        </div>
                    </div>
                )}
            </div>
        </PageWrapper>
    );
}
