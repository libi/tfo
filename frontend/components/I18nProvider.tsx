'use client';

import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';
import { dateLocales, defaultLocale, detectPreferredLocale, messages, weekdaysShort, type Locale, type TranslationKey } from '@/lib/i18n';

interface I18nContextValue {
    locale: Locale;
    setLocale: (locale: Locale) => void;
    t: (key: TranslationKey) => string;
    dateLocale: (typeof dateLocales)[Locale];
    weekdayLabels: readonly string[];
}

const I18nContext = createContext<I18nContextValue | null>(null);

export function I18nProvider({ children }: { children: React.ReactNode }) {
    const [locale, setLocale] = useState<Locale>(defaultLocale);

    useEffect(() => {
        const savedLocale = window.localStorage.getItem('tfo-locale');
        setLocale(detectPreferredLocale(savedLocale ?? navigator.language));
    }, []);

    useEffect(() => {
        window.localStorage.setItem('tfo-locale', locale);
        document.documentElement.lang = locale;
    }, [locale]);

    const value = useMemo<I18nContextValue>(() => ({
        locale,
        setLocale,
        t: (key) => messages[locale][key],
        dateLocale: dateLocales[locale],
        weekdayLabels: weekdaysShort[locale],
    }), [locale]);

    return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
    const context = useContext(I18nContext);
    if (!context) {
        throw new Error('useI18n must be used within I18nProvider');
    }
    return context;
}