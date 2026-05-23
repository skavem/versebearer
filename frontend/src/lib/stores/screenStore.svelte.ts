import {
  CloseScreen,
  GetCurrentScreenID,
  ShowScreen,
} from "$lib/bindings/changeme/dbhandler";
import { Events, Screens } from "@wailsio/runtime";

type Screen = Screens.Screen;

export const screenId = (s: Screen) => `screen ${s.ID}`;

const createScreenStore = () => {
  let list = $state<Screen[]>([]);
  let activeScreens = $state<string[]>([]);
  let currentScreenID = $state<string>("");
  let pendingScreen = $state<Screen | null>(null);

  Events.On("current_screen", ({ data }: { data: string }) => {
    currentScreenID = data;
  });
  Events.On("screen_closed", ({ data }: { data: string }) => {
    activeScreens = activeScreens.filter((v) => v !== data);
  });

  function applyToggle(s: Screen) {
    const id = screenId(s);
    if (activeScreens.includes(id)) {
      CloseScreen(id);
      activeScreens = activeScreens.filter((v) => v !== id);
    } else {
      ShowScreen(s.Bounds.X, s.Bounds.Y, s.Bounds.Width, s.Bounds.Height, id);
      activeScreens.push(id);
    }
  }

  return {
    get list() {
      return list;
    },
    set list(newList) {
      list = newList;
    },
    get activeScreens() {
      return activeScreens;
    },
    set activeScreens(newActive) {
      activeScreens = newActive;
    },
    get currentScreenID() {
      return currentScreenID;
    },
    set currentScreenID(v: string) {
      currentScreenID = v;
    },
    get pendingScreen() {
      return pendingScreen;
    },
    requestToggle(s: Screen) {
      const id = screenId(s);
      const projecting = activeScreens.includes(id);
      if (!projecting && s.ID === currentScreenID) {
        pendingScreen = s;
        return;
      }
      applyToggle(s);
    },
    confirmPending() {
      if (pendingScreen) {
        applyToggle(pendingScreen);
        pendingScreen = null;
      }
    },
    cancelPending() {
      pendingScreen = null;
    },
  };
};

export const screenStore = createScreenStore();
Screens.GetAll().then((s) => (screenStore.list = s));
GetCurrentScreenID().then((id) => (screenStore.currentScreenID = id));
