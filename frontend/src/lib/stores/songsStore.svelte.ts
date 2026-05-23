import type { ShownCouplet } from "$lib/bindings/changeme";
import { Couplet, Song } from "$lib/bindings/changeme/backend/models";
import {
  GetCouplets,
  GetShownCouplet,
  GetSongs,
} from "$lib/bindings/changeme/dbhandler";
import { Events } from "@wailsio/runtime";
import { cycleIndex } from "./cycle";

const createSongsStore = () => {
  let songsList = $state<Song[]>([]);
  let songsLoading = $state(true);
  let activeSong = $state<Song | null>(null);

  let coupletsList = $state<Couplet[]>([]);
  let coupletsLoading = $state(true);
  let activeCouplet = $state<Couplet | null>(null);

  let shownCouplet = $state<ShownCouplet | null>(null);
  const showCouplet = (v: ShownCouplet) => {
    shownCouplet = v;
  };
  const hideCouplet = () => {
    shownCouplet = null;
  };
  Events.On("show_couplet", ({ data }: { data: ShownCouplet }) => {
    showCouplet(data);
  });
  Events.On("hide_couplet", () => {
    hideCouplet();
  });

  let favoriteSongs = $state<(Song & { localId: number })[]>([]);
  let activeFavorite = $state<Song & { localId: number } | null>(null);

  const songs = {
    get loading() {
      return songsLoading;
    },
    get list() {
      return songsList;
    },
    set list(v) {
      const prevActive = activeSong;
      songsList = v;

      const kept = prevActive && v.find((s) => s.ID === prevActive.ID);
      activeSong = kept ?? v.at(0) ?? null;
      songsLoading = false;

      if (activeSong?.ID !== prevActive?.ID) {
        coupletsList = activeSong?.couplets ?? [];
        activeCouplet = coupletsList.at(0) || null;
        coupletsLoading = false;
      }

      favoriteSongs = favoriteSongs.filter((f) =>
        songsList.some((s) => s.ID === f.ID),
      );
      if (
        activeFavorite &&
        !favoriteSongs.some((f) => f.localId === activeFavorite!.localId)
      ) {
        activeFavorite = null;
      }
    },
    get active() {
      return activeSong;
    },
    set active(val) {
      activeSong = val;

      if (!val) {
        coupletsList = [];
        activeCouplet = null;
        coupletsLoading = false;
        return;
      }

      coupletsLoading = true;
      GetCouplets(val.ID).then((couplets) => {
        coupletsList = couplets;
        activeCouplet = coupletsList.at(0) || null;
        coupletsLoading = false;
      });
    },
  };
  Events.On(
    "songs_update",
    ({ data }: { data: Song[] }) => (songs.list = data),
  );
  Events.On("song_update", ({ data }: { data: Song }) => {
    if (songs.active?.ID !== data.ID) {
      return;
    }
    coupletsList = data.couplets;
    const active = coupletsList.find((v) => v.ID === activeCouplet?.ID);
    activeCouplet = active ?? coupletsList.at(0) ?? null;
  });

  const couplets = {
    get loading() {
      return coupletsLoading;
    },
    get list() {
      return coupletsList;
    },
    get active() {
      return activeCouplet;
    },
    set active(val) {
      activeCouplet = val;
    },

    next() {
      const n = cycleIndex(coupletsList, activeCouplet, 1);
      if (n) this.active = n;
    },
    prev() {
      const n = cycleIndex(coupletsList, activeCouplet, -1);
      if (n) this.active = n;
    },

    get shown() {
      return shownCouplet;
    },
    set shown(v) {
      if (v) {
        showCouplet(v);
      } else {
        hideCouplet();
      }
    },
  };

  const favorites = {
    get list() {
      return favoriteSongs;
    },
    get active() {
      return activeFavorite;
    },
    set active(v) {
      activeFavorite = v;
    },
    add(s: Song) {
      favoriteSongs.push({ ...s, localId: Math.random() });
    },
    moveDown(s: (typeof favoriteSongs)[number]) {
      const ind = favoriteSongs.findIndex((v) => v.localId === s.localId);
      if (ind === -1 || ind === favoriteSongs.length - 1) return;
      const next = favoriteSongs[ind + 1];
      favoriteSongs[ind] = next;
      favoriteSongs[ind + 1] = s;
    },
    moveUp(s: (typeof favoriteSongs)[number]) {
      const ind = favoriteSongs.findIndex((v) => v.localId === s.localId);
      if (ind === -1 || ind === 0) return;
      const next = favoriteSongs[ind - 1];
      favoriteSongs[ind] = next;
      favoriteSongs[ind - 1] = s;
    },
    remove(localId: number) {
      favoriteSongs = favoriteSongs.filter((s) => s.localId !== localId);
    },
  };

  let qr = $state(false);

  return {
    songs,
    couplets,
    favorites,
    get qr() {
      return qr;
    },
    set qr(v: boolean) {
      qr = v;
    },
  };
};

GetSongs().then((s) => (songsStore.songs.list = s));
GetShownCouplet().then((c) => (songsStore.couplets.shown = c));

export const songsStore = createSongsStore();
