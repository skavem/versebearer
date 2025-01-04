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
          primary: "#0ea5e9",
          secondary: "#10b981",
          accent: "#84cc16",
          neutral: "#0c4a6e",
          "base-100": "#ffffff",
          info: "#6366f1",
          success: "#22c55e",
          warning: "#eab308",
          error: "#ef4444",
        },
      },
    ],
  },
  plugins: [daisyui],
};
