import { useCallback, useEffect, useState } from "react";
import { Bell, Check, Clock, Trash2 } from "lucide-react";
import { api } from "./api";

interface Preferences {
  opt_in: boolean;
  no_contact: boolean;
  quiet_start: number;
  quiet_end: number;
  daily_limit: number;
  timezone: string;
}
interface ScheduledEvent {
  id: string;
  content: string;
  due_at: string;
  recurrence: string;
  status: string;
}
interface Notification {
  id: string;
  content: string;
  delivered_at: string;
}

export function NotificationSettings({
  onError,
}: {
  onError: (message: string) => void;
}) {
  const [preferences, setPreferences] = useState<Preferences | null>(null);
  const [events, setEvents] = useState<ScheduledEvent[]>([]);
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [content, setContent] = useState("");
  const [due, setDue] = useState("");
  const [recurrence, setRecurrence] = useState("none");
  const [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    const [prefs, scheduled, delivered] = await Promise.all([
      api<Preferences>("/notification-preferences"),
      api<ScheduledEvent[]>("/events"),
      api<Notification[]>("/notifications"),
    ]);
    setPreferences(prefs);
    setEvents(scheduled);
    setNotifications(delivered);
  }, []);
  useEffect(() => {
    let active = true;
    const refresh = () => {
      if (active) void load().catch((error) => onError(String(error)));
    };
    refresh();
    const timer = window.setInterval(refresh, 30000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [load, onError]);
  async function save(value: Preferences) {
    setBusy(true);
    try {
      await api("/notification-preferences", "PUT", value);
      await load();
    } catch (error) {
      onError(String(error));
    } finally {
      setBusy(false);
    }
  }
  if (!preferences) return <p>正在读取提醒偏好…</p>;
  return (
    <div className="reminder-settings">
      <div className="setting-block">
        <Bell />
        <div className="reminder-body">
          <h3>只在你约定的时候</h3>
          <p>
            默认关闭。提醒只在站内送达，不发邮件或系统推送。每 10
            秒检查到期任务，免打扰期间顺延；过期即取消。
          </p>
          <div className="reminder-toggle">
            <span>允许站内提醒</span>
            <button
              disabled={busy}
              role="switch"
              aria-label="允许站内提醒"
              aria-checked={preferences.opt_in}
              className={`toggle ${preferences.opt_in ? "on" : ""}`}
              onClick={() =>
                void save({
                  ...preferences,
                  opt_in: !preferences.opt_in,
                  no_contact: false,
                })
              }
            >
              <span />
            </button>
          </div>
          <div className="reminder-preferences">
            <label>
              免打扰开始
              <select
                aria-label="免打扰开始"
                disabled={busy}
                value={preferences.quiet_start}
                onChange={(event) =>
                  void save({
                    ...preferences,
                    quiet_start: Number(event.target.value),
                  })
                }
              >
                {Array.from({ length: 24 }, (_, hour) => (
                  <option key={hour} value={hour}>
                    {hour}:00
                  </option>
                ))}
              </select>
            </label>
            <label>
              免打扰结束
              <select
                aria-label="免打扰结束"
                disabled={busy}
                value={preferences.quiet_end}
                onChange={(event) =>
                  void save({
                    ...preferences,
                    quiet_end: Number(event.target.value),
                  })
                }
              >
                {Array.from({ length: 24 }, (_, hour) => (
                  <option key={hour} value={hour}>
                    {hour}:00
                  </option>
                ))}
              </select>
            </label>
            <label>
              每天最多
              <select
                aria-label="每日提醒上限"
                disabled={busy}
                value={preferences.daily_limit}
                onChange={(event) =>
                  void save({
                    ...preferences,
                    daily_limit: Number(event.target.value),
                  })
                }
              >
                {Array.from({ length: 10 }, (_, index) => (
                  <option key={index} value={index + 1}>
                    {index + 1} 条
                  </option>
                ))}
              </select>
            </label>
          </div>
          <p>
            免打扰时区：{preferences.timezone}
            。起止相同表示不设免打扰。在对话中说「别联系我」也会关闭授权。
          </p>
        </div>
      </div>
      {preferences.opt_in && (
        <form
          className="reminder-form"
          onSubmit={(event) => {
            event.preventDefault();
            setBusy(true);
            void api("/events", "POST", {
              content,
              due_at: new Date(due).toISOString(),
              recurrence,
              client_key: crypto.randomUUID(),
            })
              .then(async () => {
                setContent("");
                setDue("");
                await load();
              })
              .catch((error) => onError(String(error)))
              .finally(() => setBusy(false));
          }}
        >
          <h3>
            <Clock size={16} /> 约定一个提醒
          </h3>
          <label>
            提醒内容
            <input
              required
              maxLength={500}
              placeholder="比如：出门前带上面试材料"
              value={content}
              onChange={(event) => setContent(event.target.value)}
            />
          </label>
          <div className="reminder-preferences">
            <label>
              时间（你的浏览器本地时区）
              <input
                aria-label="提醒时间"
                required
                type="datetime-local"
                value={due}
                onChange={(event) => setDue(event.target.value)}
              />
            </label>
            <label>
              重复
              <select
                value={recurrence}
                onChange={(event) => setRecurrence(event.target.value)}
              >
                <option value="none">仅一次</option>
                <option value="daily">每天</option>
                <option value="weekly">每周</option>
              </select>
            </label>
          </div>
          <p>默认在首次约定后 7 天过期。你可以随时撤销。</p>
          <button className="primary-button" disabled={busy}>
            确认创建提醒 <Check size={16} />
          </button>
        </form>
      )}
      {events.length > 0 && (
        <div className="scheduled-list">
          <h3>我的约定</h3>
          {events.map((event) => (
            <div key={event.id}>
              <div>
                <strong>{event.content}</strong>
                <p>
                  {new Date(event.due_at).toLocaleString("zh-CN")} ·{" "}
                  {event.recurrence === "daily"
                    ? "每天"
                    : event.recurrence === "weekly"
                      ? "每周"
                      : "仅一次"}{" "}
                  ·{" "}
                  {
                    {
                      pending: "等待到期",
                      delivered: "已送达",
                      cancelled: "已撤销",
                      expired: "已过期",
                    }[event.status]
                  }
                </p>
              </div>
              {event.status === "pending" && (
                <button
                  aria-label="撤销提醒"
                  onClick={() =>
                    void api(`/events/${event.id}`, "DELETE")
                      .then(load)
                      .catch((error) => onError(String(error)))
                  }
                >
                  <Trash2 size={16} />
                </button>
              )}
            </div>
          ))}
        </div>
      )}
      {notifications.length > 0 && (
        <div className="scheduled-list delivered-list">
          <h3>给你的提醒</h3>
          {notifications.map((notification) => (
            <div key={notification.id}>
              <Bell size={15} />
              <div>
                <strong>{notification.content}</strong>
                <p>
                  {new Date(notification.delivered_at).toLocaleString("zh-CN")}
                </p>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
