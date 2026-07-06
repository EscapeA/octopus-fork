import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { Plus, Edit2, Trash2, Send, Calendar, Clock } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import {
    useReportScheduleList,
    useCreateReportSchedule,
    useUpdateReportSchedule,
    useDeleteReportSchedule,
    useTestReportSchedule,
    REPORT_TYPES,
    REPORT_METRICS,
    DAYS_OF_WEEK,
    parseReportMetrics,
    formatReportSendTime,
    parseReportSendHour,
    type ReportMetric,
    type ReportSchedule,
} from '@/api/endpoints/report';
import { useAlertNotifChannelList } from '@/api/endpoints/alert';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';

interface NotifChannel {
    id: number;
    name: string;
}

export function ReportScheduleManager() {
    const t = useTranslations('report');
    const { data: schedules = [], isLoading } = useReportScheduleList();
    const { data: notifChannels = [] } = useAlertNotifChannelList();
    const createSchedule = useCreateReportSchedule();
    const updateSchedule = useUpdateReportSchedule();
    const deleteSchedule = useDeleteReportSchedule();
    const testSchedule = useTestReportSchedule();

    const [isDialogOpen, setIsDialogOpen] = useState(false);
    const [editingSchedule, setEditingSchedule] = useState<ReportSchedule | null>(null);

    const handleCreate = () => {
        setEditingSchedule(null);
        setIsDialogOpen(true);
    };

    const handleEdit = (schedule: ReportSchedule) => {
        setEditingSchedule(schedule);
        setIsDialogOpen(true);
    };

    const handleDelete = async (id: number) => {
        if (!confirm(t('confirmDelete'))) return;
        try {
            await deleteSchedule.mutateAsync(id);
            toast.success(t('deleteSuccess'));
        } catch {
            toast.error(t('deleteFailed'));
        }
    };

    const handleTest = async (id: number) => {
        try {
            await testSchedule.mutateAsync(id);
            toast.success(t('testSuccess'));
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : String(error);
            toast.error(message || t('testFailed'));
        }
    };

    const handleSave = async (data: Partial<ReportSchedule>) => {
        try {
            if (editingSchedule) {
                await updateSchedule.mutateAsync({ ...editingSchedule, ...data } as ReportSchedule);
                toast.success(t('updateSuccess'));
            } else {
                await createSchedule.mutateAsync(data);
                toast.success(t('createSuccess'));
            }
            setIsDialogOpen(false);
        } catch (error: unknown) {
            const message = error instanceof Error ? error.message : String(error);
            toast.error(message || t('saveFailed'));
        }
    };

    if (isLoading) {
        return <div className="text-center py-8">{t('loading')}</div>;
    }

    return (
        <div className="space-y-4">
            <div className="flex justify-between items-center">
                <h3 className="text-lg font-semibold">{t('scheduleTitle')}</h3>
                <Button onClick={handleCreate}>
                    <Plus className="w-4 h-4 mr-2" />
                    {t('createSchedule')}
                </Button>
            </div>

            {schedules.length === 0 ? (
                <div className="text-center py-12 text-muted-foreground">
                    {t('noSchedules')}
                </div>
            ) : (
                <div className="grid gap-4">
                    {schedules.map((schedule) => (
                        <ReportScheduleCard
                            key={schedule.id}
                            schedule={schedule}
                            notifChannelName={
                                notifChannels.find((c) => c.id === schedule.notif_channel_id)?.name || t('unknownChannel')
                            }
                            onEdit={() => handleEdit(schedule)}
                            onDelete={() => handleDelete(schedule.id)}
                            onTest={() => handleTest(schedule.id)}
                        />
                    ))}
                </div>
            )}

            <ReportScheduleDialog
                open={isDialogOpen}
                onOpenChange={setIsDialogOpen}
                schedule={editingSchedule}
                onSave={handleSave}
                notifChannels={notifChannels}
            />
        </div>
    );
}

