import { request } from "./client";
import { isApiClientError, toUserMessage } from "./errors";

export interface MusicTrack {
  id: number;
  title: string;
  artist: string;
  audio_url: string;
  cover_url: string;
  duration: number;
  revision: number;
}
export interface MusicPlaylist {
  id: number;
  name: string;
  cover_url: string;
  is_public: boolean;
  is_default: boolean;
  track_ids: number[];
  revision: number;
}
export interface MusicCatalog {
  tracks: MusicTrack[];
  playlists: MusicPlaylist[];
}
export interface MusicAsset {
  id: number;
  url: string;
  kind: "audio" | "image";
}
export interface TrackInput {
  title: string;
  artist: string;
  audio_asset_id?: number;
  cover_asset_id?: number | null;
  duration: number;
}
export interface PlaylistInput {
  name: string;
  cover_asset_id?: number | null;
  is_public: boolean;
  is_default: boolean;
  track_ids: number[];
}
interface PresignResult {
  object_key: string;
  upload: { url: string; method: string; headers: Record<string, string> };
}
const base = "/admin/music";
export const musicApi = {
  catalog: () => request<MusicCatalog>(base),
  createTrack: (body: TrackInput & { audio_asset_id: number }) =>
    request<MusicTrack>(`${base}/tracks`, { method: "POST", body }),
  updateTrack: (id: number, body: TrackInput & { revision: number }) =>
    request<MusicTrack>(`${base}/tracks/${id}`, { method: "PUT", body }),
  deleteTrack: (id: number, revision: number) =>
    request<void>(`${base}/tracks/${id}`, {
      method: "DELETE",
      query: { revision },
    }),
  createPlaylist: (body: PlaylistInput) =>
    request<MusicPlaylist>(`${base}/playlists`, { method: "POST", body }),
  updatePlaylist: (id: number, body: PlaylistInput & { revision: number }) =>
    request<MusicPlaylist>(`${base}/playlists/${id}`, { method: "PUT", body }),
  deletePlaylist: (id: number, revision: number) =>
    request<void>(`${base}/playlists/${id}`, {
      method: "DELETE",
      query: { revision },
    }),
};

const audioTypes: Record<string, string> = {
  mp3: "audio/mpeg",
  m4a: "audio/mp4",
  ogg: "audio/ogg",
  wav: "audio/wav",
};
const imageTypes: Record<string, string> = {
  jpg: "image/jpeg",
  jpeg: "image/jpeg",
  png: "image/png",
  webp: "image/webp",
  gif: "image/gif",
};
export function musicFileMime(file: File, kind: MusicAsset["kind"]): string {
  const types = kind === "audio" ? audioTypes : imageTypes;
  const mime = types[file.name.split(".").pop()?.toLowerCase() ?? ""];
  // Browsers use audio/x-m4a and audio/x-wav too; send the API's canonical MIME.
  const aliases: Record<string, string> = {
    "audio/x-m4a": "audio/mp4",
    "audio/x-wav": "audio/wav",
    "audio/vnd.wave": "audio/wav",
  };
  const reported = aliases[file.type] ?? file.type;
  if (
    !mime ||
    (reported && reported !== "application/octet-stream" && reported !== mime)
  ) {
    throw new Error(
      kind === "audio"
        ? "请选择 MP3、M4A、OGG 或 WAV 音频。"
        : "请选择 JPG、PNG、WebP 或 GIF 图片。",
    );
  }
  const limit = kind === "audio" ? 100 : 20;
  if (file.size === 0 || file.size > limit * 1024 * 1024)
    throw new Error(`文件不能为空，且不能超过 ${limit} MB。`);
  return mime;
}

export function musicError(error: unknown): string {
  if (isApiClientError(error)) {
    if (error.status === 409)
      return "内容已在其他窗口修改，请重新载入最新版本后再编辑。当前输入仍保留。";
    if (error.status === 503) return "音乐存储服务暂不可用，请稍后重试。";
    if (error.status === 404) return "这条内容已被移除，请重新载入列表。";
    if (error.status === 400)
      return "内容未通过校验，请检查名称、文件格式和大小后重试。";
    return toUserMessage(error);
  }
  return error instanceof Error ? error.message : "操作未完成，请重试。";
}

/** Each manual retry gets a fresh key, including after an uncertain registration response. */
export async function uploadMusicFile(
  file: File,
  kind: MusicAsset["kind"],
  onProgress: (percent: number) => void,
  signal?: AbortSignal,
): Promise<MusicAsset> {
  const mime_type = musicFileMime(file, kind);
  const size_bytes = file.size;
  signal?.throwIfAborted();
  const signed = await request<PresignResult>(`${base}/uploads/presign`, {
    method: "POST",
    body: { filename: file.name, mime_type, size_bytes },
    ...(signal ? { signal } : {}),
  });
  signal?.throwIfAborted();
  await new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    const abort = () => xhr.abort();
    const finish = (error?: Error) => {
      signal?.removeEventListener("abort", abort);
      if (error) reject(error);
      else resolve();
    };
    xhr.open(signed.upload.method, signed.upload.url);
    xhr.timeout = 10 * 60 * 1000;
    Object.entries(signed.upload.headers).forEach(([name, value]) =>
      xhr.setRequestHeader(name, value),
    );
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable)
        onProgress(Math.min(95, Math.round((event.loaded / event.total) * 95)));
    };
    xhr.onload = () =>
      finish(
        xhr.status >= 200 && xhr.status < 300
          ? undefined
          : new Error("文件上传失败，请重试。"),
      );
    xhr.onerror = () => finish(new Error("上传连接中断，请检查网络后重试。"));
    xhr.ontimeout = () => finish(new Error("上传超时，请重试。"));
    xhr.onabort = () => finish(new DOMException("上传已取消", "AbortError"));
    signal?.addEventListener("abort", abort, { once: true });
    xhr.send(file);
  });
  signal?.throwIfAborted();
  const asset = await request<MusicAsset>(`${base}/uploads`, {
    method: "POST",
    body: { object_key: signed.object_key, mime_type, size_bytes },
    ...(signal ? { signal } : {}),
  });
  onProgress(100);
  return asset;
}
