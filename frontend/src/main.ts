import { mount } from 'svelte';
import App from './App.svelte';

// Modul epic lain mendaftarkan panel/tab saat di-import. Glob kosong aman bila folder belum ada.
import.meta.glob('./modules/*/index.ts', { eager: true });

const app = mount(App, { target: document.getElementById('app')! });
export default app;
