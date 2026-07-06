import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '../client';
import { REFETCH_INTERVAL_CONFIG } from '../constants';
import { logger } from '@/lib/logger';

export type ReportType = 'daily' | 'weekly' | 'monthly';
export type ReportMetric =
    | 'overview'
    | 'top_models'
    | 'top_channels'
    | 'top_apikeys'
    | 'cost_breakdown'
    | 'error_analysis'
    | 'daily_trend';

export interface ReportSchedule {
    id: number;
    name: string;
    enabled: boolean;
    type: ReportType;
    notif_channel_id: number;
    metrics: string; // JSON array of metric names
    send_hour: number;
    send_day_of_week: number; // 0-6 for weekly reports
    send_day_of_month: number; // 1-28 for monthly reports
    last_sent_at?: number;
}

export interface ReportHistory {
    id: number;
    schedule_id: number;
    schedule_name: string;
    type: ReportType;
    title: string;
    content: string;
    send_status: 'sent' | 'failed' | 'skipped';
    send_detail: string;
    sent_at: number;
}

export const REPORT_TYPES = [
    { value: 'daily', label: '每日报告' },
    { value: 'weekly', label: '每周报告' },
    { value: 'monthly', label: '每月报告' },
] as const;

export const REPORT_METRICS = [
    { value: 'overview', label: '总览' },
    { value: 'top_models', label: '热门模型' },
    { value: 'top_channels', label: '热门渠道' },
    { value: 'top_apikeys', label: '热门 API Key' },
    { value: 'cost_breakdown', label: '成本明细' },
    { value: 'error_analysis', label: '错误分析' },
    { value: 'daily_trend', label: '每日趋势' },
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

export function parseReportMetrics(metrics: string): ReportMetric[] {
    try {
        const parsed = JSON.parse(metrics);
        return Array.isArray(parsed) ? parsed : [];
    } catch {
        return [];
    }
}

export function formatReportSendTime(sendHour: number): string {
    const hour = Number.isFinite(sendHour) ? Math.max(0, Math.min(23, sendHour)) : 0;
    return `${String(hour).padStart(2, '0')}:00`;
}

export function parseReportSendHour(sendTime: string): number {
    const hour = Number.parseInt(sendTime.split(':')[0] || '0', 10);
    if (!Number.isFinite(hour)) return 0;
    return Math.max(0, Math.min(23, hour));
}

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
