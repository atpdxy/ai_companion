import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type FormEvent,
} from "react";
import {
  ArrowDownToLine,
  ArrowRight,
  BookHeart,
  Check,
  ChevronRight,
  Heart,
  Leaf,
  LogOut,
  Menu,
  MessageCircle,
  Plus,
  Send,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Square,
  Trash2,
  X,
} from "lucide-react";
import { api, APIError, streamMessage } from "./api";
import type {
  AgentResult,
  Conversation,
  Memory,
  Message,
  Persona,
  User,
} from "./types";
import { NotificationSettings } from "./NotificationSettings";
import { CompanionLibrary, relationshipNames } from "./CompanionLibrary";
import { VoiceRecorder, SpeechPlayer } from "./VoiceControls";
import { ToolPanel } from "./ToolPanel";
import type { Capabilities } from "./types";

type Tab = "chat" | "memories" | "persona" | "settings";
const strategyNames: Record<string, string> = {
  listen: "认真倾听",
  empathize: "温柔共情",
  clarify: "一起理清",
  advise: "一起想办法",
  lighten: "分享快乐",
  respect_space: "留一点空间",
  safety_support: "安全支持",
};
const dateLabel = (value: string) =>
  new Date(value).toLocaleDateString("zh-CN", {
    month: "long",
    day: "numeric",
  });
const errorText = (error: unknown) =>
  error instanceof Error ? error.message : "操作失败，请稍后再试";

