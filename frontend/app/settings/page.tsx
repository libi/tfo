'use client';

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
    ArrowLeft,
    Database,
    Globe,
    Keyboard,
    MessageSquare,
    Settings,
    TriangleAlert,
    Wrench,
} from 'lucide-react';
import Link from 'next/link';
import { useI18n } from '@/components/I18nProvider';
import { isLocale, type Locale } from '@/lib/i18n';
import { keyboardEventToShortcut, normalizeShortcut } from '@/lib/hotkeys';
import * as api from '@/lib/api';

export default function SettingsPage() {
    const { locale, setLocale, t } = useI18n();
    const [mounted, setMounted] = useState(false);
    const [config, setConfig] = useState<api.AppConfig | null>(null);
    const [isSaving, setIsSaving] = useState(false);
    const [toast, setToast] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

    // Draft state for all config fields
    const [draftLocale, setDraftLocale] = useState<Locale>(locale);
    const [draftShortcut, setDraftShortcut] = useState('Alt+S');
    const [isRecordingShortcut, setIsRecordingShortcut] = useState(false);
    const [draftWechat, setDraftWechat] = useState<api.WeChatConfig>({
        enabled: false,
        baseUrl: '',
        token: '',
        cdnBaseUrl: '',
        autoConnect: true,
        pollTimeoutSeconds: 35,
        reconnectIntervalSec: 5,
    });
    const [draftIndexRebuild, setDraftIndexRebuild] = useState(false);
    const [draftTitleMinLength, setDraftTitleMinLength] = useState(300);

    // Data dir change
    const [newDataDir, setNewDataDir] = useState('');
    const [showDataDirConfirm, setShowDataDirConfirm] = useState(false);
    const [dataDirSaving, setDataDirSaving] = useState(false);

    useEffect(() => { setMounted(true); }, []);

    const loadConfig = useCallback(async () => {
        try {
            const cfg = await api.getConfig();
            setConfig(cfg);
            if (cfg.uiLanguage && isLocale(cfg.uiLanguage)) {
                setDraftLocale(cfg.uiLanguage as Locale);
            }
            setDraftShortcut(normalizeShortcut(cfg.hotkeyQuickCapture) || 'Alt+S');
            setDraftWechat(cfg.wechat);
            setDraftIndexRebuild(cfg.indexRebuildOnStart);
            setDraftTitleMinLength(cfg.titleMinContentLength);
        } catch (err) {
            console.error('Failed to load config:', err);
        }
    }, []);

    useEffect(() => {
        if (mounted) loadConfig();
    }, [mounted, loadConfig]);

    const isDirty = useMemo(() => {
        if (!config) return false;
        const origLocale = config.uiLanguage && isLocale(config.uiLanguage) ? config.uiLanguage : locale;
        return (
            draftLocale !== origLocale ||
            draftShortcut !== normalizeShortcut(config.hotkeyQuickCapture) ||
            draftIndexRebuild !== config.indexRebuildOnStart ||
            draftTitleMinLength !== config.titleMinContentLength ||
            JSON.stringify(draftWechat) !== JSON.stringify(config.wechat)
        );
    }, [config, draftLocale, draftShortcut, draftWechat, draftIndexRebuild, draftTitleMinLength, locale]);

    const showToast = (type: 'success' | 'error', message: string) => {
        setToast({ type, message });
        setTimeout(() => setToast(null), type === 'success' ? 2200 : 4000);
    };

    const handleSave = async () => {
        if (!config) return;
        setIsSaving(true);
        try {
            const nextConfig: api.AppConfig = {
                ...config,
                uiLanguage: draftLocale,
                hotkeyQuickCapture: normalizeShortcut(draftShortcut) || 'Alt+S',
                wechat: draftWechat,
                indexRebuildOnStart: draftIndexRebuild,
                titleMinContentLength: draftTitleMinLength,
            };
            const saved = await api.updateConfig(nextConfig);
            setConfig(saved);
            setLocale(draftLocale);
            showToast('success', t('quickCaptureSaveSuccess'));
        } catch (err) {
            console.error('Failed to save config:', err);
            showToast('error', t('quickCaptureSaveError'));
        } finally {
            setIsSaving(false);
        }
    };

    const handleShortcutKeyDown = (event: React.KeyboardEvent<HTMLButtonElement>) => {
        event.preventDefault();
        if (event.key === 'Escape') { setIsRecordingShortcut(false); return; }
        if (event.key === 'Backspace' || event.key === 'Delete') { setDraftShortcut(''); return; }
        const shortcut = keyboardEventToShortcut(event);
        if (!shortcut) return;
        setDraftShortcut(shortcut);
        setIsRecordingShortcut(false);
    };

    const handleDataDirChange = async () => {
        if (!newDataDir.trim()) return;
        setDataDirSaving(true);
        try {
            await api.updateBootstrap(newDataDir.trim());
            showToast('success', t('settingsDataDirRestartHint'));
            setShowDataDirConfirm(false);
            setNewDataDir('');
        } catch (err) {
            console.error('Failed to update data dir:', err);
            showToast('error', String(err));
        } finally {
            setDataDirSaving(false);
        }
    };

    const updateWechatField = <K extends keyof api.WeChatConfig>(key: K, value: api.WeChatConfig[K]) => {
        setDraftWechat(prev => ({ ...prev, [key]: value }));
    };

    if (!mounted || !config) {
        return <div className="flex h-screen items-center justify-center bg-[#fafafa]"><Settings className="animate-spin text-gray-300" size={24} /></div>;
    }

    return (
        <div className="min-h-screen bg-[#fafafa]">
            <div className="mx-auto max-w-2xl px-6 py-8">
                {/* Header */}
                <div className="mb-8 flex items-center gap-4">
                    <Link href="/" className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-800 transition-colors">
                        <ArrowLeft size={16} />
                        {t('settingsBackToHome')}
                    </Link>
                </div>
                <div className="mb-8 flex items-center gap-3">
                    <div className="flex h-11 w-11 items-center justify-center rounded-full bg-gray-100 text-gray-700">
                        <Settings size={20} />
                    </div>
                    <div>
                        <h1 className="text-xl font-semibold text-gray-900">{t('settingsTitle')}</h1>
                        <p className="mt-0.5 text-sm text-gray-500">{t('settingsDescription')}</p>
                    </div>
                </div>

                <div className="space-y-6">
                    {/* Data Directory */}
                    <section className="rounded-2xl border border-gray-100 bg-white p-6">
                        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                            <Database size={16} />
                            {t('settingsDataDirTitle')}
                        </div>
                        <div className="space-y-3">
                            <div>
                                <label className="mb-1 block text-sm font-medium text-gray-700">{t('settingsDataDirLabel')}</label>
                                <div className="rounded-lg border border-gray-200 bg-gray-50 px-4 py-2.5 font-mono text-sm text-gray-700">
                                    {config.dataDir || '—'}
                                </div>
                            </div>
                            {!showDataDirConfirm ? (
                                <button
                                    onClick={() => { setNewDataDir(config.dataDir || ''); setShowDataDirConfirm(true); }}
                                    className="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 transition-colors hover:bg-gray-50"
                                >
                                    {t('settingsDataDirChangeButton')}
                                </button>
                            ) : (
                                <div className="space-y-3 rounded-xl border border-amber-200 bg-amber-50 p-4">
                                    <div className="flex items-start gap-2 text-sm text-amber-800">
                                        <TriangleAlert size={16} className="mt-0.5 flex-shrink-0" />
                                        <span>{t('settingsDataDirChangeWarning')}</span>
                                    </div>
                                    <input
                                        type="text"
                                        value={newDataDir}
                                        onChange={e => setNewDataDir(e.target.value)}
                                        placeholder={t('settingsDataDirPlaceholder')}
                                        className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 font-mono text-sm text-gray-800 focus:border-gray-400 focus:outline-none"
                                    />
                                    <p className="text-xs text-amber-700">{t('settingsDataDirRestartHint')}</p>
                                    <div className="flex gap-2">
                                        <button
                                            onClick={handleDataDirChange}
                                            disabled={dataDirSaving || !newDataDir.trim() || newDataDir.trim() === config.dataDir}
                                            className="rounded-lg bg-amber-600 px-3 py-1.5 text-sm text-white transition-colors hover:bg-amber-700 disabled:cursor-not-allowed disabled:opacity-50"
                                        >
                                            {dataDirSaving ? t('savingButton') : t('settingsDataDirChangeButton')}
                                        </button>
                                        <button
                                            onClick={() => setShowDataDirConfirm(false)}
                                            className="rounded-lg border border-gray-200 px-3 py-1.5 text-sm text-gray-600 transition-colors hover:bg-gray-50"
                                        >
                                            {t('cancelButton')}
                                        </button>
                                    </div>
                                </div>
                            )}
                        </div>
                    </section>

                    {/* General: Language */}
                    <section className="rounded-2xl border border-gray-100 bg-white p-6">
                        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                            <Globe size={16} />
                            {t('settingsGeneralTitle')}
                        </div>
                        <div>
                            <label className="mb-2 block text-sm font-medium text-gray-700">{t('languageSettingLabel')}</label>
                            <div className="flex gap-2">
                                {(['en', 'zh-CN'] as Locale[]).map(loc => (
                                    <button
                                        key={loc}
                                        onClick={() => setDraftLocale(loc)}
                                        className={`rounded-lg px-3 py-2 text-sm transition-colors ${draftLocale === loc ? 'bg-gray-900 text-white' : 'bg-gray-50 text-gray-600 hover:bg-gray-100'}`}
                                    >
                                        {loc === 'en' ? t('localeEnglish') : t('localeChinese')}
                                    </button>
                                ))}
                            </div>
                        </div>
                    </section>

                    {/* Quick Capture */}
                    <section className="rounded-2xl border border-gray-100 bg-white p-6">
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
                                    className="flex w-full items-center justify-between rounded-xl border border-gray-200 bg-gray-50 px-4 py-3 text-left transition-colors hover:border-gray-300 focus:border-gray-400 focus:outline-none"
                                >
                                    <span className="font-mono text-sm text-gray-900">{draftShortcut || '—'}</span>
                                    <span className="text-xs text-gray-400">
                                        {isRecordingShortcut ? t('quickCaptureShortcutRecording') : t('quickCaptureShortcutHint')}
                                    </span>
                                </button>
                            </div>
                        </div>
                    </section>

                    {/* WeChat */}
                    <section className="rounded-2xl border border-gray-100 bg-white p-6">
                        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                            <MessageSquare size={16} />
                            {t('settingsWeChatTitle')}
                        </div>
                        <div className="space-y-4">
                            <label className="flex items-center gap-3 text-sm text-gray-700">
                                <input
                                    type="checkbox"
                                    checked={draftWechat.enabled}
                                    onChange={e => updateWechatField('enabled', e.target.checked)}
                                    className="h-4 w-4 rounded border-gray-300"
                                />
                                {t('settingsWeChatEnabled')}
                            </label>
                            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                                <InputField label={t('settingsWeChatToken')} value={draftWechat.token} onChange={v => updateWechatField('token', v)} type="password" />
                            </div>
                            <label className="flex items-center gap-3 text-sm text-gray-700">
                                <input
                                    type="checkbox"
                                    checked={draftWechat.autoConnect}
                                    onChange={e => updateWechatField('autoConnect', e.target.checked)}
                                    className="h-4 w-4 rounded border-gray-300"
                                />
                                {t('settingsWeChatAutoConnect')}
                            </label>
                            <div className="grid grid-cols-2 gap-4">
                                <NumberField label={t('settingsWeChatPollTimeout')} value={draftWechat.pollTimeoutSeconds} onChange={v => updateWechatField('pollTimeoutSeconds', v)} />
                                <NumberField label={t('settingsWeChatReconnectInterval')} value={draftWechat.reconnectIntervalSec} onChange={v => updateWechatField('reconnectIntervalSec', v)} />
                            </div>
                        </div>
                    </section>

                    {/* Advanced */}
                    <section className="rounded-2xl border border-gray-100 bg-white p-6">
                        <div className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
                            <Wrench size={16} />
                            {t('settingsAdvancedTitle')}
                        </div>
                        <div className="space-y-4">
                            <label className="flex items-center gap-3 text-sm text-gray-700">
                                <input
                                    type="checkbox"
                                    checked={draftIndexRebuild}
                                    onChange={e => setDraftIndexRebuild(e.target.checked)}
                                    className="h-4 w-4 rounded border-gray-300"
                                />
                                {t('settingsIndexRebuild')}
                            </label>
                            <NumberField
                                label={t('settingsTitleMinLength')}
                                value={draftTitleMinLength}
                                onChange={setDraftTitleMinLength}
                            />
                        </div>
                    </section>
                </div>

                {/* Save bar */}
                <div className="sticky bottom-0 mt-8 flex items-center justify-end gap-3 rounded-2xl border border-gray-100 bg-white/80 px-6 py-4 backdrop-blur-sm">
                    <button
                        onClick={handleSave}
                        disabled={isSaving || !isDirty}
                        className="rounded-lg bg-gray-900 px-5 py-2 text-sm text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
                    >
                        {isSaving ? t('savingButton') : t('saveButton')}
                    </button>
                </div>
            </div>

            {/* Toast */}
            {toast && (
                <div className={`fixed bottom-6 left-1/2 z-50 -translate-x-1/2 rounded-lg px-4 py-2 text-sm text-white shadow-lg transition-all ${toast.type === 'success' ? 'bg-green-600' : 'bg-red-600'}`}>
                    {toast.message}
                </div>
            )}
        </div>
    );
}

function InputField({ label, value, onChange, type = 'text', placeholder }: {
    label: string; value: string; onChange: (v: string) => void; type?: string; placeholder?: string;
}) {
    return (
        <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">{label}</label>
            <input
                type={type}
                value={value}
                onChange={e => onChange(e.target.value)}
                placeholder={placeholder}
                className="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-800 focus:border-gray-400 focus:outline-none"
            />
        </div>
    );
}

function NumberField({ label, value, onChange }: {
    label: string; value: number; onChange: (v: number) => void;
}) {
    return (
        <div>
            <label className="mb-1 block text-sm font-medium text-gray-700">{label}</label>
            <input
                type="number"
                value={value}
                onChange={e => onChange(Number(e.target.value) || 0)}
                className="w-full rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-800 focus:border-gray-400 focus:outline-none"
            />
        </div>
    );
}
