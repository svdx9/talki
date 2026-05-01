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

  // Bug-class rules (highest priority): logical errors that cause runtime failures
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/no-floating-promises": ["error", { ignoreVoid: true }],
      "@typescript-eslint/no-misused-promises": ["error", {
        checksVoidReturn: false,
        checksConditionals: true,
        checksSpreads: true,
      }],
      "@typescript-eslint/await-thenable": "error",
      "@typescript-eslint/no-explicit-any": "error",
      "@typescript-eslint/no-unnecessary-type-assertion": "error",
      "@typescript-eslint/no-non-null-assertion": "error",
    },
  },

  // SDK/third-party integration issues: relax for external SDKs with poor types
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/no-unsafe-assignment": "warn",
      "@typescript-eslint/no-unsafe-member-access": "warn",
      "@typescript-eslint/no-unsafe-call": "warn",
      "@typescript-eslint/no-unsafe-return": "warn",
      "@typescript-eslint/no-unsafe-argument": "warn",
    },
  },

  // Hygiene & code quality rules
  {
    files: ["**/*.{ts,tsx}"],
    rules: {
      "@typescript-eslint/consistent-type-imports": ["error", { prefer: "type-imports" }],
      "@typescript-eslint/no-unused-vars": ["error", {
        argsIgnorePattern: "^_",
        varsIgnorePattern: "^_",
      }],
      "@typescript-eslint/require-await": "warn",
      "@typescript-eslint/return-await": "warn",
      "@typescript-eslint/promise-function-async": "warn",
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
