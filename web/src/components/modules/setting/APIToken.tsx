'use client';

import { useState } from 'react';
import { Copy, KeyRound, Plus, ShieldOff, Trash2 } from 'lucide-react';
import { useTranslations } from 'next-intl';
import {
    useAPITokenList,
    useCreateAPIToken,
    useDeleteAPIToken,
    useRevokeAPIToken,
    type APITokenCreateResponse,
    type APITokenView,
} from '@/api/endpoints/api-token';
import { useUserList } from '@/api/endpoints/user';
import { toast } from '@/components/common/Toast';
import { writeClipboardText } from '@/lib/clipboard';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { Hint } from '@/components/ui/hint';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

const NEVER_EXPIRES = '0';
const EXPIRY_OPTIONS = [NEVER_EXPIRES, '30', '90', '365'];

function formatSeconds(seconds: number) {
    if (!seconds) return '—';
    return new Date(seconds * 1000).toLocaleString();
}

function tokenState(token: APITokenView, nowSeconds: number) {
    if (token.revoked_at > 0) return 'revoked';
    if (token.expires_at > 0 && token.expires_at <= nowSeconds) return 'expired';
    if (!token.user_exists) return 'userMissing';
    return 'active';
}

const STATE_STYLES: Record<string, string> = {
    active: 'bg-emerald-500/12 text-emerald-600 dark:text-emerald-400',
    revoked: 'bg-destructive/12 text-destructive',
    expired: 'bg-amber-500/12 text-amber-600 dark:text-amber-400',
    userMissing: 'bg-destructive/12 text-destructive',
};

