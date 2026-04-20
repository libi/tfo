import React, { useState, useRef, useEffect } from 'react';
import { format } from 'date-fns';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { ChevronDown, ChevronUp, Copy, Pencil, Trash2, Check } from 'lucide-react';
import copy from 'clipboard-copy';
import type { Fragment } from '@/types';
import { useI18n } from './I18nProvider';
import { getNote } from '@/lib/api';

const DEFAULT_TITLE_MIN_CONTENT_LENGTH = 300;

function HighlightedText({ text, highlights }: { text: string; highlights: { start: number; end: number }[] }) {
  if (!highlights || highlights.length === 0) {
    return <>{text}</>;
  }
  const sorted = [...highlights].sort((a, b) => a.start - b.start);
  const parts: React.ReactNode[] = [];
  let lastEnd = 0;
  for (const h of sorted) {
    if (h.start > lastEnd) {
      parts.push(<React.Fragment key={`t${lastEnd}`}>{text.slice(lastEnd, h.start)}</React.Fragment>);
    }
    parts.push(
      <mark key={`h${h.start}`} className="bg-yellow-200 text-inherit rounded-sm px-0.5">
        {text.slice(h.start, h.end)}
      </mark>
    );
    lastEnd = h.end;
  }
  if (lastEnd < text.length) {
    parts.push(<React.Fragment key={`t${lastEnd}`}>{text.slice(lastEnd)}</React.Fragment>);
  }
  return <>{parts}</>;
}

interface DeleteDialogProps {
  filename: string;
  onConfirm: () => void;
  onCancel: () => void;
  isDeleting: boolean;
}

function DeleteDialog({ filename, onConfirm, onCancel, isDeleting }: DeleteDialogProps) {
  const { t } = useI18n();
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 backdrop-blur-sm" onClick={onCancel}>
      <div
        className="bg-white rounded-2xl shadow-xl p-6 max-w-sm w-full mx-4 space-y-4"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center gap-3">
          <div className="w-9 h-9 rounded-full bg-red-50 flex items-center justify-center shrink-0">
            <Trash2 size={16} className="text-red-500" />
          </div>
          <h2 className="text-sm font-semibold text-gray-900">{t('deleteConfirmTitle')}</h2>
        </div>
        <p className="text-sm text-gray-500 leading-relaxed">
          {t('deleteConfirmMessage').replace('{filename}', filename)}
        </p>
        <div className="flex justify-end gap-2 pt-1">
          <button
            onClick={onCancel}
            disabled={isDeleting}
            className="px-4 py-1.5 text-sm rounded-lg border border-gray-200 text-gray-600 hover:bg-gray-50 disabled:opacity-50 transition-colors"
          >
            {t('deleteConfirmCancel')}
          </button>
          <button
            onClick={onConfirm}
            disabled={isDeleting}
            className="px-4 py-1.5 text-sm rounded-lg bg-red-500 text-white hover:bg-red-600 disabled:opacity-50 transition-colors"
          >
            {isDeleting ? t('deletingContent') : t('deleteConfirmButton')}
          </button>
        </div>
      </div>
    </div>
  );
}

interface FragmentCardProps {
  fragment: Fragment;
  titleMinContentLength?: number;
  onDelete?: (id: string) => Promise<void>;
  onUpdate?: (id: string, content: string) => Promise<void>;
}

