import tseslint from "typescript-eslint";
import solid from "eslint-plugin-solid";
import globals from "globals";

export default tseslint.config(
  {
    ignores: ["dist/", "node_modules/", "**/*.config.*", "backlog/"],
  },

  // Base TypeScript config with project service for auto-discovery of tsconfig.json
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
        allowDefaultProject: ["frontend/vite.config.ts"],
      },
    },
  },

  // Only type-check TS/TSX files; ignore JS dist artifacts and config files
  {
    ignores: ["**/*.js", "**/*.cjs", "**/*.mjs", "**/dist/**"],
  },

  // Strict type-checked rules
  ...tseslint.configs.strictTypeChecked,
  ...tseslint.configs.stylisticTypeChecked,

  // Bug-class rules: floating promises, misused promises, await/no-thenable
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/no-floating-promises": ["error", { ignoreVoid: false }],
      "@typescript-eslint/no-misused-promises": ["error", {
        checksVoidReturn: true,
        checksConditionals: true,
        checksSpreads: true,
      }],
      "@typescript-eslint/await-thenable": "error",
      "@typescript-eslint/require-await": "error",
      "@typescript-eslint/return-await": "error",
      "@typescript-eslint/promise-function-async": "error",
    },
  },

  // Unsafe-any rules
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/no-unsafe-assignment": "error",
      "@typescript-eslint/no-unsafe-member-access": "error",
      "@typescript-eslint/no-unsafe-call": "error",
      "@typescript-eslint/no-unsafe-return": "error",
      "@typescript-eslint/no-unsafe-argument": "error",
      "@typescript-eslint/no-explicit-any": "error",
    },
  },

  // Hygiene rules
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/consistent-type-imports": ["error", { prefer: "type-imports" }],
      "@typescript-eslint/no-unused-vars": ["error", {
        argsIgnorePattern: "^_",
        varsIgnorePattern: "^_",
      }],
      "no-console": ["warn", { allow: ["warn", "error"] }],
      "eqeqeq": "error",
    },
  },

  // Backend-specific: Node.js globals
  {
    files: ["backend/**/*.{ts,tsx}"],
    languageOptions: {
      globals: globals.node,
    },
  },

  // Frontend-specific: Solid.js + browser globals
  {
    files: ["frontend/**/*.{ts,tsx}"],
    languageOptions: {
      globals: globals.browser,
    },
    plugins: {
      solid,
    },
    rules: {
      ...solid.configs.recommended.rules,
      ...solid.configs.typescript.rules,
    },
  },

  // Shared (treat as Node context for shared types)
  {
    files: ["shared/**/*.{ts,tsx}"],
    languageOptions: {
      globals: globals.node,
    },
  }
);
