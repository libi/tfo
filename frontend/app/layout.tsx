import type { Metadata } from 'next';
import { Inter, JetBrains_Mono } from 'next/font/google';
import { I18nProvider } from '@/components/I18nProvider';
import './globals.css'; // Global styles

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-sans',
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ['latin'],
  variable: '--font-mono',
});

export const metadata: Metadata = {
  title: 'TFO · The Flash Note',
  description: 'TFO stands for The Flash Note, a lightweight private notebook for capturing fleeting thoughts.',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`${inter.variable} ${jetbrainsMono.variable}`}>
      <body className="font-sans bg-[#fafafa] text-gray-900 antialiased selection:bg-gray-200 selection:text-gray-900" suppressHydrationWarning>
        <I18nProvider>{children}</I18nProvider>
      </body>
    </html>
  );
}
