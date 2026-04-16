import React from 'react';
import { format } from 'date-fns';
import ReactMarkdown from 'react-markdown';
import type { Fragment } from '@/types';

interface FragmentCardProps {
  fragment: Fragment;
}

export function FragmentCard({ fragment }: FragmentCardProps) {
  return (
    <article className="group relative bg-white border border-gray-100 rounded-xl p-5 hover:shadow-sm transition-all duration-200">
      <div className="flex items-baseline justify-between mb-3">
        <h3 className="text-sm font-medium text-gray-900">{fragment.title}</h3>
        <time className="text-[11px] text-gray-400 font-mono">
          {format(new Date(fragment.date), 'MMM d, HH:mm')}
        </time>
      </div>
      
      <div className="prose prose-sm max-w-none text-gray-600 prose-p:leading-relaxed prose-a:text-blue-600 hover:prose-a:text-blue-500">
        <ReactMarkdown>{fragment.content}</ReactMarkdown>
      </div>

      {fragment.tags.length > 0 && (
        <div className="mt-4 flex flex-wrap gap-1.5">
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
