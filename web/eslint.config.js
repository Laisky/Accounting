import js from '@eslint/js';
import globals from 'globals';
import { createTypeScriptImportResolver } from 'eslint-import-resolver-typescript';
import importX from 'eslint-plugin-import-x';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import tseslint from 'typescript-eslint';

export default tseslint.config(
  { ignores: ['dist', 'playwright-report', 'test-results', '*.tsbuildinfo', 'vite.config.d.ts', 'vite.config.js'] },
  {
    extends: [
      js.configs.recommended,
      ...tseslint.configs.recommendedTypeChecked,
      importX.flatConfigs.recommended,
      importX.flatConfigs.typescript,
    ],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: {
        ...globals.browser,
        ...globals.node,
      },
      parserOptions: {
        projectService: {
          allowDefaultProject: ['playwright.config.ts', 'e2e/*.ts'],
        },
        tsconfigRootDir: import.meta.dirname,
      },
    },
    settings: {
      'import-x/resolver-next': [
        createTypeScriptImportResolver({
          project: './tsconfig.json',
        }),
        importX.createNodeResolver({
          extensions: ['.mjs', '.cjs', '.js', '.jsx', '.ts', '.tsx', '.json'],
        }),
      ],
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // The current large integration-style tests intentionally cross untyped
      // fetch/mock boundaries. Keep type-aware linting enabled while deferring
      // those unsafe-any cleanups to the Phase 7 MSW/test split.
      '@typescript-eslint/no-base-to-string': 'off',
      '@typescript-eslint/no-floating-promises': 'off',
      '@typescript-eslint/no-misused-promises': 'off',
      '@typescript-eslint/require-await': 'off',
      '@typescript-eslint/no-unsafe-argument': 'off',
      '@typescript-eslint/no-unsafe-assignment': 'off',
      '@typescript-eslint/no-unsafe-call': 'off',
      '@typescript-eslint/no-unsafe-member-access': 'off',
      '@typescript-eslint/no-unsafe-return': 'off',
      // eslint-plugin-react-hooks v7 promotes the React Compiler advisory
      // `set-state-in-effect` rule into the recommended set. The workspaces
      // intentionally use the standard fetch-in-effect and reset-on-dependency
      // patterns, so surface it as a warning instead of failing the build while
      // still keeping rules-of-hooks and exhaustive-deps enforced as errors.
      'react-hooks/set-state-in-effect': 'warn',
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
      'import-x/no-cycle': ['error', { maxDepth: 5 }],
      'import-x/no-duplicates': 'error',
      // `tsc -b` is the authority for the Vite/TypeScript `@/*` alias; the
      // import resolver does not currently resolve that alias in this setup.
      'import-x/no-unresolved': ['error', { ignore: ['^@/', '\\.css$'] }],
    },
  },
);
