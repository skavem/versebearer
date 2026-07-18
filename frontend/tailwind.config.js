/** @type {import('tailwindcss').Config} */
import daisyui from "daisyui";
import { fontFamily } from "tailwindcss/defaultTheme";
export default {
  content: ["./src/**/*.{html,js,svelte,ts}"],
  theme: {
    extend: {
      colors: {
        seawave: "#1B4965",
        sealight: "#62B6CB",
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
      fontFamily: {
        sans: [...fontFamily.sans],
      },
    },
  },
  daisyui: {
    themes: [
      {
        mytheme: {
          // Палитра «Петроль + Янтарь» (светлая): петроль-navy как база,
          // тёплый янтарь как индикатор «в эфире / текущее». Серьёзно, без мультяшности.
          primary: "#22668A", // стальной синий — основные действия
          "primary-content": "#ffffff",
          secondary: "#C88A3D", // янтарь — «в эфире / текущее»
          "secondary-content": "#1a2a33",
          accent: "#1B3A4B", // резерв, в одной семье с базой
          "accent-content": "#ffffff",
          neutral: "#1B3A4B", // петроль-navy — тёмные поверхности и текст
          "neutral-content": "#eaf1f5",
          "base-100": "#ffffff",
          "base-200": "#f1f5f8",
          "base-300": "#dce5eb",
          "base-content": "#1B3A4B",
          info: "#2E7DA6",
          success: "#3E7C76",
          warning: "#B5822E",
          error: "#C0453B",
          "error-content": "#ffffff",
          "--rounded-box": "0.5rem", // border radius rounded-box utility class, used in card and other large boxes
          "--rounded-btn": "0.25rem", // border radius rounded-btn utility class, used in buttons and similar element
          "--rounded-badge": "0.25rem", // border radius rounded-badge utility class, used in badges and similar
          "--animation-btn": "0.25s", // duration of animation when you click on button
          "--animation-input": "0.2s", // duration of animation for inputs like checkbox, toggle, radio, etc
          "--btn-focus-scale": "0.95", // scale transform of button when you focus on it
          "--border-btn": "1px", // border width of buttons
          "--tab-border": "1px", // border width of tabs
          "--tab-radius": "0.5rem", // border radius of tabs
        },
      },
      {
        mydark: {
          // Палитра «Петроль + Янтарь» (тёмная). Элевация «вверх»: фон самый
          // тёмный, поверхности и рамки светлее — мягкое расслоение вместо
          // плоского MUI. Акценты приглушённо-люминесцентные, не кислотные.
          primary: "#4B9FCF", // люминесцентный стальной синий
          "primary-content": "#08151d",
          secondary: "#E0A75B", // тёплый мягкий янтарь
          "secondary-content": "#201603",
          accent: "#4B9FCF",
          "accent-content": "#08151d",
          neutral: "#21323f", // глубокий петроль — тёмный навбар (белый текст)
          "neutral-content": "#e6edf3",
          "base-100": "#17232f", // фон (страница + карточки), не чёрный
          "base-200": "#20303e", // внутренние плашки — светлее, «приподняты»
          "base-300": "#33485b", // рамки/грани — видимый мягкий контур
          "base-content": "#dbe5ee", // мягкий холодный не-белый
          info: "#56A6D2",
          success: "#5CAEA4",
          warning: "#E0B45C",
          error: "#E07064", // мягкий коралловый, не резкий
          "error-content": "#1c0806",
          "--rounded-box": "0.5rem",
          "--rounded-btn": "0.25rem",
          "--rounded-badge": "0.25rem",
          "--animation-btn": "0.25s",
          "--animation-input": "0.2s",
          "--btn-focus-scale": "0.95",
          "--border-btn": "1px",
          "--tab-border": "1px",
          "--tab-radius": "0.5rem",
        },
      },
    ],
  },
  plugins: [daisyui],
};
