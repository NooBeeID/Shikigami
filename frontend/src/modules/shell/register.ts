// Side-effect: daftarkan item rail 'agents', tab kanan 'files', item topbar 'notifications', aksi/sumber palet.
// Jangan diimpor dari modules/shell/index.ts (hindari siklus); App.svelte yang mengimpornya.
import '../sidebar-left';
import '../sidebar-right';
import '../notifications';
import '../palette';
