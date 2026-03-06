"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { clientFetch } from "@/lib/api";
import type { Session, Message } from "@/lib/types";

export default function ChatPage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const scrollRef = useRef<HTMLDivElement>(null);
  const pollRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load sessions
  useEffect(() => {
    clientFetch("/api/sessions")
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setSessions(data);
      })
      .catch(() => {});
  }, []);

  // Auto-scroll to bottom
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  // Cleanup poll on unmount
  useEffect(() => {
    return () => {
      if (pollRef.current) clearTimeout(pollRef.current);
    };
  }, []);

  const createSession = useCallback(async (systemPrompt?: string) => {
    try {
      const res = await clientFetch("/api/sessions", {
        method: "POST",
        body: JSON.stringify({ system_prompt: systemPrompt }),
      });
      const session = await res.json();
      setSessions((prev) => [session, ...prev]);
      setActiveSessionId(session.id);
      setMessages([]);
      return session.id as string;
    } catch {
      return null;
    }
  }, []);

  const sendMessage = useCallback(async () => {
    const text = input.trim();
    if (!text || sending) return;

    setSending(true);
    setInput("");

    let sessionId = activeSessionId;
    if (!sessionId) {
      sessionId = await createSession();
      if (!sessionId) {
        setSending(false);
        return;
      }
    }

    const userMsg: Message = { role: "user", content: text };
    setMessages((prev) => [...prev, userMsg]);

    try {
      const res = await clientFetch("/api/sessions/message", {
        method: "POST",
        body: JSON.stringify({ session_id: sessionId, content: text }),
      });
      const data = await res.json();

      if (data.content || data.message) {
        const assistantMsg: Message = {
          role: "assistant",
          content: data.content || data.message,
        };
        setMessages((prev) => [...prev, assistantMsg]);
      } else if (data.status === "processing") {
        // Poll for response
        const poll = async (attempts: number) => {
          if (attempts > 100) {
            setSending(false);
            return;
          }
          try {
            const pollRes = await clientFetch("/api/sessions/message", {
              method: "POST",
              body: JSON.stringify({
                session_id: sessionId,
                content: "",
                poll: true,
              }),
            });
            const pollData = await pollRes.json();
            if (pollData.content || pollData.message) {
              setMessages((prev) => [
                ...prev,
                { role: "assistant", content: pollData.content || pollData.message },
              ]);
              setSending(false);
            } else {
              pollRef.current = setTimeout(() => poll(attempts + 1), 100);
            }
          } catch {
            setSending(false);
          }
        };
        poll(0);
        return;
      }
    } catch {
      setMessages((prev) => [
        ...prev,
        { role: "assistant", content: "Failed to send message. Please try again." },
      ]);
    }
    setSending(false);
  }, [input, sending, activeSessionId, createSession]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const selectSession = (session: Session) => {
    setActiveSessionId(session.id);
    setMessages([]);
  };

  return (
    <div className="flex h-[calc(100vh-4rem)] -my-8 -mx-6">
      {/* Session sidebar */}
      {sidebarOpen && (
        <div className="w-64 flex-shrink-0 border-r border-border flex flex-col">
          <div className="p-3 border-b border-border">
            <Button
              size="sm"
              className="w-full"
              onClick={() => {
                setActiveSessionId(null);
                setMessages([]);
              }}
            >
              New Session
            </Button>
          </div>
          <ScrollArea className="flex-1">
            <div className="p-2 space-y-1">
              {sessions.map((s) => (
                <button
                  key={s.id}
                  onClick={() => selectSession(s)}
                  className={cn(
                    "w-full rounded-md px-3 py-2 text-left text-sm transition-colors",
                    activeSessionId === s.id
                      ? "bg-accent text-accent-foreground"
                      : "text-muted-foreground hover:bg-accent/50 hover:text-foreground"
                  )}
                >
                  <p className="font-medium truncate">
                    {s.system_prompt ? s.system_prompt.slice(0, 30) : `Session ${s.id.slice(0, 8)}`}
                  </p>
                  <p className="text-xs opacity-60 mt-0.5">
                    {new Date(s.created_at).toLocaleDateString()}
                  </p>
                </button>
              ))}
              {sessions.length === 0 && (
                <p className="px-3 py-4 text-xs text-muted-foreground text-center">
                  No sessions yet
                </p>
              )}
            </div>
          </ScrollArea>
        </div>
      )}

      {/* Chat area */}
      <div className="flex flex-1 flex-col min-w-0">
        {/* Chat header */}
        <div className="flex items-center gap-2 border-b border-border px-4 py-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="h-8 w-8 p-0"
          >
            <SidebarIcon className="h-4 w-4" />
          </Button>
          <span className="text-sm font-medium">
            {activeSessionId ? `Session ${activeSessionId.slice(0, 8)}` : "New Chat"}
          </span>
        </div>

        {/* Messages */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-6">
          {messages.length === 0 ? (
            <div className="flex h-full items-center justify-center">
              <div className="text-center">
                <p className="text-lg font-medium text-muted-foreground">
                  Start a conversation
                </p>
                <p className="mt-1 text-sm text-muted-foreground/60">
                  Send a message to begin chatting with an AI agent
                </p>
              </div>
            </div>
          ) : (
            <div className="mx-auto max-w-2xl space-y-4">
              {messages.map((msg, i) => (
                <div
                  key={i}
                  className={cn(
                    "flex",
                    msg.role === "user" ? "justify-end" : "justify-start"
                  )}
                >
                  <Card
                    className={cn(
                      "max-w-[80%] px-4 py-3",
                      msg.role === "user"
                        ? "bg-primary text-primary-foreground"
                        : "bg-muted"
                    )}
                  >
                    <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
                  </Card>
                </div>
              ))}
              {sending && (
                <div className="flex justify-start">
                  <Card className="bg-muted px-4 py-3">
                    <p className="text-sm text-muted-foreground animate-pulse">
                      Thinking...
                    </p>
                  </Card>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Input */}
        <div className="border-t border-border p-4">
          <div className="mx-auto max-w-2xl flex gap-2">
            <Input
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Type a message..."
              disabled={sending}
              className="flex-1"
            />
            <Button onClick={sendMessage} disabled={sending || !input.trim()}>
              Send
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

function SidebarIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <rect width="18" height="18" x="3" y="3" rx="2" />
      <path d="M9 3v18" />
    </svg>
  );
}