function ReportScheduleCard({
    schedule,
    notifChannelName,
    onEdit,
    onDelete,
    onTest,
}: {
    schedule: ReportSchedule;
    notifChannelName: string;
    onEdit: () => void;
    onDelete: () => void;
    onTest: () => void;
}) {
    const t = useTranslations('report');
    const reportTypeLabel = REPORT_TYPES.find((r) => r.value === schedule.type)?.label || schedule.type;
    const metrics = parseReportMetrics(schedule.metrics);

    return (
        <div className="border rounded-lg p-4 space-y-3">
            <div className="flex justify-between items-start">
                <div className="space-y-1">
                    <div className="flex items-center gap-2">
                        <h4 className="font-medium">{schedule.name}</h4>
                        <Badge variant={schedule.enabled ? 'default' : 'secondary'}>
                            {schedule.enabled ? t('enabled') : t('disabled')}
                        </Badge>
                    </div>
                    <div className="flex items-center gap-4 text-sm text-muted-foreground">
                        <span className="flex items-center gap-1">
                            <Calendar className="w-4 h-4" />
                            {reportTypeLabel}
                        </span>
                        <span className="flex items-center gap-1">
                            <Clock className="w-4 h-4" />
                            {formatReportSendTime(schedule.send_hour)}
                        </span>
                        <span>{notifChannelName}</span>
                    </div>
                </div>
                <div className="flex gap-2">
                    <Button variant="ghost" size="sm" onClick={onTest}>
                        <Send className="w-4 h-4" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={onEdit}>
                        <Edit2 className="w-4 h-4" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={onDelete}>
                        <Trash2 className="w-4 h-4" />
                    </Button>
                </div>
            </div>

            <div className="flex flex-wrap gap-2">
                {metrics.map((metric) => {
                    const metricLabel = REPORT_METRICS.find((m) => m.value === metric)?.label || metric;
                    return (
                        <Badge key={metric} variant="outline">
                            {metricLabel}
                        </Badge>
                    );
                })}
            </div>

            {schedule.last_sent_at && (
                <div className="text-xs text-muted-foreground">
                    {t('lastSent')}: {new Date(schedule.last_sent_at).toLocaleString()}
                </div>
            )}
        </div>
    );
}

