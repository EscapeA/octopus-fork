import { useTranslations } from 'next-intl';
import { CheckCircle2, XCircle, Clock } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { useReportHistory } from '@/api/endpoints/report';

export function ReportHistoryList() {
    const t = useTranslations('report');
    const { data: history = [], isLoading } = useReportHistory(50);

    if (isLoading) {
        return <div className="text-center py-8">{t('loading')}</div>;
    }

    if (history.length === 0) {
        return (
            <div className="text-center py-12 text-muted-foreground">
                {t('noHistory')}
            </div>
        );
    }

    return (
        <div className="space-y-3">
            <h3 className="text-lg font-semibold">{t('historyTitle')}</h3>
            <div className="space-y-2">
                {history.map((record) => (
                    <div
                        key={record.id}
                        className="border rounded-lg p-4 flex items-center justify-between"
                    >
                        <div className="flex items-center gap-3">
                            {record.status === 'success' ? (
                                <CheckCircle2 className="w-5 h-5 text-green-500" />
                            ) : record.status === 'failed' ? (
                                <XCircle className="w-5 h-5 text-red-500" />
                            ) : (
                                <Clock className="w-5 h-5 text-yellow-500" />
                            )}
                            <div>
                                <div className="font-medium">
                                    {t(`reportType.${record.report_type}`)}
                                </div>
                                <div className="text-sm text-muted-foreground">
                                    {new Date(record.sent_at).toLocaleString()}
                                </div>
                                {record.error_message && (
                                    <div className="text-sm text-red-500 mt-1">
                                        {record.error_message}
                                    </div>
                                )}
                            </div>
                        </div>
                        <div className="flex items-center gap-2">
                            <Badge variant="outline">
                                {t(`reportType.${record.report_type}`)}
                            </Badge>
                            {record.duration_ms && (
                                <span className="text-xs text-muted-foreground">
                                    {record.duration_ms}ms
                                </span>
                            )}
                        </div>
                    </div>
                ))}
            </div>
        </div>
    );
}
