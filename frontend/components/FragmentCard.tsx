import React, { useState } from 'react';
import { format } from 'date-fns';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { ChevronDown, ChevronUp } from 'lucide-react';
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

interface FragmentCardProps {
  fragment: Fragment;
  titleMinContentLength?: number;
}

export function FragmentCard({ fragment, titleMinContentLength }: FragmentCardProps) {
  const { t, dateLocale } = useI18n();
  const threshold = titleMinContentLength ?? DEFAULT_TITLE_MIN_CONTENT_LENGTH;
  const showTitle = fragment.content.length >= threshold;

  const [expanded, setExpanded] = useState(false);
  const [fullContent, setFullContent] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  // Detect if content is likely truncated (ends with "…" or is a search snippet)
  const isTruncated = fragment.content.endsWith('…') || fragment.content.endsWith('...');

  const handleToggleExpand = async () => {
    if (expanded) {
      setExpanded(false);
      return;
    }
    if (fullContent !== null) {
      setExpanded(true);
      return;
    }
    setLoading(true);
    try {
      const note = await getNote(fragment.id);
      setFullContent(note.content);
      setExpanded(true);
    } catch (err) {
      console.error('Failed to load full content:', err);
    } finally {
      setLoading(false);
    }
  };

  const displayContent = expanded && fullContent !== null ? fullContent : fragment.content;

  return (
    <article className={`group relative bg-white border border-gray-100 rounded-xl hover:shadow-sm transition-all duration-200 ${showTitle ? 'p-5' : 'px-5 py-3'}`}>
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

      {isTruncated && (
        <button
          onClick={handleToggleExpand}
          disabled={loading}
          className="mt-2 flex items-center gap-1 text-xs text-gray-400 hover:text-gray-600 transition-colors"
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
        <div className={`flex flex-wrap gap-1.5 ${showTitle ? 'mt-4' : 'mt-2'}`}>
          {fragment.tags.map(tag => (
            <span key={tag} className="text-[10px] px-1.5 py-0.5 bg-gray-50 text-gray-500 rounded border border-gray-100">
              #{tag}
            </span>
          ))}
        </div>
      )}

      {/* Hidden actions that appear on hover */}
      <div className="absolute top-4 right-4 opacity-0 group-hover:opacity-100 transition-opacity flex gap-2">
        {/* Future actions: Edit, Delete, Copy */}
      </div>
    </article>
  );
}
