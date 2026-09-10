import { mount, flushPromises } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import TrackEditor from "./TrackEditor.vue";
import PlaylistEditor from "./PlaylistEditor.vue";
import Music from "../../views/Music.vue";
import { ApiClientError } from "../../api/errors";
import { musicApi, type MusicTrack, type MusicPlaylist } from "../../api/music";

vi.mock("../../api/music", async (original) => {
  const mod = await original<typeof import("../../api/music")>();
  return {
    ...mod,
    musicApi: {
      catalog: vi.fn(),
      createTrack: vi.fn(),
      updateTrack: vi.fn(),
      deleteTrack: vi.fn(),
      createPlaylist: vi.fn(),
      updatePlaylist: vi.fn(),
      deletePlaylist: vi.fn(),
    },
  };
});
const tracks: MusicTrack[] = [1, 2].map((id) => ({
  id,
  title: `歌曲${id}`,
  artist: "作者",
  audio_url: `https://cdn.test/${id}.mp3`,
  cover_url: "",
  duration: 30,
  revision: 4,
}));
const playlist: MusicPlaylist = {
  id: 2,
  name: "夜晚",
  cover_url: "",
  is_public: true,
  is_default: true,
  track_ids: [1, 2],
  revision: 5,
};
const global = {
  stubs: {
    MusicUpload: { template: "<div />" },
    UiDialog: {
      props: ["open"],
      template: '<div v-if="open"><slot /><slot name="actions" /></div>',
    },
  },
};
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(musicApi.catalog).mockResolvedValue({
    tracks,
    playlists: [playlist],
  });
});
describe("music management", () => {
  it("saves reordered songs and public/default coherence with the current revision", async () => {
    const wrapper = mount(PlaylistEditor, {
      props: { playlist, tracks },
      global,
    });
    await wrapper.get('[aria-label="下移 歌曲1"]').trigger("click");
    await wrapper.get('[aria-label="公开歌单"]').setValue(false);
    expect(
      (wrapper.get('[aria-label="设为默认歌单"]').element as HTMLInputElement)
        .checked,
    ).toBe(false);
    await wrapper.get('[aria-label="设为默认歌单"]').setValue(true);
    expect(
      (wrapper.get('[aria-label="公开歌单"]').element as HTMLInputElement)
        .checked,
    ).toBe(true);
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(musicApi.updatePlaylist).toHaveBeenCalledWith(2, {
      name: "夜晚",
      is_public: true,
      is_default: true,
      track_ids: [2, 1],
      revision: 5,
    });
    expect(wrapper.emitted("saved")).toHaveLength(1);
    wrapper.unmount();
  });
  it("retains song input on conflict and prevents stale resubmission", async () => {
    vi.mocked(musicApi.updateTrack).mockRejectedValue(
      new ApiClientError({
        code: "CONFLICT",
        message: "conflict",
        status: 409,
      }),
    );
    const wrapper = mount(TrackEditor, { props: { track: tracks[0] }, global });
    await wrapper.get('[aria-label="歌曲名称"]').setValue("保留我的修改");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(wrapper.get('[role="alert"]').text()).toContain("其他窗口修改");
    expect(
      (wrapper.get('[aria-label="歌曲名称"]').element as HTMLInputElement)
        .value,
    ).toBe("保留我的修改");
    await wrapper.get("form").trigger("submit");
    await flushPromises();
    expect(musicApi.updateTrack).toHaveBeenCalledTimes(1);
    expect(wrapper.emitted("saved")).toBeUndefined();
    wrapper.unmount();
  });
  it("requires an explicit management action and confirmation before deletion", async () => {
    const wrapper = mount(Music, { global });
    await flushPromises();
    expect(wrapper.find('[aria-label="删除 歌曲1"]').exists()).toBe(false);
    await wrapper
      .findAll("button")
      .find((button) => button.text() === "管理")!
      .trigger("click");
    await wrapper.get('[aria-label="删除 歌曲1"]').trigger("click");
    expect(musicApi.deleteTrack).not.toHaveBeenCalled();
    const confirm = wrapper
      .findAll("button")
      .find((button) => button.text() === "确认删除")!;
    await confirm.trigger("click");
    await flushPromises();
    expect(musicApi.deleteTrack).toHaveBeenCalledExactlyOnceWith(1, 4);
    expect(musicApi.catalog).toHaveBeenCalledTimes(2);
    wrapper.unmount();
  });
});
