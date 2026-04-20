'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Sidebar } from '@/components/Sidebar';
import { MainContent } from '@/components/MainContent';
import { ClawBotModal } from '@/components/ClawBotModal';
import type { Fragment } from '@/types';
import * as api from '@/lib/api';
import { format } from 'date-fns';
import { useI18n } from '@/components/I18nProvider';
import { isLocale, type Locale } from '@/lib/i18n';
import { matchesShortcut, normalizeShortcut } from '@/lib/hotkeys';

const defaultQuickCaptureShortcut = 'Alt+S';
const defaultSaveShortcut = 'Ctrl+Enter';
const PAGE_SIZE = 20;

function isEditableTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  return target.isContentEditable || tagName === 'input' || tagName === 'textarea' || tagName === 'select';
}

export default function Home() {
  const { locale, setLocale, t } = useI18n();
  const [mounted, setMounted] = useState(false);
  const [fragments, setFragments] = useState<Fragment[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedTag, setSelectedTag] = useState<string | null>(null);
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);
  const [isClawBotModalOpen, setIsClawBotModalOpen] = useState(false);
  const [allTags, setAllTags] = useState<string[]>([]);
  const [appConfig, setAppConfig] = useState<api.AppConfig | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [hasMore, setHasMore] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [searchTotal, setSearchTotal] = useState(0);

  useEffect(() => {
    setMounted(true);
  }, []);

  const refreshConfig = useCallback(async () => {
    try {
      const config = await api.getConfig();
      setAppConfig(config);
      if (config.uiLanguage && isLocale(config.uiLanguage)) {
        setLocale(config.uiLanguage);
      }
    } catch (err) {
      console.error('Failed to load config:', err);
    }
  }, [setLocale]);

  // Load tags from backend
  const refreshTags = useCallback(() => {
    api.getAllTags().then(tags => {
      if (tags) setAllTags(tags.map(t => t.tag));
    }).catch(console.error);
  }, []);

  useEffect(() => {
    if (mounted) refreshTags();
  }, [mounted, refreshTags]);

  useEffect(() => {
    if (mounted) refreshConfig();
  }, [mounted, refreshConfig]);

  // Load notes
  const loadNotes = useCallback(async () => {
    if (!mounted) return;
    setIsLoading(true);
    try {
      if (selectedDate) {
        const summaries = await api.listNotesByDate(format(selectedDate, 'yyyy-MM-dd'));
        const frags: Fragment[] = (summaries || []).map(s => ({
          id: s.id, title: s.title, content: s.preview, date: s.createdAt, tags: s.tags || [],
        }));
        setFragments(frags);
        setHasMore(false);
      } else {
        const { items, total } = await api.listNotesRecent(0, PAGE_SIZE);
        const frags: Fragment[] = (items || []).map(s => ({
          id: s.id, title: s.title, content: s.preview, date: s.createdAt, tags: s.tags || [],
        }));
        setFragments(frags);
        setHasMore(frags.length < total);
      }
      setSearchTotal(0);
    } catch (err) {
      console.error('Failed to load notes:', err);
    } finally {
      setIsLoading(false);
    }
  }, [mounted, selectedDate]);

  useEffect(() => {
    if (!searchQuery.trim()) loadNotes();
  }, [loadNotes, searchQuery]);

  // Search with debounce
  useEffect(() => {
    if (!mounted || !searchQuery.trim()) return;
    const timer = setTimeout(async () => {
      setIsLoading(true);
      try {
        const { results, total } = await api.searchNotes(searchQuery, PAGE_SIZE, 0);
        setFragments((results || []).map(r => {
          const frag = r.fragments?.[0];
          return {
            id: r.id,
            title: r.title,
            content: frag?.text || '',
            date: '',
            tags: [],
            highlights: frag?.highlights,
          };
        }));
        setSearchTotal(total);
        setHasMore((results || []).length < total);
      } catch (err) {
        console.error('Search failed:', err);
      } finally {
        setIsLoading(false);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery, mounted]);

  // Filter by tag client-side
  const filteredFragments = fragments.filter(f => {
    return selectedTag ? (f.tags || []).includes(selectedTag) : true;
  });

  const handleAddFragment = async (content: string) => {
    try {
      const created = await api.createNote(content);
      setFragments(prev => [{
        id: created.id,
        title: created.title,
        content: created.content,
        date: created.createdAt,
        tags: created.tags || [],
      }, ...prev]);
      refreshTags();
    } catch (err) {
      console.error('Failed to create note:', err);
      throw err;
    }
  };

  const handleDeleteFragment = async (id: string) => {
    try {
      await api.deleteNote(id);
      setFragments(prev => prev.filter(f => f.id !== id));
      refreshTags();
    } catch (err) {
      console.error('Failed to delete note:', err);
      throw err;
    }
  };

  const handleUpdateFragment = async (id: string, content: string) => {
    try {
      const updated = await api.updateNote(id, content);
      setFragments(prev => prev.map(f =>
        f.id === id
          ? { ...f, title: updated.title, content: updated.content, tags: updated.tags || [] }
          : f
      ));
      refreshTags();
    } catch (err) {
      console.error('Failed to update note:', err);
      throw err;
    }
  };

  const loadMore = useCallback(async () => {
    if (isLoadingMore || !hasMore) return;
    setIsLoadingMore(true);
    try {
      const offset = fragments.length;
      if (searchQuery.trim()) {
        const { results, total } = await api.searchNotes(searchQuery, PAGE_SIZE, offset);
        const newFrags = (results || []).map(r => {
          const frag = r.fragments?.[0];
          return {
            id: r.id, title: r.title, content: frag?.text || '', date: '', tags: [] as string[], highlights: frag?.highlights,
          };
        });
        setFragments(prev => [...prev, ...newFrags]);
        setHasMore(offset + newFrags.length < total);
      } else if (!selectedDate) {
        const { items, total } = await api.listNotesRecent(offset, PAGE_SIZE);
        const newFrags = (items || []).map(s => ({
          id: s.id, title: s.title, content: s.preview, date: s.createdAt, tags: s.tags || [],
        }));
        setFragments(prev => [...prev, ...newFrags]);
        setHasMore(offset + newFrags.length < total);
      }
    } finally {
      setIsLoadingMore(false);
    }
  }, [isLoadingMore, hasMore, searchQuery, fragments.length, selectedDate]);

  const quickCaptureShortcut = normalizeShortcut(appConfig?.hotkeyInputFocus || defaultQuickCaptureShortcut);
  const saveShortcut = normalizeShortcut(appConfig?.hotkeySave || defaultSaveShortcut);

  // Global shortcut for Quick Record
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (isEditableTarget(e.target) || isClawBotModalOpen) {
        return;
      }
      if (matchesShortcut(e, quickCaptureShortcut)) {
        e.preventDefault();
        document.getElementById('quick-record-input')?.focus();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isClawBotModalOpen, quickCaptureShortcut]);

  if (!mounted) {
    return <div className="flex h-screen bg-[#fafafa]"></div>;
  }

  return (
    <div className="flex h-screen overflow-hidden bg-[#fafafa] text-gray-800 font-sans">
      <Sidebar
        tags={allTags}
        selectedTag={selectedTag}
        onSelectTag={setSelectedTag}
        selectedDate={selectedDate}
        onSelectDate={setSelectedDate}
        onOpenClawBot={() => setIsClawBotModalOpen(true)}
        wechatBound={!!(appConfig?.wechat?.token)}
      />
      <MainContent
        fragments={filteredFragments}
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        onAddFragment={handleAddFragment}
        onDeleteFragment={handleDeleteFragment}
        onUpdateFragment={handleUpdateFragment}
        quickCaptureShortcut={quickCaptureShortcut}
        saveShortcut={saveShortcut}
        isLoading={isLoading}
        hasMore={hasMore}
        isLoadingMore={isLoadingMore}
        onLoadMore={loadMore}
      />

      {isClawBotModalOpen && (
        <ClawBotModal onClose={() => setIsClawBotModalOpen(false)} />
      )}
    </div>
  );
}