export function App() {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [tab, setTab] = useState<Tab>("chat");
  const [persona, setPersona] = useState<Persona | null>(null);
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [active, setActive] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [memories, setMemories] = useState<Memory[]>([]);
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [modelMode, setModelMode] = useState("demo");
  const [capabilities, setCapabilities] = useState<Capabilities>({
    mode: "demo",
    asr_enabled: false,
    tts_enabled: false,
  });
  const [result, setResult] = useState<AgentResult | null>(null);
  const [mobileMenu, setMobileMenu] = useState(false);
  const [notice, setNotice] = useState("");
  const controller = useRef<AbortController | null>(null);
  const end = useRef<HTMLDivElement>(null);
  const session = useRef(0);
  const currentCompanion = useRef<string | null>(null);
  const loadEpoch = useRef(0);

  const refresh = useCallback(async () => {
    const [identity, character, history, saved, status] = await Promise.all([
      api<User>("/me"),
      api<Persona>("/persona"),
      api<Conversation[]>("/conversations"),
      api<Memory[]>("/memories"),
      api<Capabilities>("/status"),
    ]);
    setUser(identity);
    setPersona(character);
    currentCompanion.current = character.id;
    setConversations(history);
    setMemories(saved);
    setModelMode(status.mode);
    setCapabilities(status);
  }, []);
  useEffect(() => {
    refresh()
      .catch((error) => {
        if (!(error instanceof APIError && error.status === 401))
          setError(errorText(error));
      })
      .finally(() => setLoading(false));
  }, [refresh]);
  useEffect(() => {
    end.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, sending]);
  useEffect(
    () => () => {
      controller.current?.abort();
    },
    [],
  );
  useEffect(() => {
    if (!notice) return;
    const timer = window.setTimeout(() => setNotice(""), 3500);
    return () => window.clearTimeout(timer);
  }, [notice]);

  async function openConversation(id: string) {
    if (sending) return;
    const epoch = ++loadEpoch.current;
    setHistoryLoading(true);
    setError("");
    setMobileMenu(false);
    setTab("chat");
    setResult(null);
    try {
      const history = await api<Message[]>(`/conversations/${id}/messages`);
      if (epoch !== loadEpoch.current) return;
      setActive(id);
      setMessages(history);
    } catch (error) {
      setError(errorText(error));
    } finally {
      if (epoch === loadEpoch.current) setHistoryLoading(false);
    }
  }
  function newConversation() {
    if (sending) return;
    loadEpoch.current++;
    setHistoryLoading(false);
    setActive(null);
    setMessages([]);
    setResult(null);
    setDraft("");
    setTab("chat");
    setMobileMenu(false);
  }

  async function send(content: string, retryTurn?: string) {
    if (!content.trim() || sending || historyLoading) return;
    const currentSession = session.current;
    setSending(true);
    setError("");
    setDraft("");
    const abort = new AbortController();
    controller.current = abort;
    const turn = retryTurn ?? crypto.randomUUID();
    let conversation = active;
    try {
      if (!conversation) {
        const created = await api<Conversation>("/conversations", "POST", {
          title: content.slice(0, 28),
          companion_id: persona?.id,
        });
        conversation = created.id;
        setActive(created.id);
        setConversations((previous) => [created, ...previous]);
      }
      const now = new Date().toISOString();
      const assistantID = `pending-${turn}`;
      setMessages((previous) => [
        ...previous.filter(
          (message) =>
            !(message.turn_id === turn && message.role === "assistant"),
        ),
        ...(!retryTurn
          ? [
              {
                id: `user-${turn}`,
                turn_id: turn,
                role: "user" as const,
                content,
                created_at: now,
              },
            ]
          : []),
        {
          id: assistantID,
          turn_id: turn,
          role: "assistant",
          content: "",
          created_at: now,
        },
      ]);
      await streamMessage(
        conversation,
        content,
        turn,
        abort.signal,
        (event, payload) => {
          if (session.current !== currentSession) return;
          if (event === "delta")
            setMessages((previous) =>
              previous.map((message) =>
                message.id === assistantID
                  ? {
                      ...message,
                      content: message.content + String(payload.text),
                    }
                  : message,
              ),
            );
          if (event === "message_end") {
            const final = payload as unknown as AgentResult;
            setResult(final);
            setMessages((previous) =>
              previous.map((message) =>
                message.id === assistantID
                  ? { ...message, content: final.response }
                  : message,
              ),
            );
          }
        },
      );
      const [history, saved] = await Promise.all([
        api<Message[]>(`/conversations/${conversation}/messages`),
        api<Memory[]>(
          `/memories${persona?.id ? `?companion_id=${persona.id}` : ""}`,
        ),
      ]);
      if (session.current === currentSession) {
        setMessages(history);
        setMemories(saved);
      }
    } catch (error) {
      if (session.current === currentSession) {
        setError(
          abort.signal.aborted
            ? "已停止接收。此轮未完成时可以重试。"
            : errorText(error),
        );
        setMessages((previous) =>
          previous.map((message) =>
            message.turn_id === turn && message.role === "assistant"
              ? { ...message, incomplete: true }
              : message,
          ),
        );
      }
    } finally {
      if (session.current === currentSession) {
        setSending(false);
        controller.current = null;
      }
    }
  }
  async function memoryAction(
    memory: Memory,
    status: Memory["consent_status"],
    content = memory.content,
  ) {
    const ownerSession = session.current;
    const companionID = persona?.id;
    try {
      await api(`/memories/${memory.id}`, "PATCH", {
        content,
        consent_status: status,
        expires_at: memory.expires_at,
      });
      const saved = await api<Memory[]>(
        `/memories${persona?.id ? `?companion_id=${persona.id}` : ""}`,
      );
      if (
        session.current !== ownerSession ||
        currentCompanion.current !== companionID
      )
        return;
      setMemories(saved);
      setNotice("记忆已更新");
    } catch (error) {
      setError(errorText(error));
    }
  }
  async function deleteMemory(id: string) {
    const ownerSession = session.current;
    const companionID = persona?.id;
    try {
      await api(`/memories/${id}`, "DELETE");
      if (
        session.current !== ownerSession ||
        currentCompanion.current !== companionID
      )
        return;
      setMemories((previous) => previous.filter((memory) => memory.id !== id));
      setNotice("这条记忆已删除");
    } catch (error) {
      setError(errorText(error));
    }
  }
  async function toggleMemory() {
    if (!user) return;
    const ownerSession = session.current;
    const companionID = persona?.id;
    try {
      const identity = await api<User>("/me/preferences", "PATCH", {
        memory_consent: !user.memory_consent,
      });
      if (session.current !== ownerSession) return;
      setUser(identity);
      const saved = await api<Memory[]>(
        `/memories${persona?.id ? `?companion_id=${persona.id}` : ""}`,
      );
      if (
        session.current !== ownerSession ||
        currentCompanion.current !== companionID
      )
        return;
      setMemories(saved);
      setNotice(
        identity.memory_consent
          ? "已开启候选记忆，每一条仍需你确认"
          : "已关闭记忆提取和召回",
      );
    } catch (error) {
      setError(errorText(error));
    }
  }
  function clearSession() {
    controller.current?.abort();
    session.current++;
    currentCompanion.current = null;
    loadEpoch.current++;
    setUser(null);
    setActive(null);
    setMessages([]);
    setMemories([]);
    setConversations([]);
    setPersona(null);
    setResult(null);
    setDraft("");
    setSending(false);
    setHistoryLoading(false);
    setNotice("");
    setError("");
    setMobileMenu(false);
    setTab("chat");
  }
  async function logout() {
    try {
      await api("/auth/logout", "POST");
      clearSession();
    } catch (error) {
      setError(errorText(error));
    }
  }

  if (loading)
    return (
      <div className="loading">
        <Leaf size={32} />
        <p>为你留一盏灯…</p>
      </div>
    );
  if (!user)
    return (
      <Login
        error={error}
        onLogin={async () => {
          setError("");
          await refresh();
        }}
      />
    );

  const pending = memories.filter(
    (memory) => memory.consent_status === "pending",
  ).length;
  return (
    <div className="app-shell">
      <aside className={`sidebar ${mobileMenu ? "is-open" : ""}`}>
        <a
          className="brand"
          href="#"
          onClick={(event) => {
            event.preventDefault();
            setTab("chat");
          }}
        >
          <span className="brand-mark">
            <Leaf size={23} />
          </span>
          <span>
            知夏<small>A LITTLE CLOSER</small>
          </span>
        </a>
        <button
          className="new-chat"
          onClick={newConversation}
          disabled={sending}
        >
          <Plus size={17} /> 开启一段对话 <span>↗</span>
        </button>
        <div className="nav-caption">
          我们的空间 <span>SPACE</span>
        </div>
        <nav aria-label="主导航">
          {(
            [
              { id: "chat", icon: MessageCircle, title: "此刻的陪伴" },
              { id: "memories", icon: BookHeart, title: "我们的记忆" },
              { id: "persona", icon: SlidersHorizontal, title: "AI 伙伴" },
              { id: "settings", icon: Settings, title: "偏好与隐私" },
            ] as const
          ).map((item) => (
            <button
              key={item.id}
              className={tab === item.id ? "active" : ""}
              onClick={() => {
                setTab(item.id);
                setMobileMenu(false);
              }}
            >
              <item.icon size={18} />
              {item.title}
              {item.id === "memories" && pending > 0 && <b>{pending}</b>}
            </button>
          ))}
        </nav>
        <div className="history">
          <div className="nav-caption">
            留在这里的话 <span>RECENT</span>
          </div>
          {conversations.filter(
            (conversation) => conversation.companion_id === persona?.id,
          ).length === 0 ? (
            <p className="history-empty">第一句，随时都可以开始。</p>
          ) : (
            conversations
              .filter(
                (conversation) => conversation.companion_id === persona?.id,
              )
              .map((conversation) => (
                <button
                  className={active === conversation.id ? "selected" : ""}
                  key={conversation.id}
                  disabled={sending}
                  onClick={() => void openConversation(conversation.id)}
                >
                  <span>{conversation.title}</span>
                  <small>{dateLabel(conversation.created_at)}</small>
                </button>
              ))
          )}
        </div>
        <div className="sidebar-note">
          <Heart size={17} />
          <p>
            亲近，也尊重你的边界。
            <br />
            <span>这里的每一次停留，由你决定。</span>
          </p>
        </div>
        <div className="account">
          <span className="account-avatar">{user.email[0].toUpperCase()}</span>
          <div>
            <strong>我的小小世界</strong>
            <small>{user.email}</small>
          </div>
          <button
            className="icon-button"
            onClick={() => void logout()}
            aria-label="退出登录"
          >
            <LogOut size={17} />
          </button>
        </div>
      </aside>
      {mobileMenu && (
        <button
          className="sidebar-backdrop"
          aria-label="关闭导航"
          onClick={() => setMobileMenu(false)}
        />
      )}
      <main className="main-panel">
        <header className="topbar">
          <div>
            <button
              className="icon-button mobile-only"
              aria-label="打开导航"
              onClick={() => setMobileMenu(true)}
            >
              <Menu size={21} />
            </button>
            <span className="tiny-leaf">✳</span>
            <span>
              {tab === "chat"
                ? "慢慢说，我在听"
                : tab === "memories"
                  ? "重要的事，由你决定留下"
                  : tab === "persona"
                    ? "一点一点，成为你喜欢的样子"
                    : "把选择权留给你"}
            </span>
          </div>
          <span className="private-badge">
            <ShieldCheck size={14} /> 只属于你的空间
          </span>
        </header>
        {error && (
          <div className="error-banner" role="alert">
            <span>{error}</span>
            <button
              className="icon-button"
              aria-label="关闭错误"
              onClick={() => setError("")}
            >
              <X size={16} />
            </button>
          </div>
        )}
        {notice && (
          <div className="toast" role="status">
            <Check size={15} />
            {notice}
          </div>
        )}
        {tab === "chat" && (
          <div className="chat-layout">
            <section className="chat-main">
              <div className="character-heading">
                <div className="avatar small">
                  {persona?.name.slice(-1)}
                  <span />
                </div>
                <div>
                  <h1>{persona?.name ?? "知夏"}</h1>
                  <p>
                    {persona
                      ? relationshipNames[persona.relationship]
                      : "AI 伙伴"}{" "}
                    · 独立的陪伴空间
                  </p>
                </div>
                <span className="character-status">
                  {modelMode === "demo" ? "离线演示" : "模型已连接"}
                </span>
              </div>
              <div
                className="message-scroll"
                aria-live="polite"
                aria-busy={sending}
              >
                {messages.length === 0 ? (
                  <div className="welcome">
                    <div className="welcome-orbit">
                      <div className="avatar large">
                        {persona?.name.slice(-1)}
                      </div>
                      <span className="orbit-flower">✳</span>
                      <span className="orbit-dot" />
                    </div>
                    <div className="eyebrow">A MOMENT, JUST FOR YOU</div>
                    <h2>
                      {persona?.greeting &&
                      persona.greeting !== "今天的你，想从哪里说起？" ? (
                        persona.greeting
                      ) : (
                        <>
                          今天的你，
                          <br />
                          想从哪里说起？
                        </>
                      )}
                    </h2>
                    <p>
                      开心的、疲惫的，或是还没想好怎么说的。
                      <br />
                      不用整理好心情，也可以来见我。
                    </p>
                    <div className="suggestions">
                      {[
                        "今天有一点累了",
                        "想和你分享一件小事",
                        "就这样陪我待一会儿",
                      ].map((text, index) => (
                        <button key={text} onClick={() => void send(text)}>
                          <span>0{index + 1}</span>
                          {text}
                          <ArrowRight size={15} />
                        </button>
                      ))}
                    </div>
                    <span className="welcome-footnote">
                      每一种心情，都可以有一个落脚的地方。
                    </span>
                  </div>
                ) : (
                  <div className="messages">
                    <div className="day-divider">
                      <span>{dateLabel(messages[0].created_at)}</span>
                    </div>
                    {messages.map((message) => (
                      <article
                        className={`message ${message.role}`}
                        key={message.id}
                      >
                        {message.role === "assistant" && (
                          <div className="avatar mini">
                            {persona?.name.slice(-1)}
                          </div>
                        )}
                        <div className="message-content">
                          <div className="bubble">
                            {message.content ||
                              (sending ? (
                                <span className="typing">
                                  <i />
                                  <i />
                                  <i />
                                </span>
                              ) : (
                                "这轮回复未完成。"
                              ))}
                          </div>
                          {message.role === "assistant" &&
                            !message.incomplete &&
                            !sending &&
                            !message.id.startsWith("pending-") && (
                              <button
                                className="feedback-button"
                                aria-label="这条回复有帮助"
                                onClick={() =>
                                  void api("/feedback", "POST", {
                                    message_id: message.id,
                                    rating: 1,
                                    tag: "helpful",
                                  })
                                    .then(() => setNotice("谢谢你的反馈"))
                                    .catch((error) =>
                                      setError(errorText(error)),
                                    )
                                }
                              >
                                <Heart size={12} /> 有被听见
                              </button>
                            )}
                          {capabilities.tts_enabled &&
                            active &&
                            message.role === "assistant" &&
                            !message.incomplete &&
                            !sending &&
                            !message.id.startsWith("pending-") && (
                              <SpeechPlayer
                                conversationID={active}
                                messageID={message.id}
                                onError={setError}
                              />
                            )}
                          {message.incomplete && (
                            <button
                              className="retry-button"
                              disabled={sending}
                              onClick={() => {
                                const original = messages.find(
                                  (item) =>
                                    item.turn_id === message.turn_id &&
                                    item.role === "user",
                                );
                                if (original)
                                  void send(original.content, message.turn_id);
                              }}
                            >
                              重试这一轮 <ArrowRight size={12} />
                            </button>
                          )}
                        </div>
                      </article>
                    ))}
                    <div ref={end} />
                  </div>
                )}
              </div>
              <form
                className="composer-area"
                onSubmit={(event) => {
                  event.preventDefault();
                  void send(draft);
                }}
              >
                <div className="composer">
                  <textarea
                    aria-label="输入你想说的话"
                    placeholder={`和${persona?.name ?? "知夏"}说说话…`}
                    rows={2}
                    maxLength={8000}
                    value={draft}
                    disabled={sending || historyLoading}
                    onChange={(event) => setDraft(event.target.value)}
                    onKeyDown={(event) => {
                      if (
                        event.key === "Enter" &&
                        !event.shiftKey &&
                        !event.nativeEvent.isComposing
                      ) {
                        event.preventDefault();
                        void send(draft);
                      }
                    }}
                  />
                  <div className="composer-bottom">
                    {capabilities.asr_enabled && persona && (
                      <VoiceRecorder
                        key={persona.id}
                        companionID={persona.id}
                        disabled={sending || historyLoading}
                        onText={(text) =>
                          setDraft((previous) =>
                            previous ? previous + "\n" + text : text,
                          )
                        }
                        onError={setError}
                      />
                    )}
                    <span>
                      <Leaf size={14} /> 不着急，按你的节奏
                    </span>
                    {sending ? (
                      <button
                        type="button"
                        className="send-button"
                        aria-label="停止回复"
                        onClick={() => controller.current?.abort()}
                      >
                        <Square size={15} />
                      </button>
                    ) : (
                      <button
                        className="send-button"
                        type="submit"
                        disabled={!draft.trim()}
                        aria-label="发送消息"
                      >
                        <Send size={18} />
                      </button>
                    )}
                  </div>
                </div>
                <p className="ai-disclosure">
                  TA 是 AI，情绪理解可能有偏差。现实中的连接，同样值得珍惜。
                  <span>Enter 发送 · Shift + Enter 换行</span>
                </p>
              </form>
            </section>
            <aside className="moment-panel">
              <div className="moment-date">
                <span>此 时 此 刻</span>
                <small>
                  {new Intl.DateTimeFormat("zh-CN", {
                    month: "long",
                    day: "numeric",
                    weekday: "long",
                  }).format(new Date())}
                </small>
              </div>
              <div className="botanical" aria-hidden="true">
                <span className="stem" />
                <Leaf className="leaf leaf-one" />
                <Leaf className="leaf leaf-two" />
                <Leaf className="leaf leaf-three" />
                <div className="sun" />
              </div>
              <blockquote>
                不必每一天
                <br />
                都闪闪发光。
                <br />
                <em>在这里，做你自己。</em>
              </blockquote>
              <div className="moment-line" />
              <div className="memory-mini">
                <BookHeart size={19} />
                <h3>记住，是一种郑重</h3>
                <p>
                  {user.memory_consent
                    ? "每一条记忆，都会先问过你。你可以随时修改或忘记它。"
                    : "只有你允许的事，才会被记住。记忆功能目前关闭。"}
                </p>
                <button onClick={() => setTab("memories")}>
                  看看我们的记忆 <ChevronRight size={14} />
                </button>
              </div>
              {result && (
                <div className="turn-insight">
                  <span>
                    {result.mode === "demo"
                      ? "离线演示 · 非真实模型"
                      : "模型已连接"}
                  </span>
                  <p>
                    {strategyNames[result.strategy.action] ?? "陪伴中"}
                    {result.emotion && result.emotion.confidence > 0
                      ? ` · 情绪推测 ${Math.round(result.emotion.confidence * 100)}%`
                      : " · 情绪尚不确定"}
                  </p>
                </div>
              )}
              <div className="moment-bottom">
                <ShieldCheck size={13} /> 你可以随时离开，也随时回来。
              </div>
            </aside>
          </div>
        )}
        {tab === "memories" && (
          <section className="page-content">
            <div className="eyebrow">THE THINGS WE KEEP</div>
            <h1>
              我们的记忆<span>{persona?.name} · 与其他伙伴的记忆独立。</span>
            </h1>
            <p className="page-description">
              留住你愿意分享的，放下你想忘记的。未经确认的记忆，不会参与对话。
            </p>
            <div className="consent-card">
              <div>
                <h3>账号记忆总开关</h3>
                <p>
                  各伙伴的记忆分别保存。总开关默认关闭，关闭时清除全部待确认项并停止召回。
                </p>
              </div>
              <button
                role="switch"
                aria-checked={user.memory_consent}
                aria-label="允许记忆"
                className={`toggle ${user.memory_consent ? "on" : ""}`}
                onClick={() => void toggleMemory()}
              >
                <span />
              </button>
            </div>
            {memories.length === 0 ? (
              <div className="empty-state">
                <BookHeart size={36} />
                <h2>还没有留下记忆</h2>
                <p>
                  开启后，试着说「请记住：我喜欢下雨天」。
                  <br />
                  我们会先把它放在这里，等你确认。
                </p>
                <button onClick={() => setTab("chat")}>
                  回到对话 <ArrowRight size={15} />
                </button>
              </div>
            ) : (
              <div className="memory-grid">
                {memories.map((memory) => (
                  <MemoryCard
                    key={memory.id}
                    memory={memory}
                    onSave={(content, status) =>
                      void memoryAction(memory, status, content)
                    }
                    onDelete={() => void deleteMemory(memory.id)}
                  />
                ))}
              </div>
            )}
            {memories.length > 0 && (
              <button
                className="danger-link"
                onClick={() => {
                  if (
                    window.confirm(
                      `删除 ${persona?.name} 的全部记忆及对应索引？其他伙伴的记忆不受影响，聊天记录仍保留。`,
                    )
                  )
                    void api(`/memories?companion_id=${persona?.id}`, "DELETE")
                      .then(() => {
                        setMemories([]);
                        setNotice("全部记忆已删除");
                      })
                      .catch((error) => setError(errorText(error)));
                }}
              >
                <Trash2 size={14} /> 忘记全部记忆
              </button>
            )}
          </section>
        )}
        {tab === "persona" && persona && (
          <section className="page-content companion-page">
            <CompanionLibrary
              active={persona}
              disabled={sending}
              onError={setError}
              onSelect={async (id) => {
                if (sending) return;
                const selection = ++loadEpoch.current;
                setHistoryLoading(true);
                try {
                  const selected = await api<Persona>(
                    `/companions/${id}/activate`,
                    "POST",
                  );
                  const saved = await api<Memory[]>(
                    `/memories?companion_id=${id}`,
                  );
                  if (selection !== loadEpoch.current) return;
                  currentCompanion.current = id;
                  setPersona(selected);
                  setActive(null);
                  setMessages([]);
                  setDraft("");
                  setResult(null);
                  setMobileMenu(false);
                  setMemories(saved);
                  setTab("chat");
                } finally {
                  if (selection === loadEpoch.current) setHistoryLoading(false);
                }
              }}
            />
            <PersonaEditor
              key={`${persona.id}-${persona.version}`}
              persona={persona}
              onSave={async (value) => {
                const ownerSession = session.current;
                const companionID = persona.id;
                try {
                  const updated = await api<Persona>(
                    `/companions/${persona.id}`,
                    "PUT",
                    value,
                  );
                  if (
                    session.current !== ownerSession ||
                    currentCompanion.current !== companionID
                  )
                    return;
                  setPersona(updated);
                  setNotice("TA 会以新的风格陪伴你");
                } catch (error) {
                  setError(errorText(error));
                }
              }}
            />
          </section>
        )}
        {tab === "settings" && (
          <section className="page-content settings-page">
            <div className="eyebrow">YOUR SPACE, YOUR CHOICE</div>
            <h1>安心，是有选择。</h1>
            <p className="page-description">亲密的前提，是边界始终清晰。</p>
            <div className="setting-block">
              <ShieldCheck />
              <div>
                <h3>你掌握记忆的去留</h3>
                <p>
                  情绪分析仅用于当前回复，默认不保留推测标签。对话内容会存储在你部署的数据库；使用云端模型时，当前消息、必要上下文与授权记忆会发送给你配置的服务。
                </p>
                <button onClick={() => setTab("memories")}>
                  管理记忆 <ArrowRight size={14} />
                </button>
              </div>
            </div>
            <NotificationSettings onError={setError} />
            {persona && (
              <ToolPanel
                key={persona.id}
                companionID={persona.id}
                onError={setError}
              />
            )}
            <div className="setting-block">
              <ArrowDownToLine />
              <div>
                <h3>带走属于你的内容</h3>
                <p>导出账号、人格、会话和记忆为 JSON。</p>
                <button
                  onClick={() => {
                    void api("/me/export")
                      .then((data) => {
                        const url = URL.createObjectURL(
                          new Blob([JSON.stringify(data, null, 2)], {
                            type: "application/json",
                          }),
                        );
                        const anchor = document.createElement("a");
                        anchor.href = url;
                        anchor.download = "companion-export.json";
                        anchor.click();
                        URL.revokeObjectURL(url);
                      })
                      .catch((error) => setError(errorText(error)));
                  }}
                >
                  导出我的数据 <ArrowRight size={14} />
                </button>
              </div>
            </div>
            <div className="setting-block danger">
              <Trash2 />
              <div>
                <h3>注销账号</h3>
                <p>
                  永久删除此账号、聊天记录、记忆、索引与相关记录。无法恢复。
                </p>
                <button
                  onClick={() => {
                    if (
                      window.prompt("输入 DELETE 确认永久注销账号") === "DELETE"
                    )
                      void api("/me", "DELETE", { confirmation: "DELETE" })
                        .then(clearSession)
                        .catch((error) => setError(errorText(error)));
                  }}
                >
                  注销并删除数据
                </button>
              </div>
            </div>
          </section>
        )}
      </main>
    </div>
  );
}

