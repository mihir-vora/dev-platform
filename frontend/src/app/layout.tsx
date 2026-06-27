import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Dev Platform",
  description: "Developer platform for project builds and deployments",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body className="antialiased">{children}</body>
    </html>
  );
}
