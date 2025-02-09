import type { ShownCouplet } from "$lib/bindings/changeme";
import { Couplet, Song } from "$lib/bindings/changeme/backend/models";
import {
  GetCouplets,
  GetShownCouplet,
  GetSongs,
} from "$lib/bindings/changeme/dbhandler";
import { Events } from "@wailsio/runtime";

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
  Events.On("show_couplet", ({ data }: { data: ShownCouplet[] }) => {
    showCouplet(data[0]);
  });
  Events.On("hide_couplet", () => {
    hideCouplet();
  });

  let favoriteSongs = $state<(Song & { localId: number })[]>([]);

  const songs = {
    get loading() {
      return songsLoading;
    },
    get list() {
      return songsList;
    },
    set list(v) {
      songsList = v;
      activeSong = songsList.at(0) || null;
      songsLoading = false;

      coupletsList = activeSong?.couplets ?? [];
      activeCouplet = coupletsList.at(0) || null;
      coupletsLoading = false;
    },
    get active() {
      return activeSong;
    },
    set active(val) {
      activeSong = val;

      coupletsLoading = true;
      GetCouplets(activeSong!.ID).then((couplets) => {
        coupletsList = couplets;
        activeCouplet = coupletsList[0];
        coupletsLoading = false;
      });
    },
  };
  Events.On(
    "songs_update",
    ({ data }: { data: Song[][] }) => (songs.list = data[0]),
  );
  Events.On("song_update", ({ data }: { data: Song[] }) => {
    if (songs.active?.ID !== data[0].ID) {
      return;
    }
    coupletsList = data[0].couplets;
    const active = coupletsList.find((v) => v.ID === activeCouplet?.ID);
    if (!active) {
      return;
    }
    activeCouplet = active;
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
      const ind = coupletsList.findIndex((v) => v.ID === activeCouplet?.ID);
      if (ind === -1 || ind === coupletsList.length - 1) return;
      this.active = coupletsList[ind + 1];
    },
    prev() {
      const ind = coupletsList.findIndex((v) => v.ID === activeCouplet?.ID);
      if (ind === -1 || ind === 0) return;
      this.active = coupletsList[ind - 1];
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
