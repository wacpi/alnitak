package dto

type TranscodingInfo struct {
	Width      int     // 视频宽度
	Height     int     // 视频高度
	Duration   float64 // 视频时长
	DirName    string  // 目录名称
	OutputDir  string  // 输出位置
	InputFile  string  // 输入文件
	ResourceID uint    // 资源ID
	VideoID    uint    // 视频ID
	CodecName  string  // 视频编码名称
	FPS        string  // 视频帧率
	FPS30      string  // 30帧实际帧率
	FPS60      string  // 60帧实际帧率
	Suffix     string  // 原始文件后缀（如 .mkv）

	// 音频源文件参数（动态探测）
	AudioBitRate    int // 音频码率 (bps)，如 320000、192000
	AudioSampleRate int // 音频采样率 (Hz)，如 48000、44100
	AudioChannels   int // 音频声道数，如 2（立体声）、6（5.1）
}
