import SparkMD5 from 'spark-md5';

// 分片大小：2MB
const CHUNK_SIZE = 2 * 1024 * 1024;

export const getFileMD5 = (file: File): Promise<string> => {
  return new Promise((resolve, reject) => {
    const spark = new SparkMD5.ArrayBuffer();
    const fileReader = new FileReader();
    const chunks = Math.ceil(file.size / CHUNK_SIZE);
    let currentChunk = 0;

    const loadNext = () => {
      const start = currentChunk * CHUNK_SIZE;
      const end = Math.min(start + CHUNK_SIZE, file.size);
      fileReader.readAsArrayBuffer(file.slice(start, end));
    };

    fileReader.onload = (e) => {
      if (e.target && e.target.result instanceof ArrayBuffer) {
        spark.append(e.target.result);
        currentChunk++;

        if (currentChunk < chunks) {
          loadNext();
        } else {
          resolve(spark.end());
        }
      } else {
        reject(new Error('Failed to read file as ArrayBuffer'));
      }
    };

    fileReader.onerror = () => {
      reject(new Error('File reading failed'));
    };

    loadNext();
  });
};