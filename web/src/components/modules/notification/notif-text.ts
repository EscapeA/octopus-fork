import type { NotificationItem } from '@/api/endpoints/notification';

/**
 * 解析通知的本地化标题/正文。
 *
 * 后端为每条通知存储 i18n 键 + 参数（title_key/content_key + *_args JSON），
 * 前端按当前 UI 语言用 t() 渲染。键为空（历史通知）时回退到 title/content 原文。
 *
 * `t` 为 next-intl 的翻译函数，命名空间为 'notif'，按 `${key}.title` / `${key}.content`
 * 取模板。参数对象由 *_args JSON 反序列化得到，直接作为 t() 的第二参数（ICU 插值）。
 */
export function resolveNotifTitle(item: NotificationItem, t: (key: string, params?: Record<string, unknown>) => string): string {
    if (item.title_key) {
        try {
            const args = item.title_args ? JSON.parse(item.title_args) : undefined;
            return t(`${item.title_key}.title`, args);
        } catch {
            // 参数 JSON 解析失败，回退到原始 title
        }
    }
    return item.title;
}

export function resolveNotifContent(item: NotificationItem, t: (key: string, params?: Record<string, unknown>) => string): string {
    if (item.content_key) {
        try {
            const args = item.content_args ? JSON.parse(item.content_args) : undefined;
            return t(`${item.content_key}.content`, args);
        } catch {
            // 参数 JSON 解析失败，回退到原始 content
        }
    }
    return item.content;
}
