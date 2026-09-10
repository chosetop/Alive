import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { musicApi, musicError, musicFileMime, uploadMusicFile } from "./music";
import { ApiClientError } from "./errors";

const fetchMock = vi.fn();
class FakeXHR {
  static instances: FakeXHR[] = [];
  upload = {
    onprogress: null as
      | null
      | ((event: {
          lengthComputable: boolean;
          loaded: number;
          total: number;
        }) => void),
  };
  status = 200;
  timeout = 0;
  onload?: () => void;
  onerror?: () => void;
  ontimeout?: () => void;
  onabort?: () => void;
  open = vi.fn();
  setRequestHeader = vi.fn();
  send = vi.fn();
  abort = vi.fn(() => this.onabort?.());
  constructor() {
    FakeXHR.instances.push(this);
  }
}
const json = (data: unknown) =>
  new Response(JSON.stringify({ data }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
const presign = {
  object_key: "music/owner/key",
  upload: {
    url: "https://storage.test/file",
    method: "PUT",
    headers: { "Content-Type": "audio/mpeg" },
  },
};
beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("XMLHttpRequest", FakeXHR);
  fetchMock.mockReset();
  FakeXHR.instances = [];
});
afterEach(() => vi.unstubAllGlobals());

describe("music API contract and upload", () => {
  it("preserves omitted update assets and serializes explicit null plus revision", async () => {
    fetchMock.mockImplementation(() => Promise.resolve(json({ id: 3 })));
    await musicApi.updateTrack(3, {
      title: "雨",
      artist: "",
      duration: 3,
      revision: 8,
    });
    let init = fetchMock.mock.calls[0]![1];
    expect(init.method).toBe("PUT");
    expect(JSON.parse(init.body)).toEqual({
      title: "雨",
      artist: "",
      duration: 3,
      revision: 8,
    });
    await musicApi.updatePlaylist(4, {
      name: "夜",
      is_public: true,
      is_default: true,
      track_ids: [3, 2],
      cover_asset_id: null,
      revision: 9,
    });
    init = fetchMock.mock.calls[1]![1];
    expect(JSON.parse(init.body)).toMatchObject({
      track_ids: [3, 2],
      cover_asset_id: null,
      revision: 9,
    });
  });
  it("handles both 204 deletes with revision query and session credentials", async () => {
    fetchMock.mockImplementation(() =>
      Promise.resolve(new Response(null, { status: 204 })),
    );
    await expect(musicApi.deleteTrack(7, 4)).resolves.toBeUndefined();
    await expect(musicApi.deletePlaylist(9, 6)).resolves.toBeUndefined();
    expect(String(fetchMock.mock.calls[0]![0])).toBe(
      "http://api.test/api/v1/admin/music/tracks/7?revision=4",
    );
    expect(String(fetchMock.mock.calls[1]![0])).toBe(
      "http://api.test/api/v1/admin/music/playlists/9?revision=6",
    );
    expect(fetchMock.mock.calls[0]![1]).toMatchObject({
      method: "DELETE",
      credentials: "include",
    });
  });
  it("rejects empty, oversized, mismatched and unsupported files before presign", async () => {
    expect(() => musicFileMime(new File([], "empty.mp3"), "audio")).toThrow(
      "不能为空",
    );
    expect(() =>
      musicFileMime(
        new File(["a"], "bad.svg", { type: "image/svg+xml" }),
        "image",
      ),
    ).toThrow("图片");
    expect(() =>
      musicFileMime(new File(["a"], "bad.mp3", { type: "image/png" }), "audio"),
    ).toThrow("音频");
    const large = new File(["x"], "large.wav", { type: "audio/wav" });
    Object.defineProperty(large, "size", { value: 100 * 1024 * 1024 + 1 });
    await expect(uploadMusicFile(large, "audio", vi.fn())).rejects.toThrow(
      "100 MB",
    );
    expect(fetchMock).not.toHaveBeenCalled();
    expect(
      musicFileMime(
        new File(["x"], "track.M4A", { type: "audio/x-m4a" }),
        "audio",
      ),
    ).toBe("audio/mp4");
    expect(
      musicFileMime(
        new File(["x"], "track.wav", { type: "audio/x-wav" }),
        "audio",
      ),
    ).toBe("audio/wav");
  });
  it("reports progress, uses signed headers and registers only after successful PUT", async () => {
    fetchMock
      .mockResolvedValueOnce(json(presign))
      .mockResolvedValueOnce(json({ id: 8, kind: "audio", url: "/audio.mp3" }));
    const file = new File(["test"], "song.mp3", { type: "audio/mpeg" });
    const progress = vi.fn();
    const pending = uploadMusicFile(file, "audio", progress);
    await vi.waitFor(() => expect(FakeXHR.instances).toHaveLength(1));
    const xhr = FakeXHR.instances[0]!;
    expect(xhr.open).toHaveBeenCalledWith("PUT", presign.upload.url);
    expect(xhr.setRequestHeader).toHaveBeenCalledWith(
      "Content-Type",
      "audio/mpeg",
    );
    expect(xhr.send).toHaveBeenCalledWith(file);
    xhr.upload.onprogress?.({ lengthComputable: true, loaded: 2, total: 4 });
    expect(progress).toHaveBeenCalledWith(48);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    xhr.onload?.();
    await expect(pending).resolves.toMatchObject({ id: 8 });
    expect(JSON.parse(fetchMock.mock.calls[1]![1].body)).toEqual({
      object_key: presign.object_key,
      mime_type: "audio/mpeg",
      size_bytes: 4,
    });
    expect(progress).toHaveBeenLastCalledWith(100);
  });
  it("does not register a failed transfer and aborts active XHR on disposal", async () => {
    fetchMock.mockImplementation(() => Promise.resolve(json(presign)));
    const file = new File(["x"], "song.mp3");
    const pending = uploadMusicFile(file, "audio", vi.fn());
    const assertion = expect(pending).rejects.toThrow("上传失败");
    await vi.waitFor(() => expect(FakeXHR.instances).toHaveLength(1));
    FakeXHR.instances[0]!.status = 403;
    FakeXHR.instances[0]!.onload?.();
    await assertion;
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const controller = new AbortController();
    const aborted = uploadMusicFile(file, "audio", vi.fn(), controller.signal);
    const abortAssertion = expect(aborted).rejects.toThrow("取消");
    await vi.waitFor(() => expect(FakeXHR.instances).toHaveLength(2));
    controller.abort();
    await abortAssertion;
    expect(FakeXHR.instances[1]!.abort).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledTimes(2);
  });
  it("translates 400, conflict and storage unavailability without backend details", () => {
    for (const [status, expected] of [
      [400, "校验"],
      [409, "当前输入仍保留"],
      [503, "存储服务"],
    ] as const) {
      expect(
        musicError(
          new ApiClientError({
            code: "INVALID_INPUT",
            status,
            message: "private internal info",
          }),
        ),
      ).toContain(expected);
    }
  });
});
