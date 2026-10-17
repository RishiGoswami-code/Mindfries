import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // The copy-*.js scripts are plain CommonJS run by node at postinstall and
  // build time, outside the bundler: require() is the right call there.
  {
    files: ["scripts/**/*.js"],
    rules: { "@typescript-eslint/no-require-imports": "off" },
  },
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
    // Everything under public/ is shipped as-is, not our source: Monaco and
    // Pyodide are copied in by scripts/copy-*.js (~40MB of minified JS) and
    // vendor/ holds the pdf.js worker. Flat-config ESLint doesn't read
    // .gitignore, and linting them runs Node out of memory.
    "public/**",
  ]),
]);

export default eslintConfig;
