import request from '@/utils/request';
import { getFileMD5 } from '@/utils/md5';
import type { AxiosProgressEvent } from 'axios';
import { statusCode } from '@/utils/status-code';

// 配置常量
const CHUNK_SIZE = 5 * 1024 * 1024; // 每个分片大小为5MB
const MAX_CONCURRENT_UPLOADS = 1; // 串行上传，避免服务器连接过载
const MAX_RETRIES = 5; // 最大重试次数
const INITIAL_RETRY_DELAY = 2000; // 初始重试延迟（毫秒）
const CHUNK_TIMEOUT = 120000; // 分片上传超时时间（2分钟）
const MERGE_TIMEOUT = 600000; // 合并超时时间（10分钟，大文件合并需要较长时间）

// 延迟函数
const delay = (ms: number) => new Promise(resolve => setTimeout(resolve, ms));

// 指数退避延迟计算
const getRetryDelay = (retryCount: number) => {
  return INITIAL_RETRY_DELAY * Math.pow(2, retryCount) + Math.random() * 1000;
};

// 上传图片
export const uploadFileAPI = ({ name, file, action, onProgress, onFinish, onError }: UploadOptionsType) => {
  const formData = new FormData();
  formData.append(name, file)
  request.post(action, formData, {
    timeout: 50000,
    headers: {
      'Content-Type': 'multipart/form-data'
    },
    onUploadProgress: (progressEvent: AxiosProgressEvent) => {
      if (!progressEvent.total) {
        onProgress(0);
        return;
      }
      onProgress(Math.floor(progressEvent.loaded / progressEvent.total * 100));
    }
  }).then((res) => {
    if (res.data.code === statusCode.OK) {
      onFinish(res.data);
    } else {
      onError(res.data);
    }
  }).catch((err) => {
    onError(err);
  })
}

export const uploadFileChunkAPI = async ({ name, file, action, replaceResourceID, onProgress, onFinish, onError }: UploadOptionsType) => {
  onProgress(0);
  const hash = await getFileMD5(file)
  const size = file.size

  const totalChunks = Math.ceil(file.size / CHUNK_SIZE);
  const formDataGenerator = createFormDataGenerator(hash, name, file.name, totalChunks.toString(), size);

  // 检查是否可以秒传，获取 fileID
  const { chunks: uploadedChunks, instantUpload, fileID } = await getUploadedChunksAPI(hash, size)

  // 【秒传】文件已存在且转码完成，直接完成上传
  if (instantUpload) {
    console.log('【秒传】文件已存在，跳过上传直接完成, fileID:', fileID)
    onProgress(100);
    // 秒传时跳过合并，直接调用创建接口
    try {
      const res = await request.post(action, { hash, fileID, size, replaceResourceID }, {
        timeout: MERGE_TIMEOUT,
      });
      if (res.data.code === statusCode.OK) {
        onFinish(res.data);
      } else {
        onError(res.data);
      }
    } catch (error) {
      console.error('秒传完成请求失败', error);
      onError(error);
    }
    return { controllers: [], fileID };
  }

  const tasks: number[] = []
  for (let i = 0; i < totalChunks; i++) {
    if (!uploadedChunks.includes(i)) {
      tasks.push(i)
    }
  }

  if (tasks.length === 0) {
    finishUploadAPI({ hash, fileID, size, action, replaceResourceID, onFinish, onError })
    return { controllers: [], fileID };
  }

  // 如果服务器不存在数据则手动上传第一个分片（带重试）
  if (tasks.length === totalChunks) {
    const chunk = file.slice(0, Math.min(CHUNK_SIZE, file.size));
    const formData = formDataGenerator(chunk, "0");
    const controller = new AbortController();

    let firstChunkSuccess = false;
    for (let retry = 0; retry <= MAX_RETRIES; retry++) {
      try {
        const firstChunkRes = await uploadChunkAPI(formData, controller);
        if (firstChunkRes.data.code === statusCode.OK) {
          uploadedChunks.push(0);
          tasks.splice(tasks.indexOf(0), 1);
      // 检查是否完成所有分片上传（针对小文件只有一个分片的情况）
      if (uploadedChunks.length === totalChunks) {
        finishUploadAPI({ hash, fileID, size, action, replaceResourceID, onFinish, onError });
        return { controllers: [controller], fileID };
      }
          firstChunkSuccess = true;
          break;
        }
      } catch (error) {
        if (retry < MAX_RETRIES) {
          console.warn(`首个分片上传失败，${retry + 1}/${MAX_RETRIES} 次重试中...`);
          await delay(getRetryDelay(retry));
          continue;
        }
      }
    }

    if (!firstChunkSuccess) {
      onError({ msg: '首个分片上传失败，请检查网络后重试' });
      return { controllers: [controller], fileID };
    }
  }
  // 上传进度
 let taskQueue = [...tasks]
 let currentUploads = 0;
 let uploadedChunksCount = uploadedChunks.length;
 const controllers: AbortController[] = [];

  const processQueue = () => {
    while (currentUploads < MAX_CONCURRENT_UPLOADS && taskQueue.length > 0) {
      const nextTaskIndex = taskQueue.shift();// 从队列中取出下一个任务
      if (nextTaskIndex === undefined) break;
      currentUploads++;
      uploadChunk(nextTaskIndex); // 上传该块
    }
  };

  const uploadChunk = async (i: number, retryCount = 0): Promise<void> => {
    const start = i * CHUNK_SIZE;
    const end = Math.min(start + CHUNK_SIZE, file.size);
    const chunk = file.slice(start, end);
    const formData = formDataGenerator(chunk, i.toString());
    const controller = new AbortController();
    controllers.push(controller);

    try {
      const res = await uploadChunkAPI(formData, controller);
      if (res.data.code === statusCode.OK) {
        uploadedChunksCount++;
        // 更新进度
        const progress = Math.floor((uploadedChunksCount / totalChunks) * 100);
        if (uploadedChunksCount === totalChunks) {
          finishUploadAPI({ hash, fileID, size, action, replaceResourceID, onFinish, onError });
        } else {
          onProgress(progress);
        }
      } else {
        // 服务端返回错误，尝试重试
        if (retryCount < MAX_RETRIES) {
          console.warn(`分片 ${i} 上传失败，${retryCount + 1}/${MAX_RETRIES} 次重试中...`);
          await delay(getRetryDelay(retryCount));
          return uploadChunk(i, retryCount + 1);
        }
        onError(res.data);
      }
    } catch (error) {
      if (typeof error === 'object' && error !== null && 'name' in error && (error as any).name === 'CanceledError') {
        // 被用户取消，不重试
        return;
      }

      // 网络错误，尝试重试
      if (retryCount < MAX_RETRIES) {
        console.warn(`分片 ${i} 网络错误，${retryCount + 1}/${MAX_RETRIES} 次重试中...`, error);
        await delay(getRetryDelay(retryCount));
        return uploadChunk(i, retryCount + 1);
      }

      console.error(`分片 ${i} 上传失败，已达到最大重试次数`);
      onError(error);
    } finally {
      currentUploads--;
      processQueue(); // 尝试处理下一个任务
    }
  };

  // 开始处理队列
  processQueue();
  return { controllers, fileID };
}

