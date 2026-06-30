interface UploadOptionsType {
  name: string;
  file: File;
  action: string;
  replaceResourceID?: number; // 替换资源时传旧资源ID
  onProgress: (percent: number) => void
  onFinish: (data?: any) => void
  onError: (error?: any) => void
}

interface FinishUploadType {
  hash: string;
  fileID: number;
  size: number;
  action: string;
  replaceResourceID?: number; // 替换资源时传旧资源ID
  onFinish: (data?: any) => void
  onError: (error?: any) => void
}