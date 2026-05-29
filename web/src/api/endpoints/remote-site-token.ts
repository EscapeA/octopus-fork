import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

export interface RemoteSiteToken {
    id: number;
    remote_site_id: number;
    remote_token_id: number;
    name: string;
    key: string;
    status: number;
    remain_quota: number;
    used_quota: number;
    unlimited_quota: boolean;
    model_limits: string;
    expired_time: number;
    created_time: number;
    last_sync_at: string | null;
}

const TOKEN_KEYS = {
    all: ['remote-site-tokens'] as const,
    bySite: (siteId: number) => [...TOKEN_KEYS.all, siteId] as const,
};

export function useRemoteTokens(siteId: number, enabled = true) {
    return useQuery({
        queryKey: TOKEN_KEYS.bySite(siteId),
        queryFn: () => apiClient.get<RemoteSiteToken[]>(`/api/v1/remote-site-token/list/${siteId}`),
        enabled: enabled && siteId > 0,
    });
}

export function useSyncTokens() {
    const qc = useQueryClient();
    return useMutation({
        mutationFn: (siteId: number) =>
            apiClient.post<{ synced: number }>(`/api/v1/remote-site-token/sync/${siteId}`),
        onSuccess: (_, siteId) => {
            qc.invalidateQueries({ queryKey: TOKEN_KEYS.bySite(siteId) });
        },
    });
}

export function useSyncToChannel() {
    return useMutation({
        mutationFn: (data: {
            remote_site_id: number;
            token_id: number;
            channel_name?: string;
            models?: string;
        }) => apiClient.post<unknown>('/api/v1/remote-site-token/sync-to-channel', data),
    });
}
