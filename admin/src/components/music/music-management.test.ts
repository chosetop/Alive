import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent } from "vue";
import { ApiClientError } from "../../api/errors";
import { type MusicTrack } from "../../api/music";
import Music from "../../views/Music.vue";
import TrackEditor from "./TrackEditor.vue";
import PlaylistEditor from "./PlaylistEditor.vue";
import MusicUpload from "./MusicUpload.vue";

const api = vi.hoisted(() => ({
  catalog: vi.fn(),
  createTrack: vi.fn(),
  updateTrack: vi.fn(),
  deleteTrack: vi.fn(),
  createPlaylist: vi.fn(),
  updatePlaylist: vi.fn(),
  deletePlaylist: vi.fn(),
  upload: vi.fn(),
}));
vi.mock("../../api/music", async () => ({
  ...(await vi.importActual<typeof import("../../api/music")>(
    "../../api/music",
  )),
  musicApi: api,
  uploadMusicFile: api.upload,
}));
const first: MusicTrack = {
  id: 1,
  title: "夜雨",
  artist: "某人",
  audio_url: "/rain.mp3",
  cover_url: "/cover.png",
  duration: 61,
  revision: 4,
};
const second: MusicTrack = { ...first, id: 2, title: "清晨", cover_url: "" };
const playlist = {
  id: 7,
  name: "夜间",
  cover_url: "/list.png",
  is_public: true,
  is_default: false,
  track_ids: [1, 2],
  revision: 3,
};
const wrappers: VueWrapper[] = [];
const dialog = defineComponent({
  props: ["open", "title", "description"],
  template:
    '<div v-if="open" role="dialog"><h2>{{title}}</h2><p>{{description}}</p><slot/><slot name="actions"/></div>',
});
function render(component: Parameters<typeof mount>[0], props = {}) {
  const wrapper = mount(component, {
    props,
    global: { stubs: { UiDialog: dialog } },
  });
  wrappers.push(wrapper);
  return wrapper;
}
function button(wrapper: VueWrapper, label: string) {
  const found = wrapper.findAll("button").find((node) => node.text() === label);
  if (!found) throw new Error(`Missing button ${label}`);
  return found;
}
async function choose(wrapper: VueWrapper, file: File) {
  const input = wrapper.get('input[type="file"]');
  Object.defineProperty(input.element, "files", {
    configurable: true,
    value: [file],
  });
  await input.trigger("change");
  await flushPromises();
}
beforeEach(() => {
  vi.resetAllMocks();
  api.catalog.mockResolvedValue({
    tracks: [first, second],
    playlists: [playlist],
  });
});
afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount());
});

