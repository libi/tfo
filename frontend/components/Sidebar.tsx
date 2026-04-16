import React from 'react';
import Link from 'next/link';
import { Calendar as CalendarIcon, Hash, Settings, Smartphone } from 'lucide-react';
import { format, startOfMonth, endOfMonth, eachDayOfInterval, isSameDay } from 'date-fns';
import { useI18n } from './I18nProvider';

interface SidebarProps {
  tags: string[];
  selectedTag: string | null;
  onSelectTag: (tag: string | null) => void;
  selectedDate: Date | null;
  onSelectDate: (date: Date | null) => void;
  onOpenClawBot: () => void;
}

export function Sidebar({ tags, selectedTag, onSelectTag, selectedDate, onSelectDate, onOpenClawBot }: SidebarProps) {
  const { t, dateLocale, weekdayLabels } = useI18n();
  // Simple calendar for current month
  const today = new Date();
  const monthStart = startOfMonth(today);
  const monthEnd = endOfMonth(today);
  const days = eachDayOfInterval({ start: monthStart, end: monthEnd });

  return (
    <aside className="w-64 border-r border-gray-200 bg-[#f7f7f7] flex flex-col h-full shrink-0">
      <div className="p-6">
        <div className="mb-3 flex items-baseline gap-2.5">
          <h1 className="text-[22px] font-semibold tracking-[-0.03em] text-gray-900">TFO</h1>
          <p className="border-l border-gray-200 pl-2.5 text-[10px] font-semibold uppercase tracking-[0.22em] text-gray-400">
            {t('brandFullName')}
          </p>
        </div>
        <p className="max-w-[13rem] text-xs leading-5 text-gray-500">{t('appDescription')}</p>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-2 space-y-8">
        {/* Calendar Section */}
        <div>
          <div className="flex items-center justify-between mb-3 px-2">
            <h2 className="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-1.5">
              <CalendarIcon size={14} />
              {format(today, 'MMMM yyyy', { locale: dateLocale })}
            </h2>
            {selectedDate && (
              <button
                onClick={() => onSelectDate(null)}
                className="text-[10px] text-gray-400 hover:text-gray-700"
              >
                {t('calendarClear')}
              </button>
            )}
          </div>
          <div className="grid grid-cols-7 gap-1 px-2">
            {weekdayLabels.map((d, i) => (
              <div key={`${d}-${i}`} className="text-center text-[10px] text-gray-400 py-1">{d}</div>
            ))}
            {/* Padding for first day */}
            {Array.from({ length: monthStart.getDay() }).map((_, i) => (
              <div key={`pad-${i}`} />
            ))}
            {days.map(day => {
              const isSelected = selectedDate && isSameDay(day, selectedDate);
              const isToday = isSameDay(day, today);
              return (
                <button
                  key={day.toISOString()}
                  onClick={() => onSelectDate(isSelected ? null : day)}
                  className={`
                    aspect-square rounded-sm text-xs flex items-center justify-center transition-colors
                    ${isSelected ? 'bg-gray-800 text-white' : 'hover:bg-gray-200 text-gray-600'}
                    ${isToday && !isSelected ? 'font-bold text-gray-900' : ''}
                  `}
                >
                  {format(day, 'd')}
                </button>
              );
            })}
          </div>
        </div>

        {/* Tags Section */}
        <div>
          <div className="flex items-center justify-between mb-3 px-2">
            <h2 className="text-xs font-semibold text-gray-400 uppercase tracking-wider flex items-center gap-1.5">
              <Hash size={14} />
              {t('tagsTitle')}
            </h2>
            {selectedTag && (
              <button
                onClick={() => onSelectTag(null)}
                className="text-[10px] text-gray-400 hover:text-gray-700"
              >
                {t('calendarClear')}
              </button>
            )}
          </div>
          <div className="flex flex-wrap gap-1.5 px-2">
            {tags.map(tag => (
              <button
                key={tag}
                onClick={() => onSelectTag(selectedTag === tag ? null : tag)}
                className={`
                  px-2 py-1 rounded-md text-xs transition-colors
                  ${selectedTag === tag
                    ? 'bg-gray-800 text-white'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200'}
                `}
              >
                #{tag}
              </button>
            ))}
            {tags.length === 0 && (
              <span className="text-xs text-gray-400 italic">{t('tagsEmpty')}</span>
            )}
          </div>
        </div>
      </div>

      <div className="p-4 border-t border-gray-200 space-y-2">
        <button
          onClick={onOpenClawBot}
          className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-200 rounded-md transition-colors"
        >
          <Smartphone size={16} />
          {t('wechatSync')}
        </button>
        <Link href="/settings" className="w-full flex items-center gap-2 px-3 py-2 text-sm text-gray-700 hover:bg-gray-200 rounded-md transition-colors">
          <Settings size={16} />
          {t('settings')}
        </Link>
      </div>
    </aside>
  );
}
