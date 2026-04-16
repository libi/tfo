'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Sidebar } from '@/components/Sidebar';
import { MainContent } from '@/components/MainContent';
import { ClawBotModal } from '@/components/ClawBotModal';
import type { Fragment } from '@/types';
import * as api from '@/lib/api';
import { format } from 'date-fns';

export default function Home() {
  const [mounted, setMounted] = useState(false);
  const [fragments, setFragments] = useState<Fragment[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedTag, setSelectedTag] = useState<string | null>(null);
  const [selectedDate, setSelectedDate] = useState<Date | null>(null);
  const [isClawBotModalOpen, setIsClawBotModalOpen] = useState(false);
  const [allTags, setAllTags] = useState<string[]>([]);

  useEffect(() => {
    setMounted(true);
  }, []);

  // Load tags from backend
  const refreshTags = useCallback(() => {
    api.getAllTags().then(tags => {
      if (tags) setAllTags(tags.map(t => t.tag));
    }).catch(console.error);
  }, []);

  useEffect(() => {
    if (mounted) refreshTags();
  }, [mounted, refreshTags]);

  // Load notes by month or date
  const loadNotes = useCallback(async () => {
    if (!mounted) return;
    try {
      let summaries: api.NoteSummary[];
      if (selectedDate) {
        summaries = await api.listNotesByDate(format(selectedDate, 'yyyy-MM-dd'));
      } else {
        summaries = await api.listNotesByMonth(format(new Date(), 'yyyy-MM'));
      }
      const frags: Fragment[] = (summaries || []).map(s => ({
        id: s.id,
        title: s.title,
        content: s.preview,
        date: s.createdAt,
        tags: s.tags || [],
      }));
      setFragments(frags);
    } catch (err) {
      console.error('Failed to load notes:', err);
    }
  }, [mounted, selectedDate]);

  useEffect(() => {
    if (!searchQuery.trim()) loadNotes();
  }, [loadNotes, searchQuery]);

  // Search with debounce
  useEffect(() => {
    if (!mounted || !searchQuery.trim()) return;
    const timer = setTimeout(async () => {
      try {
        const { results } = await api.searchNotes(searchQuery);
        setFragments((results || []).map(r => ({
          id: r.id,
          title: r.title,
          content: r.fragments?.join(' ') || '',
          date: '',
          tags: [],
        })));
      } catch (err) {
        console.error('Search failed:', err);
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

  // Global shortcut for Quick Record (Alt+S)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.altKey && e.key.toLowerCase() === 's') {
        e.preventDefault();
        document.getElementById('quick-record-input')?.focus();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

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
      />
      <MainContent
        fragments={filteredFragments}
        searchQuery={searchQuery}
        onSearchChange={setSearchQuery}
        onAddFragment={handleAddFragment}
      />

      {isClawBotModalOpen && (
        <ClawBotModal onClose={() => setIsClawBotModalOpen(false)} />
      )}
    </div>
  );
}
