import type {Config} from "tailwindcss";
import {heroui} from "@heroui/theme";

export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
    "./node_modules/@heroui/theme/dist/**/*.{js,ts,jsx,tsx}"
  ],
  theme: {extend: {}},
  darkMode: "class",
  plugins: [heroui()]
} satisfies Config;
