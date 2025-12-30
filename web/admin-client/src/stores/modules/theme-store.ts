import { ref } from "vue";
import { defineStore } from "pinia";
import { useTheme } from "@/hooks/theme-hooks";

const { setDarkTheme, setDefaultTheme } = useTheme();

const useThemeStore = defineStore('theme', () => {

  // 从 localStorage 读取保存的主题设置，默认为 'default'
  const savedTheme = localStorage.getItem('theme') || 'default';
  const theme = ref(savedTheme);

  const setThemeConfig = async () => {
    if(theme.value === 'default') {
      setDefaultTheme();
    } else {
      setDarkTheme();
    }
  }

  // 初始化时应用保存的主题
  setThemeConfig();

  return {
    theme,
    setThemeConfig
  }
})

export default useThemeStore;
