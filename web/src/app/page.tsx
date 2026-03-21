import Link from "next/link";
import { Button } from "@/components/ui/button";

export default function Home() {
  return (
    <div className="min-h-screen bg-white dark:bg-black text-foreground overflow-hidden">
      {/* Gradient background orbs */}
      <div className="pointer-events-none fixed inset-0">
        <div className="absolute -top-40 -right-40 h-96 w-96 rounded-full bg-blue-500/10 dark:bg-blue-600/20 blur-[120px]" />
        <div className="absolute top-1/3 -left-40 h-96 w-96 rounded-full bg-purple-500/8 dark:bg-purple-600/15 blur-[120px]" />
        <div className="absolute bottom-0 right-1/4 h-64 w-64 rounded-full bg-cyan-500/5 dark:bg-cyan-600/10 blur-[100px]" />
      </div>

      {/* Grid pattern overlay */}
      <div
        className="pointer-events-none fixed inset-0 opacity-[0.04] dark:opacity-[0.03]"
        style={{
          backgroundImage: `linear-gradient(rgba(0,0,0,.06) 1px, transparent 1px), linear-gradient(90deg, rgba(0,0,0,.06) 1px, transparent 1px)`,
          backgroundSize: "64px 64px",
        }}
      />

      {/* Nav */}
      <header className="sticky top-0 z-50 border-b border-foreground/[0.06] bg-background/80 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-6">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-foreground text-background">
              <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2L2 22h20L12 2z" />
              </svg>
            </div>
            <span className="text-lg font-semibold tracking-tight">ABox</span>
          </div>
          <div className="flex items-center gap-4">
            <a
              href="https://github.com/jiusanzhou/agentbox"
              target="_blank"
              rel="noopener noreferrer"
              className="text-foreground/40 hover:text-foreground transition-colors"
            >
              <svg className="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
              </svg>
            </a>
            <Link href="/login">
              <Button variant="ghost" size="sm" className="text-sm text-white/60 hover:text-foreground hover:bg-foreground/[0.06]">
                Log in
              </Button>
            </Link>
            <Link href="/register">
              <Button size="sm" className="text-sm px-5 bg-foreground text-background hover:bg-foreground/90 border-0">
                Get Started
              </Button>
            </Link>
          </div>
        </div>
      </header>

      {/* Hero */}
      <section className="relative flex flex-col items-center px-6 pt-32 pb-24 text-center">
        {/* Badge */}
        <div className="mb-8 inline-flex items-center gap-2 rounded-full border border-foreground/[0.08] bg-foreground/[0.03] px-4 py-1.5 text-xs text-foreground/50">
          <span className="inline-block h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
          Now supporting 11 AI runtimes
        </div>

        <h1 className="max-w-4xl text-5xl font-bold tracking-[-0.04em] leading-[1.08] sm:text-6xl lg:text-[80px]">
          Serverless for{" "}
          <span className="bg-gradient-to-r from-blue-400 via-cyan-300 to-purple-400 bg-clip-text text-transparent">
            AI Agents
          </span>
        </h1>
        <p className="mt-6 max-w-xl text-lg text-foreground/40 leading-relaxed">
          Deploy agents in sandboxed containers. Chat from Web, Telegram, Discord, Slack. Stream everything. One platform, any runtime.
        </p>
        <div className="mt-10 flex gap-4">
          <Link href="/register">
            <Button size="lg" className="h-12 px-8 text-base bg-foreground text-background hover:bg-foreground/90 border-0 rounded-xl">
              Get Started Free →
            </Button>
          </Link>
          <Link href="/login">
            <Button variant="outline" size="lg" className="h-12 px-8 text-base border-foreground/[0.1] text-foreground/70 hover:text-foreground hover:bg-foreground/[0.04] rounded-xl">
              Live Demo
            </Button>
          </Link>
        </div>

        {/* Terminal */}
        <div className="mt-20 w-full max-w-2xl">
          <div className="rounded-2xl border border-foreground/[0.08] bg-foreground/[0.02] overflow-hidden shadow-2xl shadow-black/50">
            <div className="flex items-center gap-2 border-b border-foreground/[0.06] px-5 py-3.5">
              <div className="flex gap-2">
                <div className="h-3 w-3 rounded-full bg-foreground/[0.08]" />
                <div className="h-3 w-3 rounded-full bg-foreground/[0.08]" />
                <div className="h-3 w-3 rounded-full bg-foreground/[0.08]" />
              </div>
              <span className="ml-3 text-xs text-foreground/20 font-mono">~/projects</span>
            </div>
            <div className="p-6 font-mono text-[13px] leading-7 text-left space-y-1">
              <div>
                <span className="text-emerald-400">❯</span>{" "}
                <span className="text-foreground/70">aboxctl agent search</span>{" "}
                <span className="text-cyan-300">marketing</span>
              </div>
              <div className="text-foreground/25 text-xs">
                Found 7 agents in openagent-spec/registry
              </div>
              <div className="mt-2">
                <span className="text-emerald-400">❯</span>{" "}
                <span className="text-foreground/70">aboxctl agent run</span>{" "}
                <span className="text-cyan-300">cro-optimizer</span>{" "}
                <span className="text-foreground/30">--runtime</span>{" "}
                <span className="text-purple-300">claude</span>
              </div>
              <div className="text-foreground/25 text-xs">
                Installing CRO Optimizer v1.0.0...
              </div>
              <div className="text-foreground/25 text-xs">
                Creating sandbox... <span className="text-emerald-400">done</span>
              </div>
              <div className="text-foreground/25 text-xs">
                Runtime: claude-sonnet-4-6
              </div>
              <div className="text-emerald-400 text-xs mt-1">
                ✓ Agent started! Run ID: f8870923
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="relative border-t border-foreground/[0.06]">
        <div className="mx-auto max-w-6xl px-6 py-28">
          <div className="text-center mb-16">
            <h2 className="text-3xl font-bold tracking-tight">Everything you need</h2>
            <p className="mt-3 text-foreground/40">Built for developers who want to ship fast</p>
          </div>
          <div className="grid gap-6 sm:grid-cols-3">
            <FeatureCard
              emoji="🤖"
              title="11 Runtimes"
              description="Claude Code, Codex, Gemini CLI, Aider, Cursor, Goose, OpenHands — switch with one flag."
              gradient="from-blue-500/20 to-transparent"
            />
            <FeatureCard
              emoji="💬"
              title="6 IM Channels"
              description="Telegram, Discord, Slack, WeChat, Feishu, WeCom. Users chat where they already are."
              gradient="from-purple-500/20 to-transparent"
            />
            <FeatureCard
              emoji="🔒"
              title="Secure Sandbox"
              description="Docker, Kubernetes, Firecracker microVMs, or E2B cloud. VM-level isolation by default."
              gradient="from-cyan-500/20 to-transparent"
            />
            <FeatureCard
              emoji="⚡"
              title="WebSocket Streaming"
              description="Token-by-token output to web and IM. Real-time typing indicators and connection status."
              gradient="from-amber-500/20 to-transparent"
            />
            <FeatureCard
              emoji="🔄"
              title="Workflow Engine"
              description="Chain agents together. Output templating, dependency resolution, parallel execution."
              gradient="from-emerald-500/20 to-transparent"
            />
            <FeatureCard
              emoji="📊"
              title="Usage & Billing"
              description="Track compute, tokens, storage. Stripe integration. Free tier included."
              gradient="from-rose-500/20 to-transparent"
            />
          </div>
        </div>
      </section>

      {/* How it works */}
      <section className="relative border-t border-foreground/[0.06]">
        <div className="mx-auto max-w-6xl px-6 py-28">
          <h2 className="text-center text-3xl font-bold tracking-tight">
            Three steps to deploy
          </h2>
          <div className="mt-20 grid gap-0 sm:grid-cols-3">
            <StepCard
              number="01"
              title="Browse Agents"
              description="60+ agents in the OpenAgent Registry. Search by role, runtime, or skill."
              active
            />
            <StepCard
              number="02"
              title="Pick a Runtime"
              description="Claude, Codex, Gemini, or bring your own. Each optimized for different tasks."
            />
            <StepCard
              number="03"
              title="Ship It"
              description="One command to deploy. Chat via web or IM. Monitor with built-in dashboards."
            />
          </div>
        </div>
      </section>

      {/* Stats */}
      <section className="border-t border-foreground/[0.06]">
        <div className="mx-auto max-w-6xl px-6 py-20">
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-8 text-center">
            <div>
              <p className="text-4xl font-bold tracking-tight">60+</p>
              <p className="mt-1 text-sm text-foreground/30">Agents</p>
            </div>
            <div>
              <p className="text-4xl font-bold tracking-tight">11</p>
              <p className="mt-1 text-sm text-foreground/30">Runtimes</p>
            </div>
            <div>
              <p className="text-4xl font-bold tracking-tight">6</p>
              <p className="mt-1 text-sm text-foreground/30">IM Channels</p>
            </div>
            <div>
              <p className="text-4xl font-bold tracking-tight bg-gradient-to-r from-emerald-400 to-cyan-400 bg-clip-text text-transparent">MIT</p>
              <p className="mt-1 text-sm text-foreground/30">Open Source</p>
            </div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="border-t border-foreground/[0.06]">
        <div className="mx-auto max-w-6xl px-6 py-28 text-center">
          <h2 className="text-4xl font-bold tracking-tight">
            Ready to ship?
          </h2>
          <p className="mt-4 text-foreground/40 text-lg">
            Start deploying AI agents in minutes.
          </p>
          <div className="mt-10">
            <Link href="/register">
              <Button size="lg" className="h-14 px-10 text-base bg-foreground text-background hover:bg-foreground/90 border-0 rounded-xl font-semibold">
                Get Started Free →
              </Button>
            </Link>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-foreground/[0.06]">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-8">
          <div className="flex items-center gap-6 text-xs text-foreground/25">
            <div className="flex items-center gap-2">
              <div className="flex h-5 w-5 items-center justify-center rounded bg-foreground/[0.06]">
                <svg className="h-2.5 w-2.5 text-foreground/50" viewBox="0 0 24 24" fill="currentColor">
                  <path d="M12 2L2 22h20L12 2z" />
                </svg>
              </div>
              <span>ABox</span>
            </div>
            <span>Built by Zoe</span>
            <span>MIT License</span>
          </div>
          <a
            href="https://github.com/jiusanzhou/agentbox"
            target="_blank"
            rel="noopener noreferrer"
            className="text-foreground/20 hover:text-foreground/50 transition-colors"
          >
            <svg className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
            </svg>
          </a>
        </div>
      </footer>
    </div>
  );
}

