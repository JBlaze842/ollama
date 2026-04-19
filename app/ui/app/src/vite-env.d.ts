/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly DEV: boolean;
  readonly VITE_OLLAMA_DOT_COM_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
