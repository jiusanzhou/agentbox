import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function Home() {
  return (
    <div className="flex min-h-screen flex-col bg-background">
      {/* Nav */}
      <header className="sticky top-0 z-50 border-b border-border/50 bg-background/80 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-6">
          <div className="flex items-center gap-2.5">
            <TriangleLogo className="h-5 w-5" />
            <span className="text-lg font-semibold tracking-tight">ABox</span>
          </div>
          <div className="flex items-center gap-4">
            <a
              href="https://github.com/abox"
              target="_blank"
              rel="noopener noreferrer"
              className="text-muted-foreground hover:text-foreground transition-colors"
            >
              <GitHubIcon className="h-5 w-5" />
            </a>
            <Link href="/login">
              <Button variant="ghost" size="sm" className="text-sm">
                Log in
              </Button>
            </Link>
            <Link href="/register">
              <Button size="sm" className="text-sm px-4">
                Get Started
              </Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Hero */}
      <section className="flex flex-col items-center px-6 pt-24 pb-20 text-center">
        <h1 className="animate-fade-in max-w-3xl text-5xl font-bold tracking-tighter sm:text-6xl lg:text-7xl">
          Run AI Agents
          <br />
          in Seconds
        </h1>
        <p className="animate-fade-in animate-delay-100 mt-6 max-w-lg text-lg text-muted-foreground leading-relaxed">
          Deploy, manage, and orchestrate AI agents in secure sandboxed
          environments. One platform, any runtime.
        </p>
        <div className="animate-fade-in animate-delay-200 mt-10 flex gap-4">
          <Link href="/register">
            <Button size="lg" className="h-12 px-8 text-base">
              Get Started
            </Button>
          </Link>
          <a
            href="https://github.com/abox"
            target="_blank"
            rel="noopener noreferrer"
          >
            <Button variant="outline" size="lg" className="h-12 px-8 text-base gap-2">
              <GitHubIcon className="h-4 w-4" />
              View on GitHub
            </Button>
          </a>
        </div>

        {/* Terminal mockup */}
        <div className="animate-fade-in-up animate-delay-300 mt-16 w-full max-w-2xl">
          <div className="rounded-xl border border-border bg-card overflow-hidden">
            <div className="flex items-center gap-2 border-b border-border px-4 py-3">
              <div className="flex gap-1.5">
                <div className="h-3 w-3 rounded-full bg-border" />
                <div className="h-3 w-3 rounded-full bg-border" />
                <div className="h-3 w-3 rounded-full bg-border" />
              </div>
              <span className="ml-2 text-xs text-muted-foreground font-mono">
                Terminal
              </span>
            </div>
            <div className="p-6 font-mono text-sm text-left">
              <div className="flex gap-2">
                <span className="text-muted-foreground select-none">$</span>
                <span>
                  aboxctl agent run{" "}
                  <span className="text-muted-foreground">cro-optimizer</span>
                  {" "}--runtime{" "}
                  <span className="text-muted-foreground">claude</span>
                </span>
              </div>
              <div className="mt-3 text-muted-foreground text-xs leading-relaxed">
                <p>Creating sandbox... done</p>
                <p>Pulling agent manifest... done</p>
                <p>Runtime: claude-sonnet-4-6</p>
                <p className="text-emerald-500">Agent running on port 8080</p>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Features grid */}
      <section className="border-t border-border">
        <div className="mx-auto max-w-6xl px-6 py-24">
          <div className="grid gap-px sm:grid-cols-3 bg-border rounded-xl overflow-hidden border border-border">
            <FeatureCard
              icon={<RuntimeIcon />}
              title="11 Runtimes"
              description="Claude, Codex, Gemini, GPT, Llama, DeepSeek, and more. Run any model with a single config change."
            />
            <FeatureCard
              icon={<ChannelIcon />}
              title="Any Channel"
              description="Connect agents to Telegram, Discord, Slack, WeChat, or the built-in web chat interface."
            />
            <FeatureCard
              icon={<ShieldIcon />}
              title="Secure Sandbox"
              description="Every agent runs in isolated containers. Docker, Kubernetes, or Firecracker — your choice."
            />
          </div>
        </div>
      </section>

      {/* How it works */}
      <section className="border-t border-border">
        <div className="mx-auto max-w-6xl px-6 py-24">
          <h2 className="text-center text-3xl font-bold tracking-tight">
            How it works
          </h2>
          <p className="mt-3 text-center text-muted-foreground">
            Three steps to deploy your first agent
          </p>
          <div className="mt-16 grid gap-12 sm:grid-cols-3">
            <StepCard
              number="01"
              title="Choose an Agent"
              description="Browse the marketplace or bring your own. Agents are portable OpenAgent manifests."
            />
            <StepCard
              number="02"
              title="Pick a Runtime"
              description="Select from 11 supported runtimes. Each runtime is optimized for different workloads."
            />
            <StepCard
              number="03"
              title="Start Building"
              description="Deploy in seconds. Chat, automate workflows, and scale with built-in monitoring."
            />
          </div>
        </div>
      </section>

      {/* Stats */}
      <section className="border-t border-border">
        <div className="mx-auto max-w-6xl px-6 py-16">
          <div className="flex flex-wrap items-center justify-center gap-x-16 gap-y-4 text-center">
            <StatItem value="60+" label="Agents" />
            <StatDot />
            <StatItem value="11" label="Runtimes" />
            <StatDot />
            <StatItem value="6" label="IM Channels" />
            <StatDot />
            <StatItem value="100%" label="Open Source" />
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t border-border">
        <div className="mx-auto max-w-6xl px-6 py-24 text-center">
          <h2 className="text-3xl font-bold tracking-tight">
            Ready to build with AI agents?
          </h2>
          <p className="mt-3 text-muted-foreground">
            Start deploying agents in minutes. Free to get started.
          </p>
          <div className="mt-8">
            <Link href="/register">
              <Button size="lg" className="h-12 px-8 text-base">
                Get Started Free
              </Button>
            </Link>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-6">
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            <span>Built by Zoe</span>
            <span className="text-border">|</span>
            <span>MIT License</span>
          </div>
          <a
            href="https://github.com/abox"
            target="_blank"
            rel="noopener noreferrer"
            className="text-muted-foreground hover:text-foreground transition-colors"
          >
            <GitHubIcon className="h-4 w-4" />
          </a>
        </div>
      </footer>
    </div>
  );
}

