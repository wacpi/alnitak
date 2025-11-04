export default defineNuxtPlugin(() => {
  if (process.server) return;
  const THEME_KEY = 'ui-theme-mode';
  try {
    const saved = (localStorage.getItem(THEME_KEY) as 'light' | 'dark' | null) || 'light';
    const root = document.documentElement;
    root.setAttribute('data-theme', saved);
    if (saved === 'dark') root.classList.add('dark'); else root.classList.remove('dark');
  } catch {}
});






