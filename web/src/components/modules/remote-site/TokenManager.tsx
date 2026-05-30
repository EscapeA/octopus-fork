'use client';

import {
    useRemoteTokens,
    useSyncTokens,
    useSyncToChannel,
    useExportTokens,
    type RemoteSiteToken,
} from '@/api/endpoints/remote-site-token';
import { LoadingState } from '@/components/common/LoadingState';
import { RefreshCw, Download, KeyRound, Upload } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { toast } from '@/components/common/Toast';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';
import { useTranslations } from 'next-intl';

function tokenStatusBadge(status: number) {
    switch (status) {
        case 1: return <Badge variant="secondary" className="text-green-500">Active</Badge>;
        case 2: return <Badge variant="destructive">Disabled</Badge>;
        case 3: return <Badge variant="destructive">Expired</Badge>;
        default: return <Badge variant="outline">Unknown</Badge>;
    }
}

export function TokenManager({ siteId }: { siteId: number }) {
    const { data: tokens, isLoading } = useRemoteTokens(siteId);
    const syncTokens = useSyncTokens();
    const syncToChannel = useSyncToChannel();
    const exportTokens = useExportTokens();
    const t = useTranslations('hub');

    const handleSync = () => {
        syncTokens.mutate(siteId, {
            onSuccess: (data) => toast.success(t('tokensSynced', { count: data.synced })),
            onError: (err) => toast.error(err.message),
        });
    };

    const handleImport = (token: RemoteSiteToken) => {
        syncToChannel.mutate({
            remote_site_id: siteId,
            token_id: token.id,
        }, {
            onSuccess: () => toast.success(t('tokenImported')),
            onError: (err) => toast.error(err.message),
        });
    };

    const handleExport = () => {
        exportTokens.mutate(siteId, {
            onSuccess: () => toast.success(t('tokensExported')),
            onError: (err) => toast.error(err.message),
        });
    };

    if (isLoading) return <LoadingState />;

    return (
        <div className="flex flex-col gap-3">
            <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                    <KeyRound className="h-4 w-4" />
                    <h3 className="text-sm font-medium">{t('remoteTokens')}</h3>
                    <Badge variant="secondary">{tokens?.length ?? 0}</Badge>
                </div>
                <div className="flex items-center gap-2">
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={handleExport}
                        disabled={exportTokens.isPending || !tokens?.length}
                    >
                        <Upload className={cn('h-3.5 w-3.5 mr-1')} />
                        {t('exportTokens')}
                    </Button>
                    <Button
                        variant="outline"
                        size="sm"
                        onClick={handleSync}
                        disabled={syncTokens.isPending}
                    >
                        <RefreshCw className={cn('h-3.5 w-3.5 mr-1', syncTokens.isPending && 'animate-spin')} />
                        {t('syncTokens')}
                    </Button>
                </div>
            </div>

            {tokens && tokens.length > 0 ? (
                <div className="rounded-md border overflow-hidden">
                    <table className="w-full text-sm">
                        <thead className="bg-muted/50">
                            <tr>
                                <th className="text-left p-2 font-medium">{t('tokenName')}</th>
                                <th className="text-left p-2 font-medium">{t('status')}</th>
                                <th className="text-right p-2 font-medium">{t('remainQuota')}</th>
                                <th className="text-right p-2 font-medium">{t('usedQuota')}</th>
                                <th className="text-right p-2 font-medium">{t('actions')}</th>
                            </tr>
                        </thead>
                        <tbody>
                            {tokens.map((token) => (
                                <tr key={token.id} className="border-t">
                                    <td className="p-2 truncate max-w-[200px]">{token.name}</td>
                                    <td className="p-2">{tokenStatusBadge(token.status)}</td>
                                    <td className="p-2 text-right tabular-nums">
                                        {token.unlimited_quota ? '∞' : token.remain_quota.toFixed(2)}
                                    </td>
                                    <td className="p-2 text-right tabular-nums">{token.used_quota.toFixed(2)}</td>
                                    <td className="p-2 text-right">
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            className="h-7"
                                            onClick={() => handleImport(token)}
                                            disabled={syncToChannel.isPending}
                                        >
                                            <Download className="h-3 w-3 mr-1" />
                                            {t('importAsChannel')}
                                        </Button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </div>
            ) : (
                <p className="text-sm text-muted-foreground text-center py-4">{t('noTokens')}</p>
            )}
        </div>
    );
}
