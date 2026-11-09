interface Envelope<T> {
  code: string;
  message: string;
  data: T;
  request_id: string;
}

export class APIError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

export async function api<T>(
  path: string,
  method = "GET",
  body?: unknown,
): Promise<T> {
  const response = await fetch(`/api/v1${path}`, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const result = (await response.json()) as Envelope<T>;
  if (!response.ok)
    throw new APIError(
      response.status,
      `${result.message}（${result.request_id}）`,
    );
  return result.data;
}

export async function streamMessage(
  conversation: string,
  content: string,
  turn: string,
  signal: AbortSignal,
  receive: (event: string, data: Record<string, unknown>) => void,
): Promise<void> {
  const response = await fetch(
    `/api/v1/conversations/${conversation}/messages`,
    {
      method: "POST",
      credentials: "same-origin",
      headers: {
        "Content-Type": "application/json",
        Accept: "text/event-stream",
      },
      body: JSON.stringify({ content, turn_id: turn }),
      signal,
    },
  );
  if (!response.ok) {
    const error = (await response.json()) as Envelope<unknown>;
    throw new APIError(response.status, error.message);
  }
  if (!response.body) throw new Error("浏览器不支持流式回复");
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let completed = false;
  try {
    while (true) {
      const { done, value } = await reader.read();
      buffer += decoder.decode(value, { stream: !done }).replace(/\r\n/g, "\n");
      let boundary: number;
      while ((boundary = buffer.indexOf("\n\n")) >= 0) {
        const block = buffer.slice(0, boundary);
        buffer = buffer.slice(boundary + 2);
        const lines = block.split("\n");
        const event = lines
          .find((line) => line.startsWith("event:"))
          ?.slice(6)
          .trim();
        const raw = lines
          .filter((line) => line.startsWith("data:"))
          .map((line) => line.slice(5).trim())
          .join("\n");
        if (!event || !raw) continue;
        const data = JSON.parse(raw) as Record<string, unknown>;
        if (event === "error")
          throw new Error(String(data.message ?? "回复中断"));
        if (event === "message_end") completed = true;
        receive(event, data);
      }
      if (done) break;
    }
    if (!completed) throw new Error("回复中断，请重试此轮");
  } finally {
    await reader.cancel().catch(() => undefined);
    reader.releaseLock();
  }
}
