// tailwind.config.js
export default {
  content: ['./src/**/*.{html,svelte,ts}'],
  safelist: [
    {
      pattern: /(bg|text)-(primary|secondary|tertiary|option)-(50|100|200|300|400|500|600|700|800|900)/
    }
  ],
  theme: {},
  plugins: []
};
