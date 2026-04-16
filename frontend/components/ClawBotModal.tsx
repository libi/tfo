import React, { useEffect, useRef, useState, useCallback } from 'react';
import { X, Smartphone, CheckCircle2, RefreshCw, Loader2 } from 'lucide-react';
import { useI18n } from './I18nProvider';
import { getWeChatQRCode, pollWeChatQRCode, loginWithQRCode } from '../lib/api';

interface ClawBotModalProps {
  onClose: () => void;
}

type QRState = 'loading' | 'ready' | 'scanned' | 'confirmed' | 'expired' | 'error';

export function ClawBotModal({ onClose }: ClawBotModalProps) {
  const { t } = useI18n();
  const [qrState, setQrState] = useState<QRState>('loading');
  const [qrImgSrc, setQrImgSrc] = useState('');
  const [qrCode, setQrCode] = useState('');
  const [errorMsg, setErrorMsg] = useState('');
  const pollingRef = useRef(false);
  const abortRef = useRef<AbortController | null>(null);

  const fetchQRCode = useCallback(async () => {
    setQrState('loading');
    setErrorMsg('');
    try {
      const data = await getWeChatQRCode();
      setQrCode(data.qrcode);
      setQrImgSrc(data.qrcodeImgContent);
      setQrState('ready');
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : String(err));
      setQrState('error');
    }
  }, []);

  // Start polling when QR is ready
  useEffect(() => {
    if (qrState !== 'ready' && qrState !== 'scanned') return;
    if (pollingRef.current) return;
    pollingRef.current = true;

    const controller = new AbortController();
    abortRef.current = controller;

    const poll = async () => {
      while (!controller.signal.aborted) {
        try {
          const result = await pollWeChatQRCode(qrCode);
          if (controller.signal.aborted) break;

          if (result.status === 'scaned') {
            setQrState('scanned');
          } else if (result.status === 'confirmed') {
            setQrState('confirmed');
            // Auto login with the result
            if (result.botToken && result.baseUrl) {
              await loginWithQRCode(result.botToken, result.botId || '', result.baseUrl);
            }
            pollingRef.current = false;
            return;
          } else if (result.status === 'expired') {
            setQrState('expired');
            pollingRef.current = false;
            return;
          }
          // status === 'wait' → continue polling
        } catch {
          if (controller.signal.aborted) break;
          // Network error during poll, wait and retry
          await new Promise(r => setTimeout(r, 2000));
        }
      }
      pollingRef.current = false;
    };
    poll();

    return () => {
      controller.abort();
      pollingRef.current = false;
    };
  }, [qrState, qrCode]);

  // Fetch QR on mount
  useEffect(() => {
    fetchQRCode();
    return () => {
      abortRef.current?.abort();
    };
  }, [fetchQRCode]);

  const renderQRArea = () => {
    switch (qrState) {
      case 'loading':
        return (
          <div className="w-[212px] h-[212px] flex items-center justify-center bg-gray-50 rounded-xl border border-gray-100">
            <Loader2 size={32} className="text-gray-400 animate-spin" />
            <span className="sr-only">{t('qrLoading')}</span>
          </div>
        );
      case 'error':
        return (
          <button
            onClick={fetchQRCode}
            className="w-[212px] h-[212px] flex flex-col items-center justify-center bg-red-50 rounded-xl border border-red-100 cursor-pointer hover:bg-red-100 transition-colors"
          >
            <RefreshCw size={28} className="text-red-400 mb-2" />
            <span className="text-sm text-red-600">{t('qrError')}</span>
            {errorMsg && <span className="text-xs text-red-400 mt-1 max-w-[180px] truncate">{errorMsg}</span>}
          </button>
        );
      case 'expired':
        return (
          <button
            onClick={fetchQRCode}
            className="w-[212px] h-[212px] flex flex-col items-center justify-center bg-gray-50 rounded-xl border border-gray-200 cursor-pointer hover:bg-gray-100 transition-colors relative"
          >
            {qrImgSrc && <img src={qrImgSrc} alt="QR" className="w-[180px] h-[180px] opacity-20 absolute" />}
            <RefreshCw size={28} className="text-gray-500 mb-2 relative z-10" />
            <span className="text-sm text-gray-600 relative z-10">{t('qrExpired')}</span>
          </button>
        );
      case 'scanned':
        return (
          <div className="w-[212px] h-[212px] flex flex-col items-center justify-center bg-green-50 rounded-xl border border-green-100">
            {qrImgSrc && <img src={qrImgSrc} alt="QR" className="w-[180px] h-[180px] opacity-30 absolute" />}
            <Loader2 size={28} className="text-green-500 animate-spin mb-2 relative z-10" />
            <span className="text-sm text-green-700 relative z-10">{t('qrScanned')}</span>
          </div>
        );
      case 'confirmed':
        return (
          <div className="w-[212px] h-[212px] flex flex-col items-center justify-center bg-green-50 rounded-xl border border-green-200">
            <CheckCircle2 size={48} className="text-green-500 mb-2" />
            <span className="text-sm font-medium text-green-700">{t('qrSuccess')}</span>
          </div>
        );
      case 'ready':
      default:
        return (
          <div className="bg-white p-4 rounded-xl border border-gray-100 shadow-sm inline-block">
            {qrImgSrc ? (
              <img src={qrImgSrc} alt="WeChat QR Code" width={180} height={180} />
            ) : (
              <div className="w-[180px] h-[180px] flex items-center justify-center text-gray-400 text-sm">
                QR Code
              </div>
            )}
          </div>
        );
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/20 backdrop-blur-sm">
      <div className="bg-white rounded-2xl shadow-xl w-full max-w-md overflow-hidden relative animate-in fade-in zoom-in-95 duration-200">
        <button
          onClick={onClose}
          className="absolute top-4 right-4 text-gray-400 hover:text-gray-600 transition-colors"
        >
          <X size={20} />
        </button>

        <div className="p-8 text-center">
          <div className="w-12 h-12 bg-green-50 text-green-600 rounded-full flex items-center justify-center mx-auto mb-4">
            <Smartphone size={24} />
          </div>

          <h2 className="text-xl font-semibold text-gray-900 mb-2">{t('connectWeChat')}</h2>
          <p className="text-sm text-gray-500 mb-8">
            {t('wechatIntro')}
          </p>

          <div className="flex justify-center mb-8">
            {renderQRArea()}
          </div>

          <div className="space-y-3 text-left bg-gray-50 p-4 rounded-lg">
            <h3 className="text-xs font-semibold text-gray-900 uppercase tracking-wider">{t('howItWorks')}</h3>
            <ul className="text-sm text-gray-600 space-y-2">
              <li className="flex items-start gap-2">
                <CheckCircle2 size={16} className="text-green-500 shrink-0 mt-0.5" />
                <span>{t('wechatStep1')}</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle2 size={16} className="text-green-500 shrink-0 mt-0.5" />
                <span>{t('wechatStep2')}</span>
              </li>
              <li className="flex items-start gap-2">
                <CheckCircle2 size={16} className="text-green-500 shrink-0 mt-0.5" />
                <span>{t('wechatStep3')}</span>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
}