function Login({
  error: initialError,
  onLogin,
}: {
  error: string;
  onLogin: () => Promise<void>;
}) {
  const [register, setRegister] = useState(true);
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [adult, setAdult] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(initialError);
  async function submit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(`/auth/${register ? "register" : "login"}`, "POST", {
        email,
        password,
        adult_confirmed: adult,
      });
      await onLogin();
    } catch (error) {
      setError(errorText(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="login-page">
      <section className="login-story">
        <a className="brand" href="#">
          <span className="brand-mark">
            <Leaf size={24} />
          </span>
          <span>
            知夏<small>A LITTLE CLOSER</small>
          </span>
        </a>
        <div>
          <div className="eyebrow">SOMEWHERE TO BELONG</div>
          <h1>
            世界很快。
            <br />
            我们，<em>慢慢来。</em>
          </h1>
          <p>
            一处可以安心说话的地方，
            <br />
            一个由你定义的 AI 亲密伙伴。
          </p>
          <div className="login-flower" aria-hidden="true">
            ✳
          </div>
        </div>
        <span className="login-footer">温柔相伴 · 尊重边界 · 由你决定</span>
      </section>
      <section className="login-form-panel">
        <form onSubmit={(event) => void submit(event)}>
          <span className="form-flower">✳</span>
          <h2>{register ? "为你，留一个位置。" : "欢迎回到这里。"}</h2>
          <p>
            {register
              ? "从第一句「你好」开始。"
              : "上次没说完的话，我们可以慢慢说。"}
          </p>
          <label>
            邮箱
            <input
              type="email"
              autoComplete="email"
              required
              maxLength={254}
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              placeholder="you@example.com"
            />
          </label>
          <label>
            密码
            <input
              type="password"
              autoComplete={register ? "new-password" : "current-password"}
              required
              minLength={register ? 10 : undefined}
              maxLength={72}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder={register ? "至少 10 位字符" : "输入你的密码"}
            />
          </label>
          {register && (
            <label className="checkbox-label">
              <input
                type="checkbox"
                required
                checked={adult}
                onChange={(event) => setAdult(event.target.checked)}
              />
              <span>
                我已年满 18 岁，理解这是 AI 陪伴服务。
                <br />
                <small>不提供心理诊断，不替代现实关系。</small>
              </span>
            </label>
          )}
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <button className="primary-button" disabled={busy}>
            {busy
              ? "请稍等…"
              : register
                ? "开启我们的小小世界"
                : "回到我们的空间"}
            <ArrowRight size={17} />
          </button>
          <button
            type="button"
            className="login-switch"
            onClick={() => {
              setRegister(!register);
              setError("");
            }}
          >
            {register ? "已经来过？登录" : "第一次来？创建账号"}
          </button>
          <div className="login-privacy">
            <ShieldCheck size={14} /> 记忆默认关闭，你随时可以导出或删除数据。
          </div>
        </form>
      </section>
    </div>
  );
}

function MemoryCard({
  memory,
  onSave,
  onDelete,
}: {
  memory: Memory;
  onSave: (content: string, status: Memory["consent_status"]) => void;
  onDelete: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [content, setContent] = useState(memory.content);
  return (
    <article className="memory-card">
      <div className="memory-card-top">
        <span className={`memory-status ${memory.consent_status}`}>
          {memory.consent_status === "pending"
            ? "等待你确认"
            : memory.consent_status === "approved"
              ? "已获准记住"
              : "不参与召回"}
        </span>
        <small>{dateLabel(memory.created_at)}</small>
      </div>
      {editing ? (
        <textarea
          aria-label="纠正记忆内容"
          value={content}
          maxLength={1000}
          onChange={(event) => setContent(event.target.value)}
        />
      ) : (
        <h3>{memory.content}</h3>
      )}
      <p className="memory-source">来自你的原话：{memory.source_quote}</p>
      <div className="memory-actions">
        {editing ? (
          <button
            disabled={!content.trim()}
            onClick={() => {
              onSave(content, memory.consent_status);
              setEditing(false);
            }}
          >
            保存纠正
          </button>
        ) : (
          <button onClick={() => setEditing(true)}>纠正</button>
        )}
        {memory.consent_status !== "approved" && (
          <button onClick={() => onSave(memory.content, "approved")}>
            <Check size={14} />
            允许记住
          </button>
        )}
        {memory.consent_status === "pending" && (
          <button onClick={() => onSave(memory.content, "rejected")}>
            不保存
          </button>
        )}
        <button
          aria-label="删除这条记忆"
          className="delete-memory"
          onClick={onDelete}
        >
          <Trash2 size={15} />
        </button>
      </div>
    </article>
  );
}

function PersonaEditor({
  persona,
  onSave,
}: {
  persona: Persona;
  onSave: (value: Persona) => Promise<void>;
}) {
  const [value, setValue] = useState(persona);
  const [saving, setSaving] = useState(false);
  return (
    <section className="page-content persona-page">
      <div className="eyebrow">A PERSONAL KIND OF CLOSE</div>
      <h1>
        当前伙伴 · {persona.name}
        <span>这份设定只属于这个 TA。</span>
      </h1>
      <p className="page-description">
        调整表达风格。人格独立于对话，安全与关系边界始终保留。
      </p>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          setSaving(true);
          void onSave(value).finally(() => setSaving(false));
        }}
      >
        <div className="persona-identity">
          <div className="avatar large">{persona?.name.slice(-1)}</div>
          <div>
            <label>
              TA 的名字
              <input
                required
                maxLength={24}
                value={value.name}
                onChange={(event) =>
                  setValue({ ...value, name: event.target.value })
                }
              />
            </label>
            <label>
              希望 TA 怎样称呼你
              <input
                required
                maxLength={24}
                value={value.user_name}
                onChange={(event) =>
                  setValue({ ...value, user_name: event.target.value })
                }
              />
            </label>
          </div>
        </div>
        <div className="character-fields">
          <label>
            关系定位
            <select
              value={value.relationship}
              onChange={(event) =>
                setValue({
                  ...value,
                  relationship: event.target.value as Persona["relationship"],
                })
              }
            >
              {Object.entries(relationshipNames).map(([id, name]) => (
                <option key={id} value={id}>
                  {name}
                </option>
              ))}
            </select>
          </label>
          <label>
            角色背景与性格
            <textarea
              maxLength={2000}
              value={value.description}
              onChange={(event) =>
                setValue({ ...value, description: event.target.value })
              }
            />
          </label>
          <label>
            开场白
            <input
              maxLength={500}
              value={value.greeting}
              onChange={(event) =>
                setValue({ ...value, greeting: event.target.value })
              }
            />
          </label>
          <label>
            专属音色 ID
            <input
              maxLength={100}
              value={value.voice_id}
              placeholder="留空使用服务端默认音色"
              onChange={(event) =>
                setValue({ ...value, voice_id: event.target.value })
              }
            />
          </label>
        </div>
        {(
          [
            {
              key: "warmth",
              title: "亲近一点",
              left: "克制自然",
              right: "温柔亲密",
            },
            {
              key: "brevity",
              title: "说话的节奏",
              left: "细细展开",
              right: "简洁轻盈",
            },
            {
              key: "humor",
              title: "一点小趣味",
              left: "安静认真",
              right: "轻松俏皮",
            },
          ] as const
        ).map((trait) => (
          <div className="trait" key={trait.key}>
            <label htmlFor={trait.key}>
              {trait.title}
              <span>{Math.round(value[trait.key] * 100)}%</span>
            </label>
            <input
              id={trait.key}
              type="range"
              min="0"
              max="1"
              step="0.05"
              value={value[trait.key]}
              onChange={(event) =>
                setValue({ ...value, [trait.key]: Number(event.target.value) })
              }
            />
            <div>
              <span>{trait.left}</span>
              <span>{trait.right}</span>
            </div>
          </div>
        ))}
        <div className="persona-boundary">
          <Sparkles size={18} />
          <p>亲密无需占有。TA 会尊重你的独处、现实生活与每一次「不」。</p>
        </div>
        <button className="primary-button" disabled={saving}>
          {saving ? "保存中…" : "保存这份默契"}
          <Check size={17} />
        </button>
      </form>
    </section>
  );
}
