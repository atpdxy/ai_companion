export interface User {
  id: string;
  email: string;
  memory_consent: boolean;
}
export interface Persona {
  id: string;
  card_id: string;
  relationship: "lover" | "family" | "friend" | "custom";
  description: string;
  greeting: string;
  voice_id: string;
  name: string;
  user_name: string;
  warmth: number;
  brevity: number;
  humor: number;
  version: number;
}
export interface Conversation {
  companion_id: string;
  id: string;
  title: string;
  created_at: string;
}
export interface Message {
  id: string;
  turn_id: string;
  role: "user" | "assistant";
  content: string;
  created_at: string;
  incomplete?: boolean;
}
export interface Memory {
  companion_id: string;
  id: string;
  content: string;
  type: string;
  consent_status: "pending" | "approved" | "rejected";
  source_message_id: string;
  source_quote: string;
  confidence: number;
  expires_at: string | null;
  created_at: string;
}

export interface CharacterCard {
  id?: string;
  schema_version: string;
  definition: Persona;
  version: number;
}
export interface Capabilities {
  mode: string;
  asr_enabled: boolean;
  tts_enabled: boolean;
}
export interface AgentResult {
  turn_id: string;
  response: string;
  mode: "live" | "demo";
  strategy: { action: string; reason_code: string };
  emotion?: { label: string; confidence: number };
  latency_ms: number;
}
