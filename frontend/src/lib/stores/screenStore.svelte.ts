import { GetCurrentScreenID } from "$lib/bindings/changeme/dbhandler";
import { Events, Screens } from "@wailsio/runtime";

type Screen = Screens.Screen;

const createScreenStore = () => {
  let list = $state<Screen[]>([]);
  let activeScreens = $state<string[]>([]);
  let currentScreenID = $state<string>("");

  Events.On("current_screen", ({ data }: { data: string }) => {
    currentScreenID = data;
  });

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
  };
};

export const screenStore = createScreenStore();
Screens.GetAll().then((s) => (screenStore.list = s));
GetCurrentScreenID().then((id) => (screenStore.currentScreenID = id));