function FeatureCard({
  emoji,
  title,
  description,
  gradient,
}: {
  emoji: string;
  title: string;
  description: string;
  gradient: string;
}) {
  return (
    <div className="group relative rounded-2xl border border-foreground/[0.06] bg-foreground/[0.02] p-8 transition-all hover:border-foreground/[0.12] hover:bg-foreground/[0.04]">
      <div className={`pointer-events-none absolute inset-0 rounded-2xl bg-gradient-to-b ${gradient} opacity-0 group-hover:opacity-100 transition-opacity`} />
      <div className="relative">
        <span className="text-2xl">{emoji}</span>
        <h3 className="mt-4 text-base font-semibold">{title}</h3>
        <p className="mt-2 text-sm text-foreground/40 leading-relaxed">{description}</p>
      </div>
    </div>
  );
}

function StepCard({
  number,
  title,
  description,
  active,
}: {
  number: string;
  title: string;
  description: string;
  active?: boolean;
}) {
  return (
    <div className={`relative p-8 text-center border-l border-foreground/[0.06] first:border-l-0 ${active ? "" : ""}`}>
      <div className={`mx-auto flex h-10 w-10 items-center justify-center rounded-full text-sm font-mono ${active ? "bg-white text-black" : "border border-foreground/[0.1] text-foreground/30"}`}>
        {number}
      </div>
      <h3 className="mt-5 text-base font-semibold">{title}</h3>
      <p className="mt-2 text-sm text-foreground/40 leading-relaxed">{description}</p>
    </div>
  );
}