describe("music management", () => {
  it("renders loading, retries catalog failure, and provides a useful empty state", async () => {
    api.catalog
      .mockRejectedValueOnce(new Error("读取失败"))
      .mockResolvedValueOnce({ tracks: [], playlists: [] });
    const wrapper = render(Music);
    expect(wrapper.text()).toContain("正在整理音乐");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("读取失败");
    await button(wrapper, "重新加载").trigger("click");
    await flushPromises();
    expect(wrapper.text()).toContain("曲库还是空的");
    await button(wrapper, "添加歌曲").trigger("click");
    expect(wrapper.findComponent(TrackEditor).exists()).toBe(true);
  });
  it("searches tracks and previews only the explicitly selected audio without autoplay", async () => {
    const wrapper = render(Music);
    await flushPromises();
    expect(wrapper.find("audio").exists()).toBe(false);
    await wrapper.get('[aria-label="搜索歌曲或歌手"]').setValue("清晨");
    expect(wrapper.findAll("[data-track]")).toHaveLength(1);
    await wrapper.get('[aria-label="试听 清晨"]').trigger("click");
    expect(wrapper.get("audio").attributes()).toMatchObject({
      src: second.audio_url,
      preload: "none",
    });
    expect(wrapper.get("audio").attributes("autoplay")).toBeUndefined();
    await wrapper.get("audio").trigger("error");
    expect(wrapper.get('[role="alert"]').text()).toContain("无法播放");
    await wrapper.get('[aria-label="关闭试听"]').trigger("click");
    expect(wrapper.find("audio").exists()).toBe(false);
  });
  it("requires management mode and explicit confirmation before revision-aware deletion", async () => {
    const wrapper = render(Music);
    await flushPromises();
    expect(wrapper.find('[aria-label="删除 夜雨"]').exists()).toBe(false);
    await button(wrapper, "管理").trigger("click");
    await wrapper.get('[aria-label="删除 夜雨"]').trigger("click");
    expect(api.deleteTrack).not.toHaveBeenCalled();
    expect(wrapper.get('[role="dialog"]').text()).toContain("1 张歌单");
    await button(wrapper, "取消").trigger("click");
    expect(api.deleteTrack).not.toHaveBeenCalled();
    await wrapper.get('[aria-label="删除 夜雨"]').trigger("click");
    api.deleteTrack.mockResolvedValue(undefined);
    await button(wrapper, "确认删除").trigger("click");
    await flushPromises();
    expect(api.deleteTrack).toHaveBeenCalledExactlyOnceWith(1, 4);
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false);
    expect(api.catalog).toHaveBeenCalledTimes(2);
  });
  it("does not delete tracks when removing a playlist and leaves failed deletion open", async () => {
    const wrapper = render(Music);
    await flushPromises();
    await button(wrapper, "歌单").trigger("click");
    await button(wrapper, "管理").trigger("click");
    await wrapper.get('[aria-label="删除歌单 夜间"]').trigger("click");
    expect(wrapper.get('[role="dialog"]').text()).toContain(
      "曲库中的歌曲仍会保留",
    );
    api.deletePlaylist.mockRejectedValue(new Error("请重试"));
    await button(wrapper, "确认删除").trigger("click");
    await flushPromises();
    expect(api.deletePlaylist).toHaveBeenCalledWith(7, 3);
    expect(api.deleteTrack).not.toHaveBeenCalled();
    expect(wrapper.get('[role="dialog"]').text()).toContain("请重试");
  });
  it("does not resubmit stale deletion revisions and offers reload", async () => {
    api.deleteTrack.mockRejectedValue(
      new ApiClientError({ code: "CONFLICT", status: 409, message: "stale" }),
    );
    const wrapper = render(Music);
    await flushPromises();
    await button(wrapper, "管理").trigger("click");
    await wrapper.get('[aria-label="删除 夜雨"]').trigger("click");
    await button(wrapper, "确认删除").trigger("click");
    await flushPromises();
    expect(
      wrapper.findAll("button").some((item) => item.text() === "确认删除"),
    ).toBe(false);
    await button(wrapper, "重新载入列表").trigger("click");
    await flushPromises();
    expect(api.catalog).toHaveBeenCalledTimes(2);
  });
  it("preserves existing audio/cover on edit and explicitly clears the removed cover", async () => {
    const wrapper = render(TrackEditor, { track: first });
    await wrapper.get('[aria-label="歌曲名称"]').setValue("夜雨新版");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(api.updateTrack).toHaveBeenLastCalledWith(1, {
      title: "夜雨新版",
      artist: "某人",
      duration: 61,
      revision: 4,
    });
    await button(wrapper, "移除封面").trigger("click");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(api.updateTrack).toHaveBeenLastCalledWith(
      1,
      expect.objectContaining({ cover_asset_id: null }),
    );
  });
  it("blocks creation until audio exists and sends uploaded asset ids", async () => {
    const wrapper = render(TrackEditor);
    await wrapper.get('[aria-label="歌曲名称"]').setValue("新歌");
    await wrapper.get("form").trigger("submit");
    expect(wrapper.text()).toContain("请先上传一首音频");
    expect(api.createTrack).not.toHaveBeenCalled();
    const uploads = wrapper.findAllComponents(MusicUpload);
    uploads[0]!.vm.$emit("uploaded", {
      id: 10,
      url: "/new.mp3",
      kind: "audio",
    });
    uploads[1]!.vm.$emit("uploaded", {
      id: 11,
      url: "/new.png",
      kind: "image",
    });
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(api.createTrack).toHaveBeenCalledWith({
      title: "新歌",
      artist: "",
      duration: 0,
      audio_asset_id: 10,
      cover_asset_id: 11,
    });
    expect(wrapper.emitted("saved")).toHaveLength(1);
  });
  it("retains unsaved track input after conflict and requires explicit reload", async () => {
    api.updateTrack.mockRejectedValue(
      new ApiClientError({ code: "CONFLICT", status: 409, message: "stale" }),
    );
    const wrapper = render(TrackEditor, { track: first });
    await wrapper.get('[aria-label="歌曲名称"]').setValue("未保存的标题");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(
      (wrapper.get('[aria-label="歌曲名称"]').element as HTMLInputElement)
        .value,
    ).toBe("未保存的标题");
    expect(button(wrapper, "保存歌曲").attributes("disabled")).toBeDefined();
    await button(wrapper, "放弃当前修改并重新载入").trigger("click");
    expect(wrapper.emitted("reload")).toHaveLength(1);
    expect(wrapper.emitted("saved")).toBeUndefined();
  });
  it("selects songs, moves order, and enforces public/default invariants", async () => {
    const wrapper = render(PlaylistEditor, { tracks: [first, second] });
    await wrapper.get('[aria-label="歌单名称"]').setValue("早晚");
    await wrapper.get('[aria-label="添加 夜雨"]').setValue(true);
    await wrapper.get('[aria-label="添加 清晨"]').setValue(true);
    await wrapper.get('[aria-label="上移 清晨"]').trigger("click");
    expect(
      wrapper
        .findAll("[data-selected-track]")
        .map((item) => item.attributes("data-selected-track")),
    ).toEqual(["2", "1"]);
    await wrapper.get('[aria-label="设为默认歌单"]').setValue(true);
    expect(
      (wrapper.get('[aria-label="公开歌单"]').element as HTMLInputElement)
        .checked,
    ).toBe(true);
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(api.createPlaylist).toHaveBeenCalledWith({
      name: "早晚",
      is_public: true,
      is_default: true,
      track_ids: [2, 1],
      cover_asset_id: null,
    });
    await wrapper.get('[aria-label="公开歌单"]').setValue(false);
    expect(
      (wrapper.get('[aria-label="设为默认歌单"]').element as HTMLInputElement)
        .checked,
    ).toBe(false);
    await wrapper.get('[aria-label="移出 清晨"]').trigger("click");
    expect(wrapper.find('[aria-label="添加 清晨"]').exists()).toBe(true);
  });
  it("updates playlist revisions, preserves cover and blocks duplicate in-flight saves", async () => {
    let finish!: () => void;
    api.updatePlaylist.mockReturnValue(
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
    );
    const wrapper = render(PlaylistEditor, {
      playlist,
      tracks: [first, second],
    });
    await wrapper.get("form").trigger("submit");
    await wrapper.get("form").trigger("submit");
    expect(api.updatePlaylist).toHaveBeenCalledExactlyOnceWith(7, {
      name: "夜间",
      is_public: true,
      is_default: false,
      track_ids: [1, 2],
      revision: 3,
    });
    expect(button(wrapper, "取消").attributes("disabled")).toBeDefined();
    finish();
    await flushPromises();
    expect(wrapper.emitted("saved")).toHaveLength(1);
  });
  it("retains failed files for manual retry and releases the pending guard on success", async () => {
    api.upload
      .mockRejectedValueOnce(new Error("连接断开"))
      .mockImplementationOnce(async (_file, _kind, progress) => {
        progress(65);
        return { id: 9, kind: "audio", url: "/ok.mp3" };
      });
    const wrapper = render(MusicUpload, { kind: "audio", label: "上传音频" });
    const file = new File(["x"], "rain.mp3", { type: "audio/mpeg" });
    await choose(wrapper, file);
    expect(wrapper.text()).toContain("rain.mp3");
    expect(wrapper.text()).toContain("连接断开");
    expect(wrapper.emitted("pending")).toEqual([[true]]);
    await button(wrapper, "重试上传").trigger("click");
    await flushPromises();
    expect(api.upload.mock.calls[1]![0]).toBe(file);
    expect(wrapper.emitted("uploaded")).toEqual([
      [{ id: 9, kind: "audio", url: "/ok.mp3" }],
    ]);
    expect(wrapper.emitted("pending")).toEqual([[true], [false]]);
  });
  it("blocks save for failed replacement uploads until explicitly discarded", async () => {
    api.upload.mockRejectedValue(new Error("连接断开"));
    const wrapper = render(TrackEditor, { track: first });
    await choose(
      wrapper.findComponent(MusicUpload),
      new File(["x"], "replace.mp3"),
    );
    expect(button(wrapper, "保存歌曲").attributes("disabled")).toBeDefined();
    await wrapper.get("form").trigger("submit");
    expect(api.updateTrack).not.toHaveBeenCalled();
    await button(wrapper, "放弃此文件").trigger("click");
    expect(button(wrapper, "保存歌曲").attributes("disabled")).toBeUndefined();
  });
  it("announces progress and aborts upload when its component unmounts", async () => {
    let signal!: AbortSignal;
    api.upload.mockImplementation((_file, _kind, progress, abortSignal) => {
      signal = abortSignal;
      progress(42);
      return new Promise(() => {});
    });
    const wrapper = render(MusicUpload, { kind: "audio", label: "上传音频" });
    await choose(wrapper, new File(["x"], "progress.mp3"));
    expect(wrapper.get("progress").attributes("value")).toBe("42");
    expect(wrapper.text()).toContain("正在上传 42%");
    wrapper.unmount();
    expect(signal.aborted).toBe(true);
  });
});
