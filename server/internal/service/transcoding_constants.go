package service

import "time"

const (
	maxCPUConcurrentTranscoding = 2
	maxGPUConcurrentTranscoding = 3

	maxGpuFailCountThreshold = 3

	// 音频等待：轮询 maxWaitCount 次，每次间隔 audioWaitPollInterval
	audioWaitMaxPollCount      = 3000
	audioWaitPollInterval      = 100 * time.Millisecond
	audioWaitLogEveryPollCount = 100

	ossUploadMaxConcurrency = 10
	ossUploadRetryDelay     = 500 * time.Millisecond
)

