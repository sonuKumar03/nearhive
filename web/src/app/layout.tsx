import type { Metadata } from 'next';
import { Inter, Geist_Mono } from 'next/font/google';
import './globals.css';
import Providers from './providers';

const inter = Inter({
  subsets: ['latin'],
  variable: '--font-sans',
  display: 'swap',
});

const geistMono = Geist_Mono({
  subsets: ['latin'],
  variable: '--font-mono',
  display: 'swap',
});

export const metadata: Metadata = {
  title: 'NearHive — Scalable Tech Company Locator & Verification Engine',
  description: 'PostGIS spatial radius search, automated Overpass & Wikidata scraping, and AI verification engine.',
  icons: {
    icon: [
      { url: '/favicon.ico', sizes: 'any' },
      { url: '/icon.svg', type: 'image/svg+xml' },
      { url: '/icon.png', type: 'image/png' },
    ],
    shortcut: '/favicon.ico',
    apple: '/apple-touch-icon.png',
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className={`dark h-full ${inter.variable} ${geistMono.variable}`}>
      <body className="h-full bg-[#09090b] text-zinc-100 antialiased font-sans overflow-hidden">
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