const createFormDataGenerator = (hash: string, name: string, fileName: string, totalChunks: string, size: number) => {
  const savedHash = hash;
  const savedName = name;
  const savedFileName = fileName;
  const savedTotalChunks = totalChunks;
  const savedSize = size;

  return (chunk: Blob, i: string) => {
    const formData = new FormData();
    formData.append(savedName, chunk);
    formData.append('hash', savedHash);
    formData.append('name', savedFileName);
    formData.append('chunkIndex', i.toString());
    formData.append('totalChunks', savedTotalChunks);
    formData.append('size', savedSize.toString());

    return formData;
  };
}

// 检查上传进度，返回 { chunks: number[], fileID: number, instantUpload: boolean }
// fileID: 视频文件ID，用于后续合并
// instantUpload=true 表示文件已存在可秒传
const getUploadedChunksAPI = async (hash: string, size: number): Promise<{ chunks: number[], fileID: number, instantUpload: boolean }> => {
  const res = await request.post("v1/upload/checkVideo", { hash, size }, {})
  if (res.data.code === statusCode.OK) {
    const chunks = res.data.data.chunks || []
    const fileID = res.data.data.fileID || 0
    // 后端返回 [-1] 表示文件已就绪，可以秒传
    if (chunks.length === 1 && chunks[0] === -1) {
      return { chunks: [], fileID, instantUpload: true }
    }
    return { chunks, fileID, instantUpload: false }
  }

  return { chunks: [], fileID: 0, instantUpload: false }
}

const uploadChunkAPI = (formData: FormData, controller?: AbortController) => {
  return request.post("v1/upload/chunkVideo", formData, {
    timeout: CHUNK_TIMEOUT,
    signal: controller?.signal,
  })
}

const mergeUploadedChunksAPI = async (hash: string, fileID: number, size: number, retryCount = 0): Promise<boolean> => {
  try {
    const res = await request.post("v1/upload/mergeVideo", { hash, fileID, size }, {
      timeout: MERGE_TIMEOUT,
    });
    if (res.data.code === statusCode.OK) {
      return true;
    }
    // 服务端返回错误，尝试重试
    if (retryCount < MAX_RETRIES) {
      console.warn(`合并分片失败，${retryCount + 1}/${MAX_RETRIES} 次重试中...`);
      await delay(getRetryDelay(retryCount));
      return mergeUploadedChunksAPI(hash, fileID, size, retryCount + 1);
    }
    return false;
  } catch (error) {
    // 网络错误，尝试重试
    if (retryCount < MAX_RETRIES) {
      console.warn(`合并分片网络错误，${retryCount + 1}/${MAX_RETRIES} 次重试中...`, error);
      await delay(getRetryDelay(retryCount));
      return mergeUploadedChunksAPI(hash, fileID, size, retryCount + 1);
    }
    console.error('合并分片失败，已达到最大重试次数');
    return false;
  }
}

const finishUploadAPI = async ({ hash, fileID, size, action, replaceResourceID, onFinish, onError }: FinishUploadType) => {
  if (await mergeUploadedChunksAPI(hash, fileID, size)) {
    try {
      const res = await request.post(action, { hash, fileID, size, replaceResourceID }, {
        timeout: MERGE_TIMEOUT,
      });
      if (res.data.code === statusCode.OK) {
        onFinish(res.data);
      } else {
        onError();
      }
    } catch (error) {
      console.error('完成上传请求失败', error);
      onError();
    }
  } else {
    onError();
  }
}