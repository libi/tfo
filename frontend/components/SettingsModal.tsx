'use client';

import React, { useEffect, useMemo, useState } from 'react';
import { Globe, Keyboard, Settings, X } from 'lucide-react';
import { useI18n } from './I18nProvider';
import type { Locale } from '@/lib/i18n';
import { keyboardEventToShortcut, normalizeShortcut } from '@/lib/hotkeys';

interface SettingsModalProps {
    initialLocale: Locale;
    initialShortcut: string;
    isSaving: boolean;
    onClose: () => void;
    onSave: (settings: { locale: Locale; hotkeyInputFocus: string }) => Promise<void>;
}

export function SettingsModal({ initialLocale, initialShortcut, isSaving, onClose, onSave }: SettingsModalProps) {
    const { t } = useI18n();
    const [draftLocale, setDraftLocale] = useState<Locale>(initialLocale);
    const [draftShortcut, setDraftShortcut] = useState(normalizeShortcut(initialShortcut));
    const [isRecordingShortcut, setIsRecordingShortcut] = useState(false);
    const [error, setError] = useState<string | null>(null);

    useEffect(() => {
        setDraftLocale(initialLocale);
        setDraftShortcut(normalizeShortcut(initialShortcut));
    }, [initialLocale, initialShortcut]);

    const isDirty = useMemo(() => {
        return draftLocale !== initialLocale || draftShortcut !== normalizeShortcut(initialShortcut);
    }, [draftLocale, draftShortcut, initialLocale, initialShortcut]);

    const handleSave = async () => {
        const normalizedShortcut = normalizeShortcut(draftShortcut);
        if (!normalizedShortcut) {
            setError(t('quickCaptureShortcutEmpty'));
            return;
        }
        setError(null);
        await onSave({ locale: draftLocale, hotkeyInputFocus: normalizedShortcut });
    };

    const handleShortcutKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
        event.preventDefault();
        if (event.key === 'Escape') {
            setIsRecordingShortcut(false);
            return;
        }
        if (event.key === 'Backspace' || event.key === 'Delete') {
            setDraftShortcut('');
            setError(null);
            return;
        }
        const shortcut = keyboardEventToShortcut(event);
        if (!shortcut) return;
        setDraftShortcut(shortcut);
        setIsRecordingShortcut(false);
        setError(null);
    };

    return (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/20 backdrop-blur-sm">
            <div className="relative w-full max-w-xl overflow-hidden rounded-2xl bg-white shadow-xl animate-in fade-in zoom-in-95 duration-200">
                <button
                    onClick={onClose}
                    className="absolute right-4 top-4 text-gray-400 transition-colors hover:text-gray-600"
                    aria-label={t('cancelButton')}
                >
                    <X size={20} />
                </button>

                <div className="border-b border-gray-100 px-8 py-6">
                    <div className="flex items-center gap-3">
                        <div className="flex h-11 w-11 items-center justify-center rounded-full bg-gray-100 text-gray-700">
                            <Settings size={20} />
                        </div>
                        <div>
                            <h2 className="text-xl font-semibold text-gray-900">{t('settingsTitle')}</h2>
                            <p className="mt-1 text-sm text-gray-500">{t('settingsDescription')}</p>
                        </div>
                    </div>
                </div>

                <div className="space-y-6 px-8 py-6">
                    <section className="rounded-2xl border border-gray-100 bg-gray-50 p-5">
                        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                            <Globe size={16} />
                            {t('settingsGeneralTitle')}
                        </div>
                        <div>
                            <label className="mb-3 block text-sm font-medium text-gray-700">{t('languageSettingLabel')}</label>
                            <div className="flex gap-2">
                                <button
                                    onClick={() => setDraftLocale('en')}
                                    className={`rounded-lg px-3 py-2 text-sm transition-colors ${draftLocale === 'en' ? 'bg-gray-900 text-white' : 'bg-white text-gray-600 hover:bg-gray-100'}`}
                                >
                                    {t('localeEnglish')}
                                </button>
                                <button
                                    onClick={() => setDraftLocale('zh-CN')}
                                    className={`rounded-lg px-3 py-2 text-sm transition-colors ${draftLocale === 'zh-CN' ? 'bg-gray-900 text-white' : 'bg-white text-gray-600 hover:bg-gray-100'}`}
                                >
                                    {t('localeChinese')}
                                </button>
                            </div>
                        </div>
                    </section>

                    <section className="rounded-2xl border border-gray-100 bg-gray-50 p-5">
                        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                            <Keyboard size={16} />
                            {t('quickCaptureTitle')}
                        </div>
                        <div className="space-y-3">
                            <div>
                                <div className="text-sm font-medium text-gray-700">{t('quickCaptureActionLabel')}</div>
                                <p className="mt-1 text-sm text-gray-500">{t('quickCaptureShortcutHint')}</p>
                            </div>
                            <div>
                                <label className="mb-2 block text-sm font-medium text-gray-700">{t('quickCaptureShortcutLabel')}</label>
                                <button
                                    type="button"
                                    onFocus={() => setIsRecordingShortcut(true)}
                                    onBlur={() => setIsRecordingShortcut(false)}
                                    onKeyDown={handleShortcutKeyDown}
                                    className="flex w-full items-center justify-between rounded-xl border border-gray-200 bg-white px-4 py-3 text-left transition-colors hover:border-gray-300 focus:border-gray-400 focus:outline-none"
                                >
                                    <span className="font-mono text-sm text-gray-900">{draftShortcut || '—'}</span>
                                    <span className="text-xs text-gray-400">
                                        {isRecordingShortcut ? t('quickCaptureShortcutRecording') : t('quickCaptureShortcutHint')}
                                    </span>
                                </button>
                            </div>
                            {error && <p className="text-sm text-red-600">{error}</p>}
                        </div>
                    </section>
                </div>

                <div className="flex items-center justify-end gap-3 border-t border-gray-100 px-8 py-4">
                    <button
                        onClick={onClose}
                        className="rounded-lg border border-gray-200 px-4 py-2 text-sm text-gray-600 transition-colors hover:bg-gray-50"
                    >
                        {t('cancelButton')}
                    </button>
                    <button
                        onClick={handleSave}
                        disabled={isSaving || !isDirty}
                        className="rounded-lg bg-gray-900 px-4 py-2 text-sm text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                        {isSaving ? t('savingButton') : t('saveButton')}
                    </button>
                </div>
            </div>
        </div>
    );
}