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
    // Third-party bundles, not our code: Monaco and Pyodide are copied in by
    // scripts/copy-*.js (gitignored, ~40MB of minified JS), vendor/ holds the
    // pdf.js worker. Flat-config ESLint doesn't read .gitignore, and linting
    // these runs Node out of memory before a single real error is reported.
    "public/monaco-editor/**",
    "public/pyodide/**",
    "public/vendor/**",
  ]),
]);

export default eslintConfig;
