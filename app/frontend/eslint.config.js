import tseslint from 'typescript-eslint'

export default tseslint.config(
  ...tseslint.configs.recommended,
  { files: ['src/**/*.{ts,tsx}'], rules: { '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }] } },
  {
    files: ['tests/**/*.mjs'],
    languageOptions: {
      globals: {
        CanvasRenderingContext2D: 'readonly',
        URL: 'readonly',
        document: 'readonly',
        setInterval: 'readonly',
        setTimeout: 'readonly',
        window: 'readonly',
      },
    },
    rules: { 'no-undef': 'error' },
  },
)
