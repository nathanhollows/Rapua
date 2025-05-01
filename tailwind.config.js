/** @type {import('tailwindcss').Config} */
export const content = [
  "./docs/**/*.md",
  "./web/templates/**/*.html",
  "./internal/templates/**/*.templ",
  "./internal/blocks/templates/**/*.templ"
];
export const theme = {
  extend: {
    keyframes: {
      wobble: {
        from: { transform: 'translate3d(0, 0, 0)', },
        '15%': { transform: 'translate3d(-25%, 0, 0) rotate3d(0, 0, 1, -5deg)', },
        '30%': { transform: 'translate3d(20%, 0, 0) rotate3d(0, 0, 1, 3deg)', },
        '45%': { transform: 'translate3d(-15%, 0, 0) rotate3d(0, 0, 1, -3deg)', },
        '60%': { transform: 'translate3d(10%, 0, 0) rotate3d(0, 0, 1, 2deg)', },
        '75%': { transform: 'translate3d(-5%, 0, 0) rotate3d(0, 0, 1, -1deg)', },
        to: { transform: 'translate3d(0, 0, 0)', },
      },
    },
    animation: {
      wiggle: 'wiggle 1s ease-in-out infinite',
    },
  },
};
export const plugins = [
  require('@tailwindcss/container-queries'),
  require("@tailwindcss/typography"),
  require("daisyui"),
];
export const daisyui = {
  themes: ["cupcake", "dracula"],
  darkTheme: "dracula",
};
