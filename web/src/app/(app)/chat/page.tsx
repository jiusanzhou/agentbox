"use client";

import { useEffect, useRef, useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { cn } from "@/lib/utils";
import { clientFetch, getAiSettings } from "@/lib/api";
import type { Session, Message } from "@/lib/types";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

// ---------------------------------------------------------------------------
// useWebSocket hook
// ---------------------------------------------------------------------------
type WsStatus = "connected" | "reconnecting" | "disconnected";

interface WsMessage {
  type: "token" | "done" | "error";
  content?: string;
}

function useWebSocket(sessionId: string | null) {
  const wsRef = useRef<WebSocket | null>(null);
  const [status, setStatus] = useState<WsStatus>("disconnected");
  const [lastMessage, setLastMessage] = useState<WsMessage | null>(null);
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const backoffRef = useRef(1000);
  const intentionalClose = useRef(false);

  const connect = useCallback(() => {
    if (!sessionId) return;
    const token = typeof window !== "undefined" ? localStorage.getItem("abox_token") : null;
    if (!token) return;

    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${proto}//${window.location.host}/api/v1/ws/${sessionId}?token=${encodeURIComponent(token)}`;

    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.addEventListener("open", () => {
      setStatus("connected");
      backoffRef.current = 1000;
    });

    ws.addEventListener("message", (event) => {
      try {
        const data: WsMessage = JSON.parse(event.data);
        setLastMessage(data);
      } catch {
        // ignore malformed frames
      }
    });

    ws.addEventListener("close", () => {
      wsRef.current = null;
      if (intentionalClose.current) {
        setStatus("disconnected");
        return;
      }
      setStatus("reconnecting");
      const delay = Math.min(backoffRef.current, 30000);
      backoffRef.current = delay * 2;
      reconnectTimer.current = setTimeout(() => {
        connect();
      }, delay);
    });

    ws.addEventListener("error", () => {});
  }, [sessionId]);

  useEffect(() => {
    intentionalClose.current = false;
    connect();

    return () => {
      intentionalClose.current = true;
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
      if (wsRef.current) {
        wsRef.current.close();
        wsRef.current = null;
      }
      setStatus("disconnected");
      setLastMessage(null);
      backoffRef.current = 1000;
    };
  }, [connect]);

  const sendMessage = useCallback(
    (content: string) => {
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        wsRef.current.send(JSON.stringify({ type: "message", content }));
        return true;
      }
      return false;
    },
    []
  );

  return { sendMessage, status, lastMessage };
}

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

  const {
    sendMessage: wsSend,
    status: wsStatus,
    lastMessage: wsLastMessage,
  } = useWebSocket(activeSessionId);

  const wsFullTextRef = useRef("");
  const wsStreamingRef = useRef(false);

  useEffect(() => {
    if (!wsLastMessage || !wsStreamingRef.current) return;

    if (wsLastMessage.type === "token" && wsLastMessage.content) {
      wsFullTextRef.current += wsLastMessage.content;
      const text = wsFullTextRef.current;
      setMessages((prev) => {
        const updated = [...prev];
        updated[updated.length - 1] = { role: "assistant", content: text };
        return updated;
      });
    } else if (wsLastMessage.type === "done") {
      wsStreamingRef.current = false;
      setSending(false);
    } else if (wsLastMessage.type === "error") {
      wsStreamingRef.current = false;
      setMessages((prev) => {
        const updated = [...prev];
        updated[updated.length - 1] = {
          role: "assistant",
          content: `Error: ${wsLastMessage.content ?? "Unknown error"}`,
        };
        return updated;
      });
      setSending(false);
    }
  }, [wsLastMessage]);

  useEffect(() => {
    clientFetch("/api/sessions")
      .then((r) => r.json())
      .then((data) => {
        if (Array.isArray(data)) setSessions(data);
      })
      .catch(() => {});
  }, []);

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
    setMessages((prev) => [...prev, { role: "assistant", content: "" }]);

    if (wsStatus === "connected" && wsSend(text)) {
      wsFullTextRef.current = "";
      wsStreamingRef.current = true;
      return;
    }

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
  }, [input, sending, activeSessionId, createSession, wsStatus, wsSend]);

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

  const activeSession = sessions.find((s) => s.id === activeSessionId);

  return (
    <div className="flex h-[calc(100vh-3.5rem)] -my-8 -mx-6">
      {/* Session sidebar */}
      {sidebarOpen && (
        <div className="w-72 flex-shrink-0 border-r border-border flex flex-col bg-card/50">
          <div className="p-4 border-b border-border">
            <Button
              size="sm"
              className="w-full h-9 text-sm"
              onClick={() => {
                setActiveSessionId(null);
                setMessages([]);
              }}
            >
              <PlusIcon className="h-4 w-4 mr-2" />
              New Chat
            </Button>
          </div>
          <ScrollArea className="flex-1">
            <div className="p-2 space-y-0.5">
              {sessions.map((s) => (
                <button
                  key={s.id}
                  onClick={() => selectSession(s)}
                  className={cn(
                    "w-full rounded-lg px-3 py-2.5 text-left transition-all duration-150",
                    activeSessionId === s.id
                      ? "bg-accent border-l-2 border-foreground"
                      : "hover:bg-accent/50"
                  )}
                >
                  <div className="flex items-center gap-2">
                    <AgentIcon className="h-4 w-4 text-muted-foreground flex-shrink-0" />
                    <p className="text-sm font-medium truncate flex-1">
                      {s.system_prompt
                        ? s.system_prompt.slice(0, 30)
                        : `Session ${(s.id || "").slice(0, 8)}`}
                    </p>
                  </div>
                  <p className="text-xs text-muted-foreground mt-0.5 ml-6">
                    {s.created_at
                      ? new Date(s.created_at).toLocaleDateString()
                      : ""}
                  </p>
                </button>
              ))}
              {sessions.length === 0 && (
                <p className="px-3 py-8 text-xs text-muted-foreground text-center">
                  No sessions yet
                </p>
              )}
            </div>
          </ScrollArea>
        </div>
      )}

      {/* Chat area */}
      <div
        className={cn(
          "flex flex-1 flex-col min-w-0 relative",
          dragOver && "ring-2 ring-foreground ring-inset"
        )}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
      >
        {/* Chat header */}
        <div className="flex items-center gap-3 border-b border-border px-4 h-12 flex-shrink-0">
          <button
            onClick={() => setSidebarOpen(!sidebarOpen)}
            className="h-8 w-8 flex items-center justify-center rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          >
            <SidebarIcon className="h-4 w-4" />
          </button>
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">
              {activeSessionId
                ? `Session ${activeSessionId.slice(0, 8)}`
                : "New Chat"}
            </span>
            {activeSessionId && (
              <span
                className={cn(
                  "h-2 w-2 rounded-full",
                  wsStatus === "connected" && "bg-emerald-500",
                  wsStatus === "reconnecting" && "bg-yellow-500 animate-pulse",
                  wsStatus === "disconnected" && "bg-muted-foreground/30"
                )}
                title={`WebSocket: ${wsStatus}`}
              />
            )}
          </div>
        </div>

        {/* Drag overlay */}
        {dragOver && (
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-background/80 pointer-events-none">
            <div className="rounded-xl border-2 border-dashed border-foreground/20 p-8 text-center">
              <PaperclipIcon className="h-8 w-8 mx-auto mb-2 text-muted-foreground" />
              <p className="text-sm font-medium">Drop file to upload</p>
            </div>
          </div>
        )}

        {/* Messages */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto px-4 py-6">
          {messages.length === 0 ? (
            <div className="flex h-full items-center justify-center">
              <div className="text-center space-y-2">
                <div className="mx-auto h-12 w-12 rounded-full border border-border flex items-center justify-center">
                  <AgentIcon className="h-5 w-5 text-muted-foreground" />
                </div>
                <p className="text-base font-medium">Start a conversation</p>
                <p className="text-sm text-muted-foreground">
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
                  <div
                    className={cn(
                      "max-w-[70%] rounded-2xl px-4 py-3",
                      msg.role === "user"
                        ? "bg-foreground text-background"
                        : "bg-muted"
                    )}
                  >
                    {msg.role === "assistant" ? (
                      msg.content ? (
                        <div className="text-sm prose prose-sm dark:prose-invert max-w-none [&_pre]:bg-background/50 [&_pre]:p-3 [&_pre]:rounded-lg [&_pre]:overflow-x-auto [&_pre]:border [&_pre]:border-border [&_code:not(pre_code)]:bg-background/50 [&_code:not(pre_code)]:rounded [&_code:not(pre_code)]:px-1 [&_code:not(pre_code)]:text-xs">
                          <ReactMarkdown remarkPlugins={[remarkGfm]}>
                            {msg.content}
                          </ReactMarkdown>
                        </div>
                      ) : sending ? (
                        <div className="flex items-center gap-1 py-1">
                          <span className="typing-dot h-1.5 w-1.5 rounded-full bg-muted-foreground" />
                          <span className="typing-dot h-1.5 w-1.5 rounded-full bg-muted-foreground" />
                          <span className="typing-dot h-1.5 w-1.5 rounded-full bg-muted-foreground" />
                        </div>
                      ) : null
                    ) : (
                      <p className="text-sm whitespace-pre-wrap">{msg.content}</p>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>

        {/* Input */}
        <div className="border-t border-border p-4">
          <div className="mx-auto max-w-2xl">
            <div className="flex items-center gap-2 rounded-xl border border-border bg-card px-3 py-2 focus-within:border-foreground/20 transition-colors">
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
              <button
                className="h-8 w-8 flex items-center justify-center rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors flex-shrink-0"
                onClick={() => fileInputRef.current?.click()}
                disabled={!activeSessionId || uploading}
                title={activeSessionId ? "Upload file" : "Start a session first"}
              >
                {uploading ? (
                  <span className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                ) : (
                  <PaperclipIcon className="h-4 w-4" />
                )}
              </button>
              <input
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Type a message..."
                disabled={sending}
                className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              />
              <button
                onClick={sendMessage}
                disabled={sending || !input.trim()}
                className={cn(
                  "h-8 w-8 flex items-center justify-center rounded-lg transition-all duration-150 flex-shrink-0",
                  input.trim()
                    ? "bg-foreground text-background hover:opacity-80"
                    : "text-muted-foreground"
                )}
              >
                <ArrowUpIcon className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Icons ─── */

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

function ArrowUpIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 19V5" />
      <path d="m5 12 7-7 7 7" />
    </svg>
  );
}

function PlusIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 5v14" />
      <path d="M5 12h14" />
    </svg>
  );
}

function AgentIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <path d="M8 14s1.5 2 4 2 4-2 4-2" />
      <line x1="9" y1="9" x2="9.01" y2="9" />
      <line x1="15" y1="9" x2="15.01" y2="9" />
    </svg>
  );
}
