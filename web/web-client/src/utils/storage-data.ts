interface StorageType {
    key: string,
    value: {
        data: object,
        expired: number
    }
}

// 检查 localStorage 是否可用(SSR 兼容 + 隐私模式兼容)
const isLocalStorageAvailable = (): boolean => {
    if (typeof window === 'undefined' || typeof localStorage === 'undefined') {
        return false;
    }
    try {
        const testKey = '__localStorage_test__';
        localStorage.setItem(testKey, 'test');
        localStorage.removeItem(testKey);
        return true;
    } catch (e) {
        // Safari 隐私模式下会抛出 QuotaExceededError
        return false;
    }
};

export const storageData = {
    //key保存键,value保存内容,*expired 失效时间,单位分钟
    set(key: string, value: string, expired: number): void {
        if (!isLocalStorageAvailable()) {
            console.warn('localStorage is not available');
            return;
        }

        try {
            const now = Date.now();
            const source = { key: key, value: value };

            if (expired) {
                source.value = JSON.stringify({ data: value, expired: now + (1000 * 60 * expired) });
            } else {
                source.value = JSON.stringify({ data: value });
            }
            localStorage.setItem(source.key, source.value);
        } catch (error) {
            console.error('Failed to set localStorage:', error);
        }
    },
    get(key: string): any {
        if (!isLocalStorageAvailable()) {
            return null;
        }

        try {
            const now = Date.now();
            const source: StorageType = {
                key: key,
                value: {
                    data: {},
                    expired: 0
                }
            };

            const readStorage = localStorage.getItem(source.key);
            if (!readStorage) {
                return null;
            }

            source.value = JSON.parse(readStorage);

            if (source.value) {
                //超过失效时 删除
                if (!source.value.expired) {
                    return source.value.data;
                }
                else if (now >= source.value.expired) {
                    this.remove(source.key);
                    return null;
                } else {
                    return source.value.data;
                }
            }

            return null;
        } catch (error) {
            console.error('Failed to get localStorage:', error);
            return null;
        }
    },
    //更新
    update(key: string, data: any): void {
        if (!isLocalStorageAvailable()) {
            console.warn('localStorage is not available');
            return;
        }

        try {
            const read = localStorage.getItem(key);
            if (read) {
                const value = JSON.parse(read);
                value.data = data;
                localStorage.setItem(key, JSON.stringify(value));
            }
        } catch (error) {
            console.error('Failed to update localStorage:', error);
        }
    },
    remove(key: string): void {
        if (!isLocalStorageAvailable()) {
            return;
        }

        try {
            localStorage.removeItem(key);
        } catch (error) {
            console.error('Failed to remove localStorage:', error);
        }
    },
}