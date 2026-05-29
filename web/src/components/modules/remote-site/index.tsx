'use client';

import { useState } from 'react';
import {
    useRemoteSiteList,
    useDeleteRemoteSite,
    useRefreshRemoteSite,
    useRefreshAllRemoteSites,
    type RemoteSite as RemoteSiteModel,
} from '@/api/endpoints/remote-site';
import { LoadingState } from '@/components/common/LoadingState';
import { ErrorState } from '@/components/common/ErrorState';
import { Globe, RefreshCw, Plus, Trash2, ExternalLink, CircleDot, Pencil, KeyRound } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useTranslations } from 'next-intl';
import { SiteDialog } from './SiteDialog';
import { TokenManager } from './TokenManager';

function healthColor(status: string): string {
    switch (status) {
        case 'healthy': return 'text-green-500';
        case 'warning': return 'text-yellow-500';
        case 'error': return 'text-red-500';
        default: return 'text-muted-foreground';
    }
}

function SiteCard({ site, onEdit, onDelete, onRefresh, onOpenTokens }: {
    site: RemoteSiteModel;
    onEdit: (site: RemoteSiteModel) => void;
    onDelete: (id: number) => void;
    onRefresh: (id: number) => void;
    onOpenTokens: (id: number) => void;
}) {
    const t = useTranslations('hub');

    return (
        <div className="rounded-lg border bg-card text-card-foreground shadow-sm p-4 flex flex-col gap-3">
            <div className="flex items-start justify-between">
                <div className="flex items-center gap-2 min-w-0">
                    <CircleDot className={cn('h-3 w-3 shrink-0', healthColor(site.health_status))} />
                    <h3 className="font-medium truncate">{site.name}</h3>
                    {site.pinned && <Badge variant="secondary" className="text-xs shrink-0">Pin</Badge>}
                </div>
                <div className="flex items-center gap-1 shrink-0">
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onOpenTokens(site.id)} title="Tokens">
                        <KeyRound className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onRefresh(site.id)}>
                        <RefreshCw className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => onEdit(site)}>
                        <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive" onClick={() => onDelete(site.id)}>
                        <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                </div>
            </div>
            <div className="text-xs text-muted-foreground space-y-1">
                <div className="flex items-center gap-1.5">
                    <Badge variant="outline" className="text-xs font-normal">{site.site_type}</Badge>
                    {!site.enabled && <Badge variant="destructive" className="text-xs">{t('disabled')}</Badge>}
                </div>
                <div className="flex items-center gap-1 truncate">
                    <ExternalLink className="h-3 w-3 shrink-0" />
                    <span className="truncate">{site.base_url}</span>
                </div>
                {site.remote_username && (
                    <div className="truncate">{t('user')}: {site.remote_username}</div>
                )}
            </div>
            <div className="flex items-center justify-between pt-1 border-t">
                <div className="text-sm font-medium">
                    {t('quota')}: <span className="tabular-nums">{site.quota.toFixed(2)}</span>
                </div>
                {site.last_sync_at && (
                    <div className="text-xs text-muted-foreground">
                        {new Date(site.last_sync_at).toLocaleString()}
                    </div>
                )}
            </div>
        </div>
    );
}

export function RemoteSite() {
    const { data: sites, isLoading, isError, refetch } = useRemoteSiteList();
    const deleteSite = useDeleteRemoteSite();
    const refreshSite = useRefreshRemoteSite();
    const refreshAll = useRefreshAllRemoteSites();
    const [dialogOpen, setDialogOpen] = useState(false);
    const [editingSite, setEditingSite] = useState<RemoteSiteModel | null>(null);
    const [tokenSiteId, setTokenSiteId] = useState<number | null>(null);
    const t = useTranslations('hub');

    const handleDelete = (id: number) => {
        if (!confirm(t('confirmDelete'))) return;
        deleteSite.mutate(id, {
            onSuccess: () => toast.success(t('deleted')),
            onError: (err) => toast.error(err.message),
        });
    };

    const handleRefresh = (id: number) => {
        refreshSite.mutate(id, {
            onSuccess: () => toast.success(t('refreshed')),
            onError: (err) => toast.error(err.message),
        });
    };

    const handleRefreshAll = () => {
        refreshAll.mutate(undefined, {
            onSuccess: () => toast.success(t('allRefreshed')),
            onError: (err) => toast.error(err.message),
        });
    };

    const handleEdit = (site: RemoteSiteModel) => {
        setEditingSite(site);
        setDialogOpen(true);
    };

    const handleCreate = () => {
        setEditingSite(null);
        setDialogOpen(true);
    };

    const handleOpenTokens = (id: number) => {
        setTokenSiteId(id);
    };

    if (isLoading) return <LoadingState />;
    if (isError) return <ErrorState onRetry={refetch} />;

    return (
        <div className="flex flex-col gap-4">
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <Globe className="h-5 w-5" />
                    <h2 className="text-lg font-semibold">{t('title')}</h2>
                    <Badge variant="secondary">{sites?.length ?? 0}</Badge>
                </div>
                <div className="flex items-center gap-2">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={handleRefreshAll}
                        disabled={refreshAll.isPending}
                    >
                        <RefreshCw className={cn('h-4 w-4 mr-1', refreshAll.isPending && 'animate-spin')} />
                        {t('refreshAll')}
                    </Button>
                    <Button size="sm" onClick={handleCreate}>
                        <Plus className="h-4 w-4 mr-1" />
                        {t('addSite')}
                    </Button>
                </div>
            </div>

            {sites && sites.length > 0 ? (
                <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
                    {sites.map((site) => (
                        <SiteCard
                            key={site.id}
                            site={site}
                            onEdit={handleEdit}
                            onDelete={handleDelete}
                            onRefresh={handleRefresh}
                            onOpenTokens={handleOpenTokens}
                        />
                    ))}
                </div>
            ) : (
                <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                    <Globe className="h-12 w-12 mb-3 opacity-50" />
                    <p>{t('empty')}</p>
                    <Button variant="outline" size="sm" className="mt-3" onClick={handleCreate}>
                        <Plus className="h-4 w-4 mr-1" />
                        {t('addSite')}
                    </Button>
                </div>
            )}

            <SiteDialog
                open={dialogOpen}
                onOpenChange={setDialogOpen}
                editingSite={editingSite}
            />

            {tokenSiteId && (
                <div className="fixed inset-0 z-50 bg-black/50 flex items-center justify-center p-4" onClick={() => setTokenSiteId(null)}>
                    <div className="bg-background rounded-lg shadow-lg w-full max-w-3xl max-h-[80vh] overflow-y-auto p-6" onClick={(e) => e.stopPropagation()}>
                        <div className="flex items-center justify-between mb-4">
                            <h3 className="text-lg font-semibold">{t('remoteTokens')}</h3>
                            <Button variant="ghost" size="sm" onClick={() => setTokenSiteId(null)}>✕</Button>
                        </div>
                        <TokenManager siteId={tokenSiteId} />
                    </div>
                </div>
            )}
        </div>
    );
}
