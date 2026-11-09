import { useEffect, useState } from "react";
import { Plug, Play } from "lucide-react";
import { api } from "./api";

interface Tool {
  id: string;
  description: string;
  input_schema: Record<string, unknown>;
}
export function ToolPanel({
  companionID,
  onError,
}: {
  companionID: string;
  onError: (message: string) => void;
}) {
  const [tools, setTools] = useState<Tool[]>([]);
  const [selected, setSelected] = useState("");
  const [argumentsJSON, setArgumentsJSON] = useState("{}");
  const [result, setResult] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let mounted = true;
    void api<Tool[]>(`/integrations/tools?companion_id=${companionID}`)
      .then((value) => {
        if (mounted) {
          setTools(value);
          setSelected("");
          setResult("");
        }
      })
      .catch((error) => {
        if (mounted) onError(String(error));
      });
    return () => {
      mounted = false;
    };
  }, [companionID, onError]);
  return (
    <details className="tool-panel">
      <summary>
        <Plug size={16} />
        扩展工具与数据源
      </summary>
      <p>
        仅显示管理员为当前账号授权的只读工具。每次调用由你发起，外部结果不会变成系统指令。
      </p>
      {tools.length === 0 ? (
        <p>当前没有已授权的外部工具。</p>
      ) : (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            try {
              const args: unknown = JSON.parse(argumentsJSON);
              if (!args || typeof args !== "object" || Array.isArray(args))
                throw new Error("参数必须为 JSON 对象");
              setBusy(true);
              void api("/integrations/tools/invoke", "POST", {
                companion_id: companionID,
                tool_id: selected,
                arguments: args,
              })
                .then((value) => setResult(JSON.stringify(value, null, 2)))
                .catch((error) => onError(String(error)))
                .finally(() => setBusy(false));
            } catch (error) {
              onError(String(error));
            }
          }}
        >
          <label>
            工具
            <select
              aria-label="扩展工具"
              required
              value={selected}
              onChange={(event) => setSelected(event.target.value)}
            >
              <option value="">选择工具</option>
              {tools.map((tool) => (
                <option key={tool.id} value={tool.id}>
                  {tool.id}
                </option>
              ))}
            </select>
          </label>
          {selected && (
            <p>{tools.find((tool) => tool.id === selected)?.description}</p>
          )}
          <label>
            调用参数（JSON）
            <textarea
              aria-label="工具调用参数"
              maxLength={16000}
              value={argumentsJSON}
              onChange={(event) => setArgumentsJSON(event.target.value)}
            />
          </label>
          <button disabled={busy || !selected}>
            <Play size={13} />
            {busy ? "调用中…" : "调用这个只读工具"}
          </button>
        </form>
      )}
      {result && <pre>{result}</pre>}
    </details>
  );
}
