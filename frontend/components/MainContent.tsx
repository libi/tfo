import React, { useState } from 'react';
import { Search, Send } from 'lucide-react';
import type { Fragment } from '@/types';
import { FragmentCard } from './FragmentCard';

interface MainContentProps {
  fragments: Fragment[];
  searchQuery: string;
  onSearchChange: (query: string) => void;
  onAddFragment: (content: string) => Promise<void>;
}

export function MainContent({ fragments, searchQuery, onSearchChange, onAddFragment }: MainContentProps) {
  const [inputValue, setInputValue] = useState('');
  const [sending, setSending] = useState(false);
  const [toast, setToast] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const showToast = (type: 'success' | 'error', message: string) => {
    setToast({ type, message });
    setTimeout(() => setToast(null), type === 'success' ? 2000 : 4000);
  };

  const doSubmit = async () => {
    if (!inputValue.trim() || sending) return;
    setSending(true);
    try {
      await onAddFragment(inputValue);
      setInputValue('');
      showToast('success', 'Fragment saved successfully!');
    } catch {
      showToast('error', 'Failed to save fragment. Please try again.');
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      doSubmit();
    }
  };

  const handleSubmit = () => {
    doSubmit();
  };

  return (
    <main className="flex-1 flex flex-col h-full bg-white relative">
      {/* Top Bar: Search */}
      <header className="h-16 border-b border-gray-100 flex items-center px-8 shrink-0">
        <div className="relative w-full max-w-2xl">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" size={18} />
          <input
            type="text"
            placeholder="Search fragments (Bleve index)..."
            value={searchQuery}
            onChange={(e) => onSearchChange(e.target.value)}
            className="w-full pl-10 pr-4 py-2 bg-gray-50 border-none rounded-lg text-sm focus:outline-none focus:ring-1 focus:ring-gray-200 transition-shadow"
          />
        </div>
      </header>

      {/* Fragments List */}
      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="max-w-3xl mx-auto space-y-6 pb-32">
          {fragments.length === 0 ? (
            <div className="text-center text-gray-400 mt-20">
              <p>No fragments found.</p>
            </div>
          ) : (
            fragments.map(fragment => (
              <FragmentCard key={fragment.id} fragment={fragment} />
            ))
          )}
        </div>
      </div>

      {/* Quick Record Input (Bottom Floating) */}
      <div className="absolute bottom-0 left-0 right-0 p-6 bg-gradient-to-t from-white via-white to-transparent">
        <div className="max-w-3xl mx-auto relative shadow-lg rounded-xl border border-gray-100 bg-white overflow-hidden focus-within:ring-1 focus-within:ring-gray-300 transition-shadow">
          <textarea
            id="quick-record-input"
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Record a thought... (Alt+S to focus, Cmd+Enter to save)"
            className="w-full p-4 pb-12 resize-none border-none focus:outline-none text-sm text-gray-800 bg-transparent min-h-[100px]"
          />
          <div className="absolute bottom-3 right-3 flex items-center gap-3">
            <span className="text-[10px] text-gray-400">Markdown supported</span>
            <button
              onClick={handleSubmit}
              disabled={!inputValue.trim() || sending}
              className="bg-gray-900 text-white p-1.5 rounded-md hover:bg-gray-800 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              <Send size={16} />
            </button>
          </div>
        </div>
      </div>

      {/* Toast */}
      {toast && (
        <div className={`fixed bottom-6 left-1/2 -translate-x-1/2 z-50 px-4 py-2 rounded-lg shadow-lg text-sm text-white transition-all ${toast.type === 'success' ? 'bg-green-600' : 'bg-red-600'
          }`}>
          {toast.message}
        </div>
      )}
    </main>
  );
}
