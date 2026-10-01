import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';

/**
 * 机器令牌（api_tokens）：脚本 / agent / CI 免登录调用管理面 API 的凭证。
 * 明文只在创建响应里返回一次，列表只含前缀与元信息。
 */
export interface APITokenView {
    id: number;
    name: string;
    prefix: string;
    user_id: number;
    username: string;
    user_exists: boolean;
    expires_at: number;
    last_used_at: number;
    last_used_ip: string;
    revoked_at: number;
    created_by: number;
    created_at: string;
}

export interface APITokenCreateRequest {
    name: string;
    username: string;
    /** 0 = 永不过期 */
    expires_days: number;
}

export interface APITokenCreateResponse {
    token: string;
    id: number;
    name: string;
    prefix: string;
    user_id: number;
    username: string;
    expires_at: number;
}

const API_TOKEN_QUERY_KEY = ['api-tokens', 'list'];

export function useAPITokenList() {
    return useQuery({
        queryKey: API_TOKEN_QUERY_KEY,
        queryFn: async () => apiClient.get<APITokenView[]>('/api/v1/api-token/list'),
    });
}

export function useCreateAPIToken() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data: APITokenCreateRequest) =>
            apiClient.post<APITokenCreateResponse>('/api/v1/api-token/create', data),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: API_TOKEN_QUERY_KEY });
        },
    });
}

export function useRevokeAPIToken() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (id: number) => apiClient.post<null>('/api/v1/api-token/revoke', { id }),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: API_TOKEN_QUERY_KEY });
        },
    });
}

export function useDeleteAPIToken() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (id: number) => apiClient.delete<null>(`/api/v1/api-token/delete/${id}`),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: API_TOKEN_QUERY_KEY });
        },
    });
}
