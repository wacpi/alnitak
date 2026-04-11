/**
 * 服务端全局错误处理插件
 * 捕获 unhandledRejection 和 uncaughtException，防止进程崩溃和日志噪音
 */
export default defineNuxtPlugin(() => {
  // 仅在服务端执行
  if (process.client) return;

  // 已知可忽略的网络错误码和关键字
  const IGNORABLE_CODES = ['ECONNRESET', 'ECONNREFUSED', 'ETIMEDOUT', 'EPIPE', 'ENOTFOUND', 'ECONNABORTED', 'ERR_STREAM_DESTROYED'];
  const IGNORABLE_MESSAGES = ['socket hang up', 'read ECONNRESET', 'write ECONNRESET', 'aborted', 'Client network socket disconnected'];

  const isIgnorableError = (err: any): boolean => {
    if (!err) return true; // undefined/null 错误直接忽略

    const code = String(err?.code || '');
    const message = String(err?.message || err || '');

    // 检查错误码
    if (IGNORABLE_CODES.some(c => code.includes(c))) return true;

    // 检查错误消息
    if (IGNORABLE_MESSAGES.some(m => message.includes(m))) return true;
    if (IGNORABLE_CODES.some(c => message.includes(c))) return true;

    return false;
  };

  // 避免重复注册
  if (!(process as any).__errorHandlerRegistered) {
    (process as any).__errorHandlerRegistered = true;

    process.on('unhandledRejection', (reason: any) => {
      if (isIgnorableError(reason)) {
        // 网络波动导致的错误，静默忽略（不打印警告以减少日志噪音）
        return;
      }
      // 其他未处理的 rejection 仍然记录
      console.error('[unhandledRejection]', reason);
    });

    process.on('uncaughtException', (err: Error) => {
      if (isIgnorableError(err)) {
        // 网络错误静默忽略
        return;
      }
      // 严重错误，记录但不退出进程（Nuxt 开发服务器会处理）
      console.error('[uncaughtException]', err);
    });
  }
});
