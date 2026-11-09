import { useCallback, useEffect, useState } from "react";
import { ArrowRight, Download, Plus, Save, Upload, Users } from "lucide-react";
import { api } from "./api";
import type { CharacterCard, Persona } from "./types";

export const relationshipNames = {
  lover: "虚拟爱人",
  family: "虚拟亲人",
  friend: "虚拟朋友",
  custom: "自定义关系",
};

export function CompanionLibrary({
  active,
  disabled,
  onSelect,
  onError,
}: {
  active: Persona;
  disabled: boolean;
  onSelect: (id: string) => Promise<void>;
  onError: (message: string) => void;
}) {
  const [companions, setCompanions] = useState<Persona[]>([]);
  const [cards, setCards] = useState<CharacterCard[]>([]);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [cardID, setCardID] = useState("");
  const [busy, setBusy] = useState(false);
  const load = useCallback(async () => {
    const [people, templates] = await Promise.all([
      api<Persona[]>("/companions"),
      api<CharacterCard[]>("/character-cards"),
    ]);
    setCompanions(people);
    setCards(templates);
  }, []);
  useEffect(() => {
    void load().catch((error) => onError(String(error)));
  }, [active.id, active.version, load, onError]);
  async function importCard(file?: File) {
    if (!file) return;
    if (file.size > 32000) {
      onError("角色卡 JSON 请控制在 32KB 以内");
      return;
    }
    try {
      const value = JSON.parse(await file.text()) as unknown;
      await api("/character-cards", "POST", value);
      await load();
    } catch (error) {
      onError(String(error));
    }
  }
  return (
    <div className="companion-library">
      <div className="eyebrow">DIFFERENT PEOPLE, DIFFERENT STORIES</div>
      <div className="library-heading">
        <div>
          <h1>我的 AI 伙伴</h1>
          <p>每个 TA 都有自己的角色卡、对话、记忆与陪伴历程。</p>
        </div>
        <button
          disabled={disabled}
          className="library-add"
          onClick={() => setAdding(!adding)}
        >
          <Plus size={16} />
          添加伙伴
        </button>
      </div>
      <div className="companion-grid">
        {companions.map((companion) => (
          <article
            className={`companion-card ${companion.id === active.id ? "chosen" : ""}`}
            key={companion.id}
          >
            <div className="avatar small">{companion.name.slice(-1)}</div>
            <div>
              <span className="relationship-label">
                {relationshipNames[companion.relationship]}
              </span>
              <h2>{companion.name}</h2>
              <p>{companion.description}</p>
              <button
                disabled={disabled || busy}
                onClick={() => {
                  setBusy(true);
                  void onSelect(companion.id)
                    .catch((error) => onError(String(error)))
                    .finally(() => setBusy(false));
                }}
              >
                {companion.id === active.id ? "进入 TA 的空间" : "和 TA 聊聊"}
                <ArrowRight size={14} />
              </button>
            </div>
          </article>
        ))}
      </div>
      {adding && (
        <form
          className="create-companion"
          onSubmit={(event) => {
            event.preventDefault();
            setBusy(true);
            void api<Persona>("/companions", "POST", { name, card_id: cardID })
              .then(async (companion) => {
                await load();
                setAdding(false);
                setName("");
                await onSelect(companion.id);
              })
              .catch((error) => onError(String(error)))
              .finally(() => setBusy(false));
          }}
        >
          <h3>
            <Users size={17} />
            认识一个新的 TA
          </h3>
          <label>
            伙伴名字
            <input
              required
              maxLength={24}
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="比如：林舟"
            />
          </label>
          <label>
            使用的角色卡模板
            <select
              aria-label="使用的角色卡模板"
              required
              value={cardID}
              onChange={(event) => setCardID(event.target.value)}
            >
              <option value="">选择一张角色卡</option>
              {cards.map((card) => (
                <option key={card.id} value={card.id}>
                  {card.definition.name} ·{" "}
                  {relationshipNames[card.definition.relationship]}
                </option>
              ))}
            </select>
          </label>
          <p>
            会复制这张卡的设定。之后修改这个伙伴，不会影响使用同一模板的其他伙伴。
          </p>
          <button className="primary-button" disabled={disabled || busy}>
            创建独立伙伴
            <Plus size={16} />
          </button>
        </form>
      )}
      <div className="card-template-tools">
        <button
          disabled={disabled || busy}
          onClick={() =>
            void api("/character-cards", "POST", {
              schema_version: "companion.character.v1",
              definition: active,
              version: 1,
            })
              .then(load)
              .catch((error) => onError(String(error)))
          }
        >
          <Save size={14} />
          将当前设定存为角色卡
        </button>
        <button
          onClick={() => {
            const blob = new Blob(
              [
                JSON.stringify(
                  {
                    schema_version: "companion.character.v1",
                    definition: { ...active, id: "", card_id: "" },
                    version: 1,
                  },
                  null,
                  2,
                ),
              ],
              { type: "application/json" },
            );
            const url = URL.createObjectURL(blob);
            const anchor = document.createElement("a");
            anchor.href = url;
            anchor.download = `${active.name}-角色卡.json`;
            anchor.click();
            URL.revokeObjectURL(url);
          }}
        >
          <Download size={14} />
          导出角色卡
        </button>
        <label>
          <Upload size={14} />
          导入 JSON 角色卡
          <input
            aria-label="导入 JSON 角色卡"
            type="file"
            accept=".json,application/json"
            onChange={(event) => {
              void importCard(event.target.files?.[0]);
              event.target.value = "";
            }}
          />
        </label>
      </div>
    </div>
  );
}
