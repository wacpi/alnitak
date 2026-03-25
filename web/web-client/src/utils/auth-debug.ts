// auth 调试日志（仅开发环境输出）
// 目的：线上不产生日志噪音，同时保留定位能力

export const authDebug = (...args: any[]) => {
  if (import.meta.dev) {
    // eslint-disable-next-line no-console
    console.debug(...args);
  }
};

