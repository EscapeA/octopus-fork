import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient, API_BASE_URL } from '../client';
import { useAuthStore } from './user';

function getAuthHeader(): string {
    const token = useAuthStore.getState().token;
    if (!token) throw new Error('Not authenticated');
    return `Bearer ${token}`;
}

function parseFilename(contentDisposition: string | null): string | null {
    if (!contentDisposition) return null;
    const match = contentDisposition.match(/filename="([^"]+)"/i);
    return match?.[1] ?? null;
}

async function downloadBlob(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob);
    try {
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        a.remove();
    } finally {
        URL.revokeObjectURL(url);
    }
}

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

export interface BatchExportResult {
    site_name: string;
    site_base_url: string;
    exported_at: string;
    total_tokens: number;
    active_tokens: number;
    tokens: Array<{
        name: string;
        key: string;
        base_url: string;
        status: number;
        remain_quota: number;
        used_quota: number;
        unlimited_quota: boolean;
        model_limits?: string;
        expired_time: number;
    }>;
}

export function useExportTokens() {
    return useMutation({
        mutationFn: async (siteId: number) => {
            const res = await fetch(`${API_BASE_URL}/api/v1/remote-site-token/export/${siteId}`, {
                method: 'GET',
                headers: {
                    Authorization: getAuthHeader(),
                },
            });

            if (!res.ok) {
                const text = await res.text();
                throw new Error(text || res.statusText);
            }

            const blob = await res.blob();
            const filename = parseFilename(res.headers.get('content-disposition')) || `tokens-${siteId}.json`;
            await downloadBlob(blob, filename);
            return { filename };
        },
    });
}