export function SettingAPIToken() {
    const t = useTranslations('setting');
    const { data: tokens, isLoading } = useAPITokenList();
    const { data: users } = useUserList();
    const createToken = useCreateAPIToken();
    const revokeToken = useRevokeAPIToken();
    const deleteToken = useDeleteAPIToken();

    const [name, setName] = useState('');
    const [username, setUsername] = useState('');
    const [expiresDays, setExpiresDays] = useState(NEVER_EXPIRES);
    const [created, setCreated] = useState<APITokenCreateResponse | null>(null);
    const [pending, setPending] = useState<{ action: 'revoke' | 'delete'; token: APITokenView } | null>(null);

    // Date.now() 是 impure，不能在 render 中直接调用（react-hooks/purity）；与 pool/index.tsx 同法取惰性初值。
    const [nowSeconds] = useState(() => Math.floor(Date.now() / 1000));
    const userList = users ?? [];

    const handleCreate = async () => {
        const trimmedName = name.trim();
        const trimmedUser = username.trim();
        if (!trimmedName) {
            toast.error(t('apiToken.needName'));
            return;
        }
        if (!trimmedUser) {
            toast.error(t('apiToken.needUser'));
            return;
        }
        try {
            const result = await createToken.mutateAsync({
                name: trimmedName,
                username: trimmedUser,
                expires_days: Number(expiresDays),
            });
            setCreated(result);
            setName('');
        } catch (error) {
            toast.error(error instanceof Error ? error.message : t('apiToken.createFailed'));
        }
    };

    const handleConfirm = async () => {
        if (!pending) return;
        const { action, token } = pending;
        try {
            if (action === 'revoke') {
                await revokeToken.mutateAsync(token.id);
                toast.success(t('apiToken.revoked'));
            } else {
                await deleteToken.mutateAsync(token.id);
                toast.success(t('apiToken.deleted'));
            }
            setPending(null);
        } catch (error) {
            toast.error(error instanceof Error ? error.message : t('apiToken.actionFailed'));
        }
    };

    const handleCopy = async () => {
        if (!created) return;
        try {
            await writeClipboardText(created.token);
            toast.success(t('apiToken.copied'));
        } catch {
            toast.error(t('apiToken.copyFailed'));
        }
    };

    return (
        <div className="relative overflow-hidden rounded-xl border-border/35 bg-card p-4 sm:p-6 text-card-foreground shadow-md">
            <div className="space-y-4 sm:space-y-5">
                <div className="space-y-1.5">
                    <h2 className="flex items-center gap-2 text-lg font-bold text-card-foreground">
                        <KeyRound className="h-5 w-5" />
                        {t('apiToken.title')}
                        <Hint text={t('apiToken.hint')} />
                    </h2>
                    <p className="text-sm text-muted-foreground">{t('apiToken.description')}</p>
                </div>

                <div className="grid grid-cols-1 gap-3 rounded-xl border border-border/30 bg-muted/10 p-4 sm:grid-cols-2 lg:grid-cols-4">
                    <Input
                        value={name}
                        onChange={(event) => setName(event.target.value)}
                        placeholder={t('apiToken.namePlaceholder')}
                        className="rounded-lg"
                        autoComplete="off"
                    />
                    {userList.length > 0 ? (
                        <Select value={username} onValueChange={setUsername}>
                            <SelectTrigger className="w-full rounded-lg">
                                <SelectValue placeholder={t('apiToken.boundUserPlaceholder')} />
                            </SelectTrigger>
                            <SelectContent className="rounded-lg">
                                {userList.map((item) => (
                                    <SelectItem key={item.id} value={item.username}>
                                        {item.username} · {item.role}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    ) : (
                        <Input
                            value={username}
                            onChange={(event) => setUsername(event.target.value)}
                            placeholder={t('apiToken.boundUserPlaceholder')}
                            className="rounded-lg"
                            autoComplete="off"
                        />
                    )}
                    <Select value={expiresDays} onValueChange={setExpiresDays}>
                        <SelectTrigger className="w-full rounded-lg">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent className="rounded-lg">
                            {EXPIRY_OPTIONS.map((option) => (
                                <SelectItem key={option} value={option}>
                                    {option === NEVER_EXPIRES
                                        ? t('apiToken.expiresNever')
                                        : t('apiToken.expiresDays', { days: option })}
                                </SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                    <Button onClick={handleCreate} disabled={createToken.isPending} className="rounded-lg">
                        <Plus className="size-4" />
                        {createToken.isPending ? t('apiToken.creating') : t('apiToken.create')}
                    </Button>
                </div>

                {isLoading ? (
                    <p className="text-sm text-muted-foreground">{t('apiToken.loading')}</p>
                ) : !tokens || tokens.length === 0 ? (
                    <p className="text-sm text-muted-foreground">{t('apiToken.empty')}</p>
                ) : (
                    <div className="divide-y divide-border/30 rounded-xl border border-border/30 bg-muted/10">
                        {tokens.map((token) => {
                            const state = tokenState(token, nowSeconds);
                            return (
                                <div key={token.id} className="flex flex-col gap-2 p-3 sm:flex-row sm:items-center sm:justify-between">
                                    <div className="min-w-0 space-y-1">
                                        <div className="flex flex-wrap items-center gap-2">
                                            <span className="text-sm font-semibold text-card-foreground">{token.name}</span>
                                            <span className="rounded-md bg-muted/40 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
                                                {token.prefix || '—'}…
                                            </span>
                                            <span className={`rounded-md px-1.5 py-0.5 text-[11px] font-medium ${STATE_STYLES[state]}`}>
                                                {t(`apiToken.state.${state}`)}
                                            </span>
                                        </div>
                                        <div className="flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-muted-foreground">
                                            <span>{t('apiToken.boundUser')}: {token.username || '—'}</span>
                                            <span>{t('apiToken.createdAt')}: {new Date(token.created_at).toLocaleString()}</span>
                                            <span>
                                                {t('apiToken.expiresAt')}:{' '}
                                                {token.expires_at > 0 ? formatSeconds(token.expires_at) : t('apiToken.expiresNever')}
                                            </span>
                                            <span>
                                                {t('apiToken.lastUsed')}:{' '}
                                                {token.last_used_at > 0
                                                    ? `${formatSeconds(token.last_used_at)}${token.last_used_ip ? ` · ${token.last_used_ip}` : ''}`
                                                    : t('apiToken.neverUsed')}
                                            </span>
                                        </div>
                                    </div>
                                    <div className="flex shrink-0 gap-2">
                                        {token.revoked_at === 0 && (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                className="rounded-lg"
                                                onClick={() => setPending({ action: 'revoke', token })}
                                            >
                                                <ShieldOff className="size-3.5" />
                                                {t('apiToken.revoke')}
                                            </Button>
                                        )}
                                        <Button
                                            variant="ghost"
                                            size="sm"
                                            className="rounded-lg text-destructive hover:text-destructive"
                                            onClick={() => setPending({ action: 'delete', token })}
                                        >
                                            <Trash2 className="size-3.5" />
                                            {t('apiToken.delete')}
                                        </Button>
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                )}
            </div>

            <Dialog open={created !== null} onOpenChange={(open) => !open && setCreated(null)}>
                <DialogContent className="rounded-xl sm:max-w-lg">
                    <DialogTitle>{t('apiToken.createdTitle')}</DialogTitle>
                    <p className="text-sm text-muted-foreground">{t('apiToken.createdHint')}</p>
                    <div className="flex items-center gap-2 rounded-lg border border-border/40 bg-muted/20 p-2">
                        <code className="min-w-0 flex-1 break-all font-mono text-xs text-card-foreground">{created?.token}</code>
                        <Button size="sm" variant="outline" className="rounded-lg shrink-0" onClick={handleCopy}>
                            <Copy className="size-3.5" />
                            {t('apiToken.copy')}
                        </Button>
                    </div>
                </DialogContent>
            </Dialog>

            <Dialog open={pending !== null} onOpenChange={(open) => !open && setPending(null)}>
                <DialogContent className="rounded-xl sm:max-w-md">
                    <DialogTitle>
                        {pending?.action === 'revoke' ? t('apiToken.revokeTitle') : t('apiToken.deleteTitle')}
                    </DialogTitle>
                    <p className="text-sm text-muted-foreground">
                        {pending?.action === 'revoke' ? t('apiToken.revokeConfirm') : t('apiToken.deleteConfirm')}
                    </p>
                    <div className="flex justify-end gap-2">
                        <Button variant="outline" className="rounded-lg" onClick={() => setPending(null)}>
                            {t('apiToken.cancel')}
                        </Button>
                        <Button
                            variant={pending?.action === 'delete' ? 'destructive' : 'default'}
                            className="rounded-lg"
                            disabled={revokeToken.isPending || deleteToken.isPending}
                            onClick={handleConfirm}
                        >
                            {pending?.action === 'revoke' ? t('apiToken.revoke') : t('apiToken.delete')}
                        </Button>
                    </div>
                </DialogContent>
            </Dialog>
        </div>
    );
}
