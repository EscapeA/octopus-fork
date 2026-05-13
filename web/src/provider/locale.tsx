'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { NextIntlClientProvider } from 'next-intl';
import { DEFAULT_TIME_ZONE, normalizeLocale, normalizeTimeZone, useSettingStore, type Locale } from '@/stores/setting';

import zh_hansMessages from '../../public/locale/zh_hans.json';
import zh_hantMessages from '../../public/locale/zh_hant.json';
import enMessages from '../../public/locale/en.json';

const messages: Record<Locale, typeof zh_hansMessages> = {
    'zh-Hans': zh_hansMessages,
    'zh-Hant': zh_hantMessages,
    en: enMessages,
};

export function LocaleProvider({ children }: { children: ReactNode }) {
    const { locale, timeZone } = useSettingStore();
    const [currentLocale, setCurrentLocale] = useState<Locale>('zh-Hans');
    const [currentTimeZone, setCurrentTimeZone] = useState(DEFAULT_TIME_ZONE);

    useEffect(() => {
        setCurrentLocale(normalizeLocale(locale));
        setCurrentTimeZone(normalizeTimeZone(timeZone));
    }, [locale, timeZone]);

    return (
        <NextIntlClientProvider
            locale={currentLocale}
            messages={messages[currentLocale]}
            timeZone={currentTimeZone}
        >
            {children}
        </NextIntlClientProvider>
    );
}
