"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Card } from "@/components/ui/card";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { clientFetch, getAiSettings } from "@/lib/api";
import type { Session, Message } from "@/lib/types";

export default function ChatPage() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [dragOver, setDragOver] = useState(false);
  const scrollRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

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

  const handleUpload = useCallback(async (file: File) => {
    if (!activeSessionId) return;
    setUploading(true);
    try {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("session_id", activeSessionId);

      const token = typeof window !== "undefined" ? localStorage.getItem("abox_token") : null;
      const res = await fetch("/api/upload", {
        method: "POST",
        headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}) },
        body: formData,
      });
      const data = await res.json();
      if (data.path) {
        setInput(`I uploaded a file: ${data.name} (${data.size} bytes) at ${data.path}. Please read and process it.`);
      } else if (data.error) {
        setMessages((prev) => [...prev, { role: "assistant", content: `Upload error: ${data.error}` }]);
      }
    } catch {
      setMessages((prev) => [...prev, { role: "assistant", content: "Failed to upload file." }]);
    }
    setUploading(false);
  }, [activeSessionId]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const file = e.dataTransfer.files?.[0];
    if (file) handleUpload(file);
  }, [handleUpload]);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
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

    // Add empty assistant message for streaming
    setMessages((prev) => [...prev, { role: "assistant", content: "" }]);

    try {
      const token = typeof window !== "undefined" ? localStorage.getItem("abox_token") : null;
      const ai = getAiSettings();

      const res = await fetch("/api/sessions/stream", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...(ai?.apiKey ? { "x-api-key": ai.apiKey } : {}),
          ...(ai?.baseUrl ? { "x-base-url": ai.baseUrl } : {}),
          ...(ai?.model ? { "x-model": ai.model } : {}),
        },
        body: JSON.stringify({ session_id: sessionId, message: text }),
      });

      const reader = res.body?.getReader();
      const decoder = new TextDecoder();
      let fullText = "";

      if (reader) {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;

          const chunk = decoder.decode(value, { stream: true });
          const lines = chunk.split("\n");

          for (const line of lines) {
            if (!line.startsWith("data: ")) continue;
            try {
              const data = JSON.parse(line.slice(6));
              if (data.token) {
                fullText += data.token;
                setMessages((prev) => {
                  const updated = [...prev];
                  updated[updated.length - 1] = { role: "assistant", content: fullText };
                  return updated;
                });
              }
              if (data.done) {
                if (data.result) {
                  fullText = data.result;
                  setMessages((prev) => {
                    const updated = [...prev];
                    updated[updated.length - 1] = { role: "assistant", content: fullText };
                    return updated;
                  });
                }
              }
              if (data.error) {
                setMessages((prev) => {
                  const updated = [...prev];
                  updated[updated.length - 1] = { role: "assistant", content: `Error: ${data.error}` };
                  return updated;
                });
              }
            } catch {
              // ignore parse errors for partial SSE chunks
            }
          }
        }
      }
    } catch {
      setMessages((prev) => {
        const updated = [...prev];
        if (updated[updated.length - 1]?.role === "assistant" && !updated[updated.length - 1]?.content) {
          updated[updated.length - 1] = { role: "assistant", content: "Failed to send message." };
        }
        return updated;
      });
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
                    {s.system_prompt ? s.system_prompt.slice(0, 30) : `Session ${(s.id || '').slice(0, 8)}`}
                  </p>
                  <p className="text-xs opacity-60 mt-0.5">
                    {s.created_at ? new Date(s.created_at).toLocaleDateString() : ""}
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
      <div
        className={cn("flex flex-1 flex-col min-w-0", dragOver && "ring-2 ring-primary ring-inset")}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
      >
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

        {/* Drag overlay */}
        {dragOver && (
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-background/80 pointer-events-none">
            <div className="rounded-lg border-2 border-dashed border-primary p-8 text-center">
              <PaperclipIcon className="h-8 w-8 mx-auto mb-2 text-primary" />
              <p className="text-sm font-medium">Drop file to upload</p>
            </div>
          </div>
        )}

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
                    <p className="text-sm whitespace-pre-wrap">
                      {msg.content || (sending && msg.role === "assistant" ? (
                        <span className="text-muted-foreground animate-pulse">Thinking...</span>
                      ) : msg.content)}
                    </p>
                  </Card>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Input */}
        <div className="border-t border-border p-4">
          <div className="mx-auto max-w-2xl flex gap-2">
            <input
              ref={fileInputRef}
              type="file"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) handleUpload(file);
                e.target.value = "";
              }}
            />
            <Button
              variant="ghost"
              size="sm"
              className="h-9 w-9 p-0 flex-shrink-0"
              onClick={() => fileInputRef.current?.click()}
              disabled={!activeSessionId || uploading}
              title={activeSessionId ? "Upload file" : "Start a session first"}
            >
              {uploading ? (
                <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
              ) : (
                <PaperclipIcon className="h-4 w-4" />
              )}
            </Button>
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

function PaperclipIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="m21.44 11.05-9.19 9.19a6 6 0 0 1-8.49-8.49l8.57-8.57A4 4 0 1 1 18 8.84l-8.59 8.57a2 2 0 0 1-2.83-2.83l8.49-8.48" />
    </svg>
  );
}
