import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "clo-ui/styles/default.scss";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "maintainer-d",
  description: "Maintainer-d web console",
  themeColor: "#2a0552",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" data-theme="light">
      <body className={`${geistSans.variable} ${geistMono.variable}`}>
        <div id="clo-wrapper">{children}</div>
      </body>
    </html>
  );
}