function ReportScheduleDialog({
    open,
    onOpenChange,
    schedule,
    onSave,
    notifChannels,
}: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    schedule: ReportSchedule | null;
    onSave: (data: Partial<ReportSchedule>) => void;
    notifChannels: NotifChannel[];
}) {
    const t = useTranslations('report');
    const [name, setName] = useState('');
    const [enabled, setEnabled] = useState(true);
    const [reportType, setReportType] = useState<'daily' | 'weekly' | 'monthly'>('daily');
    const [notifChannelId, setNotifChannelId] = useState<number>(0);
    const [metrics, setMetrics] = useState<ReportMetric[]>([]);
    const [sendTime, setSendTime] = useState('09:00');
    const [dayOfWeek, setDayOfWeek] = useState<number>(1);

    const handleOpenChange = (newOpen: boolean) => {
        if (newOpen && schedule) {
            setName(schedule.name);
            setEnabled(schedule.enabled);
            setReportType(schedule.type);
            setNotifChannelId(schedule.notif_channel_id);
            setMetrics(parseReportMetrics(schedule.metrics));
            setSendTime(formatReportSendTime(schedule.send_hour));
            setDayOfWeek(schedule.send_day_of_week || 1);
        } else if (newOpen) {
            setName('');
            setEnabled(true);
            setReportType('daily');
            setNotifChannelId(0);
            setMetrics([]);
            setSendTime('09:00');
            setDayOfWeek(1);
        }
        onOpenChange(newOpen);
    };

    const handleSave = () => {
        if (!name.trim()) {
            toast.error(t('nameRequired'));
            return;
        }
        if (notifChannelId === 0) {
            toast.error(t('channelRequired'));
            return;
        }
        if (metrics.length === 0) {
            toast.error(t('metricsRequired'));
            return;
        }

        onSave({
            name,
            enabled,
            type: reportType,
            notif_channel_id: notifChannelId,
            metrics: JSON.stringify(metrics),
            send_hour: parseReportSendHour(sendTime),
            send_day_of_week: reportType === 'weekly' ? dayOfWeek : 1,
            send_day_of_month: 1,
        });
    };

    const toggleMetric = (metric: ReportMetric) => {
        setMetrics((prev) =>
            prev.includes(metric) ? prev.filter((m) => m !== metric) : [...prev, metric]
        );
    };

    const handleReportTypeChange = (value: string) => {
        setReportType(value as 'daily' | 'weekly' | 'monthly');
    };

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>
                        {schedule ? t('editSchedule') : t('createSchedule')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('scheduleDescription')}
                    </DialogDescription>
                </DialogHeader>

                <div className="space-y-4 py-4">
                    <div className="space-y-2">
                        <Label>{t('name')}</Label>
                        <Input
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            placeholder={t('namePlaceholder')}
                        />
                    </div>

                    <div className="flex items-center space-x-2">
                        <Switch checked={enabled} onCheckedChange={setEnabled} />
                        <Label>{t('enabled')}</Label>
                    </div>

                    <div className="space-y-2">
                        <Label>{t('reportTypeLabel')}</Label>
                        <Select value={reportType} onValueChange={handleReportTypeChange}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {REPORT_TYPES.map((type) => (
                                    <SelectItem key={type.value} value={type.value}>
                                        {type.label}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    {reportType === 'weekly' && (
                        <div className="space-y-2">
                            <Label>{t('dayOfWeek')}</Label>
                            <Select
                                value={String(dayOfWeek)}
                                onValueChange={(v) => setDayOfWeek(Number(v))}
                            >
                                <SelectTrigger>
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {DAYS_OF_WEEK.map((day) => (
                                        <SelectItem key={day.value} value={String(day.value)}>
                                            {day.label}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    )}

                    <div className="space-y-2">
                        <Label>{t('sendTime')}</Label>
                        <Input
                            type="time"
                            value={sendTime}
                            onChange={(e) => setSendTime(e.target.value)}
                        />
                    </div>

                    <div className="space-y-2">
                        <Label>{t('notifChannel')}</Label>
                        <Select
                            value={String(notifChannelId)}
                            onValueChange={(v) => setNotifChannelId(Number(v))}
                        >
                            <SelectTrigger>
                                <SelectValue placeholder={t('selectChannel')} />
                            </SelectTrigger>
                            <SelectContent>
                                {notifChannels.map((channel) => (
                                    <SelectItem key={channel.id} value={String(channel.id)}>
                                        {channel.name}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="space-y-2">
                        <Label>{t('metrics')}</Label>
                        <div className="grid grid-cols-2 gap-2">
                            {REPORT_METRICS.map((metric) => (
                                <div
                                    key={metric.value}
                                    className={cn(
                                        'border rounded-md p-3 cursor-pointer transition-colors',
                                        metrics.includes(metric.value)
                                            ? 'border-primary bg-primary/5'
                                            : 'hover:border-primary/50'
                                    )}
                                    onClick={() => toggleMetric(metric.value)}
                                >
                                    <div className="flex items-center space-x-2">
                                        <div
                                            className={cn(
                                                'w-4 h-4 rounded border-2 flex items-center justify-center',
                                                metrics.includes(metric.value)
                                                    ? 'border-primary bg-primary'
                                                    : 'border-muted-foreground'
                                            )}
                                        >
                                            {metrics.includes(metric.value) && (
                                                <div className="w-2 h-2 bg-white rounded" />
                                            )}
                                        </div>
                                        <span className="text-sm">{metric.label}</span>
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                </div>

                <DialogFooter>
                    <Button variant="outline" onClick={() => onOpenChange(false)}>
                        {t('cancel')}
                    </Button>
                    <Button onClick={handleSave}>{t('save')}</Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