function FeatureCard({
  icon,
  title,
  description,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
}) {
  return (
    <div className="bg-card p-8 transition-colors hover:bg-accent/50">
      <div className="mb-4 text-foreground">{icon}</div>
      <h3 className="text-base font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">
        {description}
      </p>
    </div>
  );
}

function StepCard({
  number,
  title,
  description,
}: {
  number: string;
  title: string;
  description: string;
}) {
  return (
    <div className="text-center">
      <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full border border-border text-sm font-mono text-muted-foreground">
        {number}
      </div>
      <h3 className="mt-4 text-lg font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-muted-foreground leading-relaxed">
        {description}
      </p>
    </div>
  );
}

function StatItem({ value, label }: { value: string; label: string }) {
  return (
    <div>
      <p className="text-2xl font-bold tracking-tight">{value}</p>
      <p className="text-sm text-muted-foreground">{label}</p>
    </div>
  );
}

function StatDot() {
  return (
    <div className="hidden sm:block h-1 w-1 rounded-full bg-border" />
  );
}

function TriangleLogo({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 2L2 22h20L12 2z" />
    </svg>
  );
}

function GitHubIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
    </svg>
  );
}

function RuntimeIcon() {
  return (
    <svg className="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <rect x="2" y="2" width="20" height="20" rx="3" />
      <path d="M7 8h10M7 12h6M7 16h8" />
    </svg>
  );
}

function ChannelIcon() {
  return (
    <svg className="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" />
    </svg>
  );
}

function ShieldIcon() {
  return (
    <svg className="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      <path d="m9 12 2 2 4-4" />
    </svg>
  );
}
