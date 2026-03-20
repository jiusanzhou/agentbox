"use client";

import { Sidebar, TopBar } from "@/components/sidebar";

export default function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-screen">
      <Sidebar />
      {/* Main content area — shifts right based on sidebar width */}
      <main className="md:pl-60 transition-all duration-200">
        <TopBar />
        <div className="mx-auto max-w-6xl px-6 py-8">{children}</div>
      </main>
    </div>
  );
}