export function FragmentCard({ fragment, titleMinContentLength, onDelete, onUpdate }: FragmentCardProps) {
  const { t, dateLocale } = useI18n();
  const threshold = titleMinContentLength ?? DEFAULT_TITLE_MIN_CONTENT_LENGTH;
  const showTitle = fragment.content.length >= threshold;

  const [expanded, setExpanded] = useState(false);
  const [fullContent, setFullContent] = useState<string | null>(null);
  const [filePath, setFilePath] = useState<string | null>(fragment.filePath ?? null);
  const [loading, setLoading] = useState(false);

  // Edit state
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState('');
  const [saving, setSaving] = useState(false);

  // Delete state
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [loadingMeta, setLoadingMeta] = useState(false);

  // Copy state
  const [copied, setCopied] = useState(false);

  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Detect if content is likely truncated (ends with "…" or is a search snippet)
  const isTruncated = fragment.content.endsWith('…') || fragment.content.endsWith('...');

  const ensureFullNote = async () => {
    if (fullContent !== null) return { content: fullContent, filePath };
    setLoading(true);
    try {
      const note = await getNote(fragment.id);
      setFullContent(note.content);
      if (note.filePath) setFilePath(note.filePath);
      return { content: note.content, filePath: note.filePath ?? null };
    } finally {
      setLoading(false);
    }
  };

  const handleToggleExpand = async () => {
    if (expanded) {
      setExpanded(false);
      return;
    }
    try {
      await ensureFullNote();
      setExpanded(true);
    } catch (err) {
      console.error('Failed to load full content:', err);
    }
  };

  const displayContent = expanded && fullContent !== null ? fullContent : fragment.content;

  // --- Copy ---
  const handleCopy = async () => {
    let contentToCopy = fullContent ?? fragment.content;
    if (isTruncated && fullContent === null) {
      try {
        const result = await ensureFullNote();
        contentToCopy = result.content ?? fragment.content;
      } catch {
        contentToCopy = fragment.content;
      }
    }
    await copy(contentToCopy);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  // --- Edit ---
  const handleEdit = async () => {
    let content = fullContent;
    if (content === null) {
      try {
        const result = await ensureFullNote();
        content = result.content;
      } catch (err) {
        console.error('Failed to load full content for edit:', err);
        return;
      }
    }
    setEditValue(content ?? fragment.content);
    setEditing(true);
  };

  useEffect(() => {
    if (editing && textareaRef.current) {
      textareaRef.current.focus();
      textareaRef.current.setSelectionRange(textareaRef.current.value.length, textareaRef.current.value.length);
    }
  }, [editing]);

  const handleSaveEdit = async () => {
    if (!editValue.trim() || saving || !onUpdate) return;
    setSaving(true);
    try {
      await onUpdate(fragment.id, editValue);
      setFullContent(editValue);
      setEditing(false);
    } catch (err) {
      console.error('Failed to save edit:', err);
    } finally {
      setSaving(false);
    }
  };

  const handleCancelEdit = () => {
    setEditing(false);
  };

  // --- Delete ---
  const handleDeleteClick = async () => {
    if (filePath === null) {
      setLoadingMeta(true);
      try {
        const note = await getNote(fragment.id);
        if (note.filePath) setFilePath(note.filePath);
      } catch (err) {
        console.error('Failed to load note metadata:', err);
      } finally {
        setLoadingMeta(false);
      }
    }
    setShowDeleteDialog(true);
  };

  const handleConfirmDelete = async () => {
    if (!onDelete) return;
    setDeleting(true);
    try {
      await onDelete(fragment.id);
    } catch (err) {
      console.error('Failed to delete:', err);
      setDeleting(false);
      setShowDeleteDialog(false);
    }
  };

  const displayFilename = filePath ? filePath.split('/').pop() ?? filePath : `${fragment.id}.md`;

  const actionButtons = (
    <div className="flex items-center gap-0.5">
      <button
        onClick={handleCopy}
        title={t('copyAction')}
        className="p-1.5 rounded-md text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors"
      >
        {copied ? <Check size={13} className="text-green-500" /> : <Copy size={13} />}
      </button>
      <button
        onClick={handleEdit}
        disabled={loading}
        title={t('editAction')}
        className="p-1.5 rounded-md text-gray-400 hover:text-gray-700 hover:bg-gray-100 transition-colors disabled:opacity-40"
      >
        <Pencil size={13} />
      </button>
      <button
        onClick={handleDeleteClick}
        disabled={loadingMeta}
        title={t('deleteAction')}
        className="p-1.5 rounded-md text-gray-400 hover:text-red-500 hover:bg-red-50 transition-colors disabled:opacity-40"
      >
        <Trash2 size={13} />
      </button>
    </div>
  );

  return (
    <>
      <article className={`group relative bg-white border border-gray-100 rounded-xl hover:shadow-sm transition-all duration-200 ${showTitle ? 'p-5' : 'px-5 py-3'}`}>
        {editing ? (
          /* ── Edit mode ── */
          <div className="space-y-3">
            <textarea
              ref={textareaRef}
              value={editValue}
              onChange={e => setEditValue(e.target.value)}
              className="w-full min-h-[140px] resize-y border border-gray-200 rounded-lg p-3 text-sm text-gray-800 focus:outline-none focus:ring-1 focus:ring-gray-400 transition-shadow"
            />
            <div className="flex items-center justify-end gap-2">
              <button
                onClick={handleCancelEdit}
                disabled={saving}
                className="px-3 py-1.5 text-sm rounded-lg border border-gray-200 text-gray-600 hover:bg-gray-50 disabled:opacity-50 transition-colors"
              >
                {t('editCancel')}
              </button>
              <button
                onClick={handleSaveEdit}
                disabled={saving || !editValue.trim()}
                className="px-3 py-1.5 text-sm rounded-lg bg-gray-900 text-white hover:bg-gray-800 disabled:opacity-50 transition-colors"
              >
                {saving ? t('savingButton') : t('editSave')}
              </button>
            </div>
          </div>
        ) : (
          /* ── Normal view mode ── */
          <>
            {showTitle ? (
              <>
                <div className="flex items-baseline justify-between mb-3">
                  <h3 className="text-sm font-medium text-gray-900">{fragment.title || t('untitledFragment')}</h3>
                  <time className="text-[11px] text-gray-400 font-mono">
                    {fragment.date ? format(new Date(fragment.date), 'MMM d, HH:mm', { locale: dateLocale }) : ''}
                  </time>
                </div>
                <div className="prose prose-sm max-w-none text-gray-600 prose-p:leading-relaxed prose-a:text-blue-600 hover:prose-a:text-blue-500">
                  {fragment.highlights && fragment.highlights.length > 0 && !expanded
                    ? <p className="leading-relaxed"><HighlightedText text={displayContent} highlights={fragment.highlights} /></p>
                    : <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ a: ({ children, ...props }) => <a {...props} target="_blank" rel="noopener noreferrer">{children}</a> }}>{displayContent}</ReactMarkdown>}
                </div>
              </>
            ) : (
              <div className="flex items-start gap-3">
                <div className="flex-1 prose prose-sm max-w-none text-gray-700 prose-p:my-1 prose-p:leading-relaxed prose-a:text-blue-600 hover:prose-a:text-blue-500">
                  {fragment.highlights && fragment.highlights.length > 0 && !expanded
                    ? <p className="my-1 leading-relaxed"><HighlightedText text={displayContent} highlights={fragment.highlights} /></p>
                    : <ReactMarkdown remarkPlugins={[remarkGfm]} components={{ a: ({ children, ...props }) => <a {...props} target="_blank" rel="noopener noreferrer">{children}</a> }}>{displayContent}</ReactMarkdown>}
                </div>
                <time className="shrink-0 text-[11px] text-gray-400 font-mono pt-1">
                  {fragment.date ? format(new Date(fragment.date), 'MMM d, HH:mm', { locale: dateLocale }) : ''}
                </time>
              </div>
            )}

            {/* Bottom row: expand button + tags + actions */}
            <div className={`${(isTruncated || fragment.tags.length > 0) ? 'mt-3' : 'mt-1'} flex items-end justify-between gap-2`}>
              <div className="flex-1 space-y-2">
                {isTruncated && (
                  <button
                    onClick={handleToggleExpand}
                    disabled={loading}
                    className="flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 transition-colors"
                  >
                    {loading ? (
                      t('loadingContent')
                    ) : expanded ? (
                      <><ChevronUp size={14} />{t('collapseContent')}</>
                    ) : (
                      <><ChevronDown size={14} />{t('expandContent')}</>
                    )}
                  </button>
                )}
                {fragment.tags.length > 0 && (
                  <div className="flex flex-wrap gap-1.5">
                    {fragment.tags.map(tag => (
                      <span key={tag} className="text-[10px] px-1.5 py-0.5 bg-gray-50 text-gray-500 rounded border border-gray-100">
                        #{tag}
                      </span>
                    ))}
                  </div>
                )}
              </div>

              {/* Action buttons – visible on hover */}
              <div className="shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                {actionButtons}
              </div>
            </div>
          </>
        )}
      </article>

      {showDeleteDialog && (
        <DeleteDialog
          filename={displayFilename}
          onConfirm={handleConfirmDelete}
          onCancel={() => setShowDeleteDialog(false)}
          isDeleting={deleting}
        />
      )}
    </>
  );
}
