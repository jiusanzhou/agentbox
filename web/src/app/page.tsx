import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function Home() {
  return (
    <div className="flex min-h-screen flex-col bg-background">
      {/* Nav */}
      <header className="border-b border-border">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-6">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-md bg-primary text-primary-foreground text-xs font-bold">
              A
            </div>
            <span className="text-lg font-semibold tracking-tight">ABox</span>
          </div>
          <div className="flex items-center gap-3">
            <Link href="/login">
              <Button variant="ghost" size="sm">Log in</Button>
            </Link>
            <Link href="/register">
              <Button size="sm">Get Started</Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Hero */}
      <main className="flex flex-1 flex-col items-center justify-center px-6 text-center">
        <h1 className="max-w-2xl text-4xl font-bold tracking-tight sm:text-5xl">
          AI Agents, One Click Away
        </h1>
        <p className="mt-4 max-w-lg text-lg text-muted-foreground">
          Run AI agents in sandboxed containers. Chat, automate, and build
          with a single platform.
        </p>
        <div className="mt-8 flex gap-3">
          <Link href="/register">
            <Button size="lg">Get Started Free</Button>
          </Link>
          <Link href="/login">
            <Button variant="outline" size="lg">Log in</Button>
          </Link>
        </div>

        {/* Features */}
        <div className="mt-20 grid w-full max-w-3xl gap-6 sm:grid-cols-3">
          <FeatureCard
            title="Sandboxed Agents"
            description="Every agent runs in an isolated Kubernetes container with full lifecycle management."
          />
          <FeatureCard
            title="Chat Interface"
            description="Interact with agents through a familiar chat UI. Send messages and get real-time responses."
          />
          <FeatureCard
            title="Skills Marketplace"
            description="Browse pre-built agent skills or create your own. One click to deploy."
          />
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-border py-6 text-center text-xs text-muted-foreground">
        ABox — Sandboxed AI Agent Platform
      </footer>
    </div>
  );
}

function FeatureCard({ title, description }: { title: string; description: string }) {
  return (
    <div className="rounded-lg border border-border p-5 text-left">
      <h3 className="text-sm font-semibold">{title}</h3>
      <p className="mt-1 text-sm text-muted-foreground">{description}</p>
    </div>
  );
}
