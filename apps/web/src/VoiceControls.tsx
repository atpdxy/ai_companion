import { useEffect, useRef, useState } from "react";
import { Mic, Square, Volume2 } from "lucide-react";
import { api } from "./api";

export function VoiceRecorder({
  companionID,
  disabled,
  onText,
  onError,
}: {
  companionID: string;
  disabled: boolean;
  onText: (text: string) => void;
  onError: (message: string) => void;
}) {
  const [recording, setRecording] = useState(false);
  const [busy, setBusy] = useState(false);
  const recorder = useRef<MediaRecorder | null>(null);
  const stream = useRef<MediaStream | null>(null);
  const timer = useRef<number | undefined>(undefined);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      if (recorder.current?.state === "recording") recorder.current.stop();
      stream.current?.getTracks().forEach((track) => track.stop());
      window.clearTimeout(timer.current);
    };
  }, []);
  async function start() {
    if (
      !navigator.mediaDevices?.getUserMedia ||
      typeof MediaRecorder === "undefined"
    ) {
      onError(
        "当前浏览器不支持录音，请使用 HTTPS 或 localhost 上的现代浏览器。",
      );
      return;
    }
    try {
      const microphone = await navigator.mediaDevices.getUserMedia({
        audio: true,
      });
      if (!mounted.current) {
        microphone.getTracks().forEach((track) => track.stop());
        return;
      }
      stream.current = microphone;
      const mime = [
        "audio/webm;codecs=opus",
        "audio/ogg;codecs=opus",
        "audio/mp4",
      ].find((type) => MediaRecorder.isTypeSupported(type));
      const media = new MediaRecorder(
        microphone,
        mime ? { mimeType: mime } : undefined,
      );
      recorder.current = media;
      const chunks: Blob[] = [];
      media.ondataavailable = (event) => {
        if (event.data.size) chunks.push(event.data);
      };
      media.onstop = () => {
        window.clearTimeout(timer.current);
        microphone.getTracks().forEach((track) => track.stop());
        if (!mounted.current) return;
        setRecording(false);
        setBusy(true);
        const blob = new Blob(chunks, { type: media.mimeType });
        if (blob.size > 10 * 1024 * 1024) {
          onError("录音超过 10MB，请缩短录音。");
          setBusy(false);
          return;
        }
        void blob
          .arrayBuffer()
          .then((buffer) => {
            let binary = "";
            const bytes = new Uint8Array(buffer);
            for (let offset = 0; offset < bytes.length; offset += 8192)
              binary += String.fromCharCode(
                ...bytes.subarray(offset, offset + 8192),
              );
            const format = media.mimeType.includes("ogg")
              ? "ogg"
              : media.mimeType.includes("mp4")
                ? "mp4"
                : "webm";
            return api<{ text: string }>("/audio/transcriptions", "POST", {
              companion_id: companionID,
              audio_base64: btoa(binary),
              format,
            });
          })
          .then((result) => {
            if (mounted.current) onText(result.text);
          })
          .catch((error) => {
            if (mounted.current) onError(String(error));
          })
          .finally(() => {
            if (mounted.current) setBusy(false);
          });
      };
      media.start();
      setRecording(true);
      timer.current = window.setTimeout(() => {
        if (media.state === "recording") media.stop();
      }, 60000);
    } catch (error) {
      onError(String(error));
      stream.current?.getTracks().forEach((track) => track.stop());
    }
  }
  return (
    <button
      type="button"
      className={`voice-record ${recording ? "recording" : ""}`}
      disabled={(disabled && !recording) || busy}
      title="音频会发给配置的语音识别服务，转写后由你确认发送；原音频不保存。最长录音 60 秒。"
      aria-label={recording ? "结束录音并转文字" : "录音转文字"}
      onClick={() => {
        if (recording) recorder.current?.stop();
        else void start();
      }}
    >
      {recording ? <Square size={14} /> : <Mic size={15} />}
      <span>{busy ? "正在转文字…" : recording ? "结束录音" : "语音输入"}</span>
    </button>
  );
}

export function SpeechPlayer({
  conversationID,
  messageID,
  onError,
}: {
  conversationID: string;
  messageID: string;
  onError: (message: string) => void;
}) {
  const [busy, setBusy] = useState(false);
  const [playing, setPlaying] = useState(false);
  const audio = useRef<HTMLAudioElement | null>(null);
  const objectURL = useRef<string | null>(null);
  const controller = useRef<AbortController | null>(null);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      controller.current?.abort();
      audio.current?.pause();
      if (objectURL.current) URL.revokeObjectURL(objectURL.current);
    };
  }, []);
  async function play() {
    if (playing || busy) {
      controller.current?.abort();
      audio.current?.pause();
      setPlaying(false);
      setBusy(false);
      return;
    }
    setBusy(true);
    const abort = new AbortController();
    controller.current = abort;
    try {
      const response = await fetch(
        `/api/v1/conversations/${conversationID}/messages/${messageID}/audio`,
        { method: "POST", credentials: "same-origin", signal: abort.signal },
      );
      if (!response.ok) {
        const error = (await response.json()) as { message: string };
        throw new Error(error.message);
      }
      const blob = await response.blob();
      if (!mounted.current) return;
      if (objectURL.current) URL.revokeObjectURL(objectURL.current);
      const url = URL.createObjectURL(blob);
      objectURL.current = url;
      const player = new Audio(url);
      audio.current = player;
      player.onended = () => {
        if (mounted.current) setPlaying(false);
      };
      player.onerror = () => {
        if (mounted.current) {
          setPlaying(false);
          onError("音频播放失败，请重试。");
        }
      };
      await player.play();
      if (mounted.current) setPlaying(true);
    } catch (error) {
      if (mounted.current && !abort.signal.aborted) onError(String(error));
    } finally {
      if (mounted.current) setBusy(false);
    }
  }
  return (
    <button
      className="speech-button"
      aria-label={playing || busy ? "停止朗读" : "朗读这条回复"}
      onClick={() => void play()}
    >
      {playing || busy ? <Square size={12} /> : <Volume2 size={13} />}{" "}
      {busy ? "正在合成…" : playing ? "停止朗读" : "听 TA 说"}
    </button>
  );
}
