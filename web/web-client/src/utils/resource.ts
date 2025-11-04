import { isUrl } from "./verify";
import { globalConfig } from "./global-config";

export const getResourceUrl = (originalUrl: string) => {
    // 如果本身就是一个完整的URL,直接返回
    if (isUrl(originalUrl)) {
        return originalUrl;
    }
    // 开发环境：无论是SSR还是客户端，都返回相对路径，通过Vite代理访问（避免暴露后端地址）
    // 这样可以避免SSR时序列化完整URL到HTML中，导致刷新后暴露后端地址
    if (process.dev) {
        return originalUrl;
    }

    // 生产环境：如果配置了域名则拼接完整URL，否则保持相对路径
    if (!globalConfig.domain) {
        return originalUrl;
    }

    // 前后端不同源（生产环境）
    return `http${globalConfig.https ? 's' : ''}://${globalConfig.domain}${originalUrl}`;
}
