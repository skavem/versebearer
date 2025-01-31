import { Screens } from "@wailsio/runtime";

type Screen = Screens.Screen;

const createScreenStore = () => {
  let list = $state<Screen[]>([]);
  let activeScreens = $state<string[]>([]);

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
  };
};

export const screenStore = createScreenStore();
Screens.GetAll().then((s) => (screenStore.list = s));
