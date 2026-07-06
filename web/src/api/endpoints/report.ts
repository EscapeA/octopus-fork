import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import { REFETCH_INTERVAL_CONFIG } from '../constants';
import { logger } from '@/lib/logger';

export interface ReportSchedule {
    id: number;
    name: string;
    enabled: boolean;
    report_type: 'daily' | 'weekly' | 'monthly';
    notif_channel_id: number;
    metrics: string[]; // JSON array of metric names
    send_time: string; // HH:MM format
    day_of_week?: number; // 0-6 for weekly reports
    last_sent_at?: string;
}

export interface ReportHistory {
    id: number;
    schedule_id: number;
    report_type: 'daily' | 'weekly' | 'monthly';
    status: 'success' | 'failed' | 'pending';
    sent_at: string;
    error_message?: string;
    duration_ms?: number;
}

export const REPORT_TYPES = [
    { value: 'daily', label: '每日报告' },
    { value: 'weekly', label: '每周报告' },
    { value: 'monthly', label: '每月报告' },
] as const;

export const REPORT_METRICS = [
    { value: 'request_count', label: '请求总数' },
    { value: 'success_rate', label: '成功率' },
    { value: 'avg_latency', label: '平均延迟' },
    { value: 'p95_latency', label: 'P95延迟' },
    { value: 'token_usage', label: 'Token使用量' },
    { value: 'cost', label: '成本' },
    { value: 'top_models', label: '热门模型' },
    { value: 'top_channels', label: '热门渠道' },
] as const;

export const DAYS_OF_WEEK = [
    { value: 0, label: '周日' },
    { value: 1, label: '周一' },
    { value: 2, label: '周二' },
    { value: 3, label: '周三' },
    { value: 4, label: '周四' },
    { value: 5, label: '周五' },
    { value: 6, label: '周六' },
] as const;

export function useReportScheduleList() {
    return useQuery({
        queryKey: ['reports', 'schedules'],
        queryFn: async () => apiClient.get<ReportSchedule[]>('/api/v1/report/schedule/list'),
        refetchInterval: REFETCH_INTERVAL_CONFIG,
    });
}

export function useCreateReportSchedule() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data: Partial<ReportSchedule>) => {
            return apiClient.post<ReportSchedule>('/api/v1/report/schedule/create', data);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['reports', 'schedules'] });
        },
        onError: (error) => logger.error('Create report schedule failed:', error),
    });
}

export function useUpdateReportSchedule() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (data: ReportSchedule) => {
            return apiClient.post<null>('/api/v1/report/schedule/update', data);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['reports', 'schedules'] });
        },
        onError: (error) => logger.error('Update report schedule failed:', error),
    });
}

export function useDeleteReportSchedule() {
    const queryClient = useQueryClient();
    return useMutation({
        mutationFn: async (id: number) => {
            return apiClient.delete<null>(`/api/v1/report/schedule/delete/${id}`);
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['reports', 'schedules'] });
        },
        onError: (error) => logger.error('Delete report schedule failed:', error),
    });
}

export function useTestReportSchedule() {
    return useMutation({
        mutationFn: async (id: number) => {
            return apiClient.post<null>(`/api/v1/report/schedule/test`, { id });
        },
        onError: (error) => logger.error('Test report schedule failed:', error),
    });
}

export function useReportHistory(limit: number = 50) {
    return useQuery({
        queryKey: ['reports', 'history', limit],
        queryFn: async () => apiClient.get<ReportHistory[]>(`/api/v1/report/history/list?limit=${limit}`),
        refetchInterval: REFETCH_INTERVAL_CONFIG,
    });
}
