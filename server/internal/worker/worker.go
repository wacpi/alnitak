package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"interastral-peace.com/alnitak/internal/config"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/ffmpeg"
	"interastral-peace.com/alnitak/pkg/oss"
)

// =============================================================================
// 常量
// =============================================================================

// Redis Key 常量（与服务端 remote_transcoder.go 保持一致）
const (
	transcodingQueueStream     = "transcoding:queue"
	transcoderGroup            = "transcoder"
	transcodingStatusPrefix    = "transcoding:status:"
	transcodingCancelChannel   = "transcoding:cancel"
	transcodingCompleteChannel = "transcoding:complete"
	deadLetterStream           = "transcoding:deadletter"
)

const (
	// 超过此空闲时间的 pending 消息视为被遗弃（前一个 Worker crash）
	maxPendingIdle = 5 * time.Minute
	// 单个消息最大重试次数（含重新投递），超限进入死信
	maxRetryCount = 3
	// XReadGroup 阻塞时长
	readBlockTime = 5 * time.Second
	// XReadGroup 每次批量拉取上限
	readBatchSize = 10
)

// =============================================================================
// 画质预设
// =============================================================================

// qualityPreset 画质预设（与服务端 qualityPresets 一致）
type qualityPreset struct {
	LongSide   int
	ShortSide  int
	BitrateH   string
	BitrateV   string
	Bitrate60H string
	Bitrate60V string
}

var qualityPresets = []qualityPreset{
	{1920, 1080, "8000k", "5000k", "12000k", "8000k"},
	{1280, 720, "5000k", "3000k", "7500k", "5000k"},
	{854, 480, "2500k", "1500k", "", ""},
	{640, 360, "1000k", "700k", "", ""},
}

// =============================================================================
// Worker 主结构
// =============================================================================

// Worker 远程转码 Worker，从 Redis Stream 消费任务，
// 执行 ffmpeg 编码后将结果上传到 OSS。
type Worker struct {
	rdb         *redis.Client
	storage     oss.Storage
	cfg         *config.Config
	groupID     string
	concurrency int
	cancelSub   *redis.PubSub
	sem         chan struct{}

	mu           sync.RWMutex
	activeVideos map[uint]context.CancelFunc

	// 健康与统计
	startedAt  time.Time
	jobsActive atomic.Int32
	jobsTotal  atomic.Int64
	jobsFailed atomic.Int64
	healthy    atomic.Bool
}

// NewWorker 创建 Worker 实例，建立 Redis/OSS 连接。
func NewWorker(cfg *config.Config, workerID string, concurrency int) (*Worker, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       0,
	})

	var storage oss.Storage
	if cfg.Storage.OssType != "local" {
		storage = oss.InitStorage(cfg.Storage)
	}

	w := &Worker{
		rdb:          rdb,
		storage:      storage,
		cfg:          cfg,
		groupID:      fmt.Sprintf("transcoder-worker-%s", workerID),
		concurrency:  concurrency,
		sem:          make(chan struct{}, concurrency),
		activeVideos: make(map[uint]context.CancelFunc),
		startedAt:    time.Now(),
	}
	w.healthy.Store(true)
	return w, nil
}

// =============================================================================
// 健康检查（供 HTTP 管理接口调用）
// =============================================================================

// HealthStatus Worker 运行时状态快照
type HealthStatus struct {
	Healthy     bool      `json:"healthy"`
	StartedAt   time.Time `json:"startedAt"`
	Uptime      string    `json:"uptime"`
	Concurrency int       `json:"concurrency"`
	JobsActive  int32     `json:"jobsActive"`
	JobsTotal   int64     `json:"jobsTotal"`
	JobsFailed  int64     `json:"jobsFailed"`
	GroupID     string    `json:"groupID"`
}

// Health 返回当前状态快照。
func (w *Worker) Health() HealthStatus {
	return HealthStatus{
		Healthy:     w.healthy.Load(),
		StartedAt:   w.startedAt,
		Uptime:      time.Since(w.startedAt).Truncate(time.Second).String(),
		Concurrency: w.concurrency,
		JobsActive:  w.jobsActive.Load(),
		JobsTotal:   w.jobsTotal.Load(),
		JobsFailed:  w.jobsFailed.Load(),
		GroupID:     w.groupID,
	}
}

// Ready 检查 Worker 是否就绪（Redis 连通 + 正常运行）。
func (w *Worker) Ready() bool {
	if !w.healthy.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return w.rdb.Ping(ctx).Err() == nil
}

// =============================================================================
// Run — 主消费循环
// =============================================================================

// Run 进入消费循环，阻塞至 ctx 取消。
func (w *Worker) Run(ctx context.Context) error {
	// 确保 Redis Stream 消费者组存在（幂等）
	if err := w.rdb.XGroupCreateMkStream(ctx, transcodingQueueStream, transcoderGroup, "0").Err(); err != nil {
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			return fmt.Errorf("create consumer group: %w", err)
		}
	}

	// 启动时恢复 crash 遗留的 pending 消息
	if err := w.recoverPending(ctx); err != nil {
		zap.L().Warn("Pending recovery partial", zap.Error(err))
		// 不阻断启动，部分消息可能被其他消费者持有
	}

	// 订阅取消频道
	w.cancelSub = w.rdb.Subscribe(ctx, transcodingCancelChannel)
	defer w.cancelSub.Close()
	cancelCh := w.cancelSub.Channel()

	var wg sync.WaitGroup
	defer wg.Wait() // 退出时等待所有 goroutine 排空

	zap.L().Info("Worker started",
		zap.String("groupID", w.groupID),
		zap.Int("concurrency", w.concurrency),
	)

	for {
		select {
		case <-ctx.Done():
			zap.L().Info("Worker shutting down, waiting for in-flight jobs...")
			wg.Wait()
			return ctx.Err()

		case msg := <-cancelCh:
			w.handleCancel(ctx, msg.Payload)

		default:
			// 周期性 Redis 连通性检测（每轮循环顺便检查）
			if err := w.rdb.Ping(ctx).Err(); err != nil {
				w.healthy.Store(false)
				zap.L().Error("Redis ping failed, retrying in 5s", zap.Error(err))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
				}
				continue
			}
			w.healthy.Store(true)

			streams, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    transcoderGroup,
				Consumer: w.groupID,
				Streams:  []string{transcodingQueueStream, ">"},
				Count:    readBatchSize,
				Block:    readBlockTime,
			}).Result()
			if err != nil {
				if err == redis.Nil {
					continue
				}
				zap.L().Error("XReadGroup error", zap.Error(err))
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					msgID := msg.ID
					jobJSON, ok := msg.Values["job"].(string)
					if !ok {
						w.rdb.XAck(ctx, transcodingQueueStream, transcoderGroup, msgID)
						continue
					}

					w.sem <- struct{}{}
					wg.Add(1)
					w.jobsActive.Add(1)
					w.jobsTotal.Add(1)
					go func(id string, rawJob string) {
						defer func() { <-w.sem }()
						defer wg.Done()
						defer w.jobsActive.Add(-1)
						if err := w.processJob(ctx, rawJob, id); err != nil {
							w.jobsFailed.Add(1)
							zap.L().Error("Job failed", zap.String("msgID", id), zap.Error(err))
						}
					}(msgID, jobJSON)
				}
			}
		}
	}
}

// =============================================================================
// Pending 消息恢复
// =============================================================================

// recoverPending 启动时处理其他 Worker crash 后遗留的 pending 消息。
func (w *Worker) recoverPending(ctx context.Context) error {
	pendingItems, err := w.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: transcodingQueueStream,
		Group:  transcoderGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
	}).Result()
	if err != nil {
		return fmt.Errorf("xpendingext: %w", err)
	}
	if len(pendingItems) == 0 {
		return nil
	}

	zap.L().Info("Found pending messages for recovery",
		zap.Int("count", len(pendingItems)))

	var expiredIDs []string
	for _, p := range pendingItems {
		if p.Idle < maxPendingIdle {
			continue // 仍在处理中，跳过
		}
		if p.RetryCount >= maxRetryCount {
			// 超过重试上限 → 移入死信
			w.sendToDeadLetter(ctx, p.ID, fmt.Sprintf("retry count %d exceeded max", p.RetryCount))
			w.rdb.XAck(ctx, transcodingQueueStream, transcoderGroup, p.ID)
			zap.L().Warn("Moved to dead letter",
				zap.String("msgID", p.ID),
				zap.Int64("retryCount", p.RetryCount))
			continue
		}
		expiredIDs = append(expiredIDs, p.ID)
	}

	if len(expiredIDs) == 0 {
		return nil
	}

	// 用 XClaim 将这些消息重新分配给当前消费者
	claimed, err := w.rdb.XClaim(ctx, &redis.XClaimArgs{
		Stream:   transcodingQueueStream,
		Group:    transcoderGroup,
		Consumer: w.groupID,
		MinIdle:  maxPendingIdle,
		Messages: expiredIDs,
	}).Result()
	if err != nil {
		return fmt.Errorf("xclaim %d messages: %w", len(expiredIDs), err)
	}

	zap.L().Info("Claimed abandoned messages",
		zap.Int("count", len(claimed)))

	for _, msg := range claimed {
		jobJSON, ok := msg.Values["job"].(string)
		if !ok {
			w.rdb.XAck(ctx, transcodingQueueStream, transcoderGroup, msg.ID)
			continue
		}
		// recovery 阶段使用轻量 goroutine，受全局 sem 约束
		w.sem <- struct{}{}
		w.jobsActive.Add(1)
		w.jobsTotal.Add(1)
		go func(id, raw string) {
			defer func() { <-w.sem }()
			defer w.jobsActive.Add(-1)
			if err := w.processJob(ctx, raw, id); err != nil {
				w.jobsFailed.Add(1)
				zap.L().Error("Recovered job failed", zap.String("msgID", id), zap.Error(err))
			}
		}(msg.ID, jobJSON)
	}

	return nil
}

// sendToDeadLetter 将无法处理的消息写入死信队列以便人工排查。
func (w *Worker) sendToDeadLetter(ctx context.Context, msgID, reason string) {
	_ = w.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: deadLetterStream,
		Values: map[string]interface{}{
			"originalMsgID": msgID,
			"reason":        reason,
			"movedAt":       time.Now().Unix(),
		},
	}).Err()
}

// =============================================================================
// 任务处理
// =============================================================================

// processJob 处理单个转码任务。
func (w *Worker) processJob(ctx context.Context, rawJob, msgID string) error {
	var info dto.TranscodingInfo
	if err := json.Unmarshal([]byte(rawJob), &info); err != nil {
		return fmt.Errorf("unmarshal job: %w", err)
	}

	jobCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 注册当前任务以便取消
	w.mu.Lock()
	w.activeVideos[info.VideoID] = cancel
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		delete(w.activeVideos, info.VideoID)
		w.mu.Unlock()
	}()

	// 1. 下载输入文件
	statusKey := fmt.Sprintf("%s%d", transcodingStatusPrefix, info.ResourceID)
	w.writeStatus(statusKey, "processing", 0, "")

	inputLocal := filepath.Join(os.TempDir(), fmt.Sprintf("alnitak_input_%d%s", info.ResourceID, info.Suffix))
	if err := w.downloadInput(&info, inputLocal); err != nil {
		w.writeStatus(statusKey, "failed", 0, err.Error())
		return fmt.Errorf("download input: %w", err)
	}
	defer os.Remove(inputLocal)
	info.InputFile = inputLocal

	// 2. 计算转码目标
	targets := w.computeTargets(&info)
	totalQualities := len(targets)
	var completed atomic.Int32

	// 3. 并发编码视频 + 音轨
	var encWg sync.WaitGroup
	errCh := make(chan error, totalQualities)

	// 3a. 视频编码（每个 quality 一个 goroutine）
	for _, target := range targets {
		qualityName := target.Resolution + "_" + target.BitrateRate + "_" + target.FpsName
		videoOutput := filepath.Join(os.TempDir(), fmt.Sprintf("alnitak_v_%d_%s.m4s", info.ResourceID, qualityName))

		encWg.Add(1)
		go func(t qualityTarget, outPath, qName string) {
			defer encWg.Done()
			if err := w.encodeQuality(jobCtx, &info, t, outPath); err != nil {
				errCh <- fmt.Errorf("%s: %w", qName, err)
				return
			}
			completed.Add(1)
		}(target, videoOutput, qualityName)
	}

	// 3b. 音频编码（与视频并发）
	var audioFiles []audioFileEntry
	audioErrCh := make(chan error, 1)
	go func() {
		files, err := w.encodeAudio(jobCtx, &info)
		if err == nil {
			audioFiles = files
		}
		audioErrCh <- err
	}()

	// 等待视频完成
	encWg.Wait()
	close(errCh)

	// 等待音频完成
	if audioErr := <-audioErrCh; audioErr != nil {
		w.writeStatus(statusKey, "audio_failed", float64(completed.Load())/float64(totalQualities)*100, audioErr.Error())
		// 主音轨失败视为整体失败
		return fmt.Errorf("audio encode: %w", audioErr)
	}

	// 收集视频编码错误
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		w.writeStatus(statusKey, "partial_fail", float64(completed.Load())/float64(totalQualities)*100,
			fmt.Sprintf("errors: %s", strings.Join(errs, "; ")))
	} else {
		w.writeStatus(statusKey, "encoding_done", 100, "")
	}

	// 4. 上传转码产物到 OSS
	if w.storage != nil {
		w.writeStatus(statusKey, "uploading", 90, "")
		if err := w.uploadOutputs(&info, targets, audioFiles); err != nil {
			w.writeStatus(statusKey, "upload_failed", 90, err.Error())
			return fmt.Errorf("upload outputs: %w", err)
		}
	}

	// 5. 标记完成
	if err := w.rdb.HSet(ctx, statusKey, "status", "completed", "progress", "100").Err(); err != nil {
		zap.L().Error("HSet status failed", zap.String("key", statusKey), zap.Error(err))
	}
	w.rdb.Publish(ctx, transcodingCompleteChannel, fmt.Sprintf("%d", info.ResourceID))

	// 6. ACK
	w.rdb.XAck(ctx, transcodingQueueStream, transcoderGroup, msgID)

	zap.L().Info("Job completed",
		zap.Uint("resourceID", info.ResourceID),
		zap.Uint("videoID", info.VideoID),
		zap.Int("qualities", totalQualities),
	)
	return nil
}

// =============================================================================
// 进度上报
// =============================================================================

// writeStatus 写入转码进度到 Redis Hash
func (w *Worker) writeStatus(key, status string, progress float64, errMsg string) {
	fields := map[string]interface{}{
		"status":   status,
		"progress": fmt.Sprintf("%.1f", progress),
		"updated":  time.Now().Unix(),
	}
	if errMsg != "" {
		fields["error"] = errMsg
	}
	if err := w.rdb.HSet(context.Background(), key, fields).Err(); err != nil {
		zap.L().Error("HSet status failed", zap.String("key", key), zap.Error(err))
	}
}

// =============================================================================
// OSS 操作
// =============================================================================

// downloadInput 从 OSS 下载输入文件到本地临时路径
func (w *Worker) downloadInput(info *dto.TranscodingInfo, localPath string) error {
	if w.storage == nil {
		return fmt.Errorf("no OSS storage configured")
	}
	objectKey := fmt.Sprintf("video/%s/input%s", info.DirName, info.Suffix)
	return w.storage.GetObjectToFile(objectKey, localPath)
}

// uploadOutputs 将转码产物（视频+音频）上传到 OSS。
// audioFiles 由 encodeAudio 返回，含临时路径与 OSS key。
func (w *Worker) uploadOutputs(info *dto.TranscodingInfo, targets []qualityTarget, audioFiles []audioFileEntry) error {
	// 上传视频文件
	for _, target := range targets {
		qName := target.Resolution + "_" + target.BitrateRate + "_" + target.FpsName
		videoFile := filepath.Join(os.TempDir(), fmt.Sprintf("alnitak_v_%d_%s.m4s", info.ResourceID, qName))
		if _, err := os.Stat(videoFile); os.IsNotExist(err) {
			continue
		}

		objectKey := fmt.Sprintf("video/%s/%s_video.m4s", info.DirName, qName)
		if err := w.storage.PutObjectFromFile(objectKey, videoFile); err != nil {
			return fmt.Errorf("upload %s: %w", qName, err)
		}
		defer os.Remove(videoFile)
	}

	// 上传音频文件
	for _, af := range audioFiles {
		if _, err := os.Stat(af.LocalPath); os.IsNotExist(err) {
			continue
		}
		if err := w.storage.PutObjectFromFile(af.OSSKey, af.LocalPath); err != nil {
			return fmt.Errorf("upload audio [%s]: %w", af.OSSKey, err)
		}
		defer os.Remove(af.LocalPath)
	}

	return nil
}

// audioFileEntry 音频上传条目
type audioFileEntry struct {
	LocalPath string // 本地临时文件路径
	OSSKey    string // OSS 目标路径
	Language  string // 语言代码
}

// =============================================================================
// 音频编码
// =============================================================================

// workerAudioFileName 生成音频文件名（与服务端 audioFileNameForTrack 保持一致）。
func workerAudioFileName(language string) string {
	if language == "" || language == "und" {
		return "audio.m4s"
	}
	return "audio_" + language + ".m4s"
}

// encodeAudio 编码音轨（单音轨或多音轨），返回上传条目列表。
//   - 多音轨模式：每个 AudioStream 独立编码，主音轨（第一条）失败视为整体失败。
//   - 单音轨模式：使用 info.AudioBitRate / AudioSampleRate / AudioChannels 回退。
func (w *Worker) encodeAudio(ctx context.Context, info *dto.TranscodingInfo) ([]audioFileEntry, error) {
	leadMs := ffmpeg.BFramePresentationLeadMs(info.FPS30)
	var files []audioFileEntry

	if len(info.AudioStreams) > 0 {
		// 多音轨模式
		for i, stream := range info.AudioStreams {
			audioFileName := workerAudioFileName(stream.Language)
			audioOutput := filepath.Join(os.TempDir(),
				fmt.Sprintf("alnitak_a_%d_%s.m4s", info.ResourceID, stream.Language))

			args := ffmpeg.AudioTrackEncodeArgs(info.InputFile, stream.StreamIndex,
				stream.BitRate, stream.SampleRate, stream.Channels,
				info.Duration, leadMs)
			args = append(args, "-y", audioOutput)

			if err := ffmpeg.EncodeAudio(ctx, args); err != nil {
				if i == 0 {
					return nil, fmt.Errorf("primary audio track %s: %w", stream.Language, err)
				}
				// 附加音轨失败仅记日志
				zap.L().Warn("Secondary audio track encode failed",
					zap.String("language", stream.Language),
					zap.Error(err))
				continue
			}

			ossKey := fmt.Sprintf("video/%s/%s", info.DirName, audioFileName)
			files = append(files, audioFileEntry{
				LocalPath: audioOutput,
				OSSKey:    ossKey,
				Language:  stream.Language,
			})
		}
		return files, nil
	}

	// 单音轨模式（向后兼容）
	audioOutput := filepath.Join(os.TempDir(),
		fmt.Sprintf("alnitak_a_%d.m4s", info.ResourceID))

	args := ffmpeg.AudioEncodeArgs(info.InputFile, info.AudioBitRate,
		info.AudioSampleRate, info.AudioChannels,
		info.Duration, leadMs)
	args = append(args, "-y", audioOutput)

	if err := ffmpeg.EncodeAudio(ctx, args); err != nil {
		return nil, fmt.Errorf("audio encode: %w", err)
	}

	files = append(files, audioFileEntry{
		LocalPath: audioOutput,
		OSSKey:    fmt.Sprintf("video/%s/audio.m4s", info.DirName),
		Language:  "und",
	})
	return files, nil
}

// =============================================================================
// 视频编码
// =============================================================================

// qualityTarget 单画质编码目标
type qualityTarget struct {
	Resolution  string // e.g. "1920x1080"
	BitrateRate string // e.g. "8000k"
	FPS         string // e.g. "30000/1000"
	FpsName     string // e.g. "30"
}

// computeTargets 计算转码目标列表（与服务端 getTranscodingTarget 逻辑一致）
func (w *Worker) computeTargets(info *dto.TranscodingInfo) []qualityTarget {
	maxLevel := getMaxQualityLevel(info.Width, info.Height)
	isPortrait := info.Width < info.Height

	var maxPreset qualityPreset
	for _, p := range qualityPresets {
		if p.ShortSide == maxLevel {
			maxPreset = p
			break
		}
	}
	if maxPreset.ShortSide == 0 {
		return nil
	}

	sourceMaxKbps := info.VideoBitRate / 1000
	if sourceMaxKbps <= 0 {
		sourceMaxKbps = parseBitrateKbps(maxPreset.BitrateH)
	}

	enable60 := w.cfg.Transcoding.Generate1080p60 && info.FPS60 != ""

	var targets []qualityTarget
	for _, preset := range qualityPresets {
		if preset.ShortSide > maxLevel {
			continue
		}
		wRes, hRes := calcResolution(info.Width, info.Height, preset.ShortSide)
		resStr := fmt.Sprintf("%dx%d", wRes, hRes)

		currKbps30 := getPresetBitrateKbps(preset, isPortrait, false)
		currKbps60 := getPresetBitrateKbps(preset, isPortrait, true)

		if preset.ShortSide == maxLevel && enable60 && currKbps60 > 0 {
			dynKbps60 := scaleBitrateBySource(sourceMaxKbps, currKbps60, currKbps60)
			targets = append(targets, qualityTarget{
				Resolution: resStr, BitrateRate: formatBitrateKbps(dynKbps60),
				FPS: info.FPS60, FpsName: "60",
			})
		}
		dynKbps30 := scaleBitrateBySource(sourceMaxKbps, currKbps30, currKbps30)
		targets = append(targets, qualityTarget{
			Resolution: resStr, BitrateRate: formatBitrateKbps(dynKbps30),
			FPS: info.FPS30, FpsName: "30",
		})
	}
	return targets
}

// encodeQuality 执行单画质视频编码
func (w *Worker) encodeQuality(ctx context.Context, info *dto.TranscodingInfo, target qualityTarget, outputPath string) error {
	useGpu := w.cfg.Transcoding.UseGpu
	useHevc := w.cfg.Transcoding.UseH265
	useAv1 := w.cfg.Transcoding.UseAv1

	args := ffmpeg.VideoEncodeArgs(info.InputFile, target.Resolution, target.BitrateRate, target.FPS, info.Duration, useGpu, useAv1, useHevc)

	numCPU := runtime.NumCPU()
	threads := 4
	if !useGpu {
		threads = (numCPU - 2) / w.concurrency
		if threads < 1 {
			threads = 1
		}
		if threads > 8 {
			threads = 8
		}
	}

	args = append(args, "-progress", "pipe:1", "-nostats",
		"-threads", strconv.Itoa(threads),
		"-y", outputPath)

	_, err := ffmpeg.EncodeVideo(ctx, args, info.Duration, func(pct float64) {
		statusKey := fmt.Sprintf("%s%d", transcodingStatusPrefix, info.ResourceID)
		qName := target.Resolution + "_" + target.BitrateRate + "_" + target.FpsName
		_ = w.rdb.HSet(ctx, statusKey,
			fmt.Sprintf("progress_%s", qName), fmt.Sprintf("%.1f", pct),
		).Err()
	})
	return err
}

// =============================================================================
// 取消
// =============================================================================

// handleCancel 处理取消信号
func (w *Worker) handleCancel(ctx context.Context, videoIDStr string) {
	videoID, err := strconv.ParseUint(videoIDStr, 10, 64)
	if err != nil {
		return
	}

	w.mu.RLock()
	cancel, ok := w.activeVideos[uint(videoID)]
	w.mu.RUnlock()

	if ok {
		cancel()
		zap.L().Info("Cancelled job", zap.Uint64("videoID", videoID))
	}
}

// =============================================================================
// 纯函数：画质/码率计算（与服务端一致）
// =============================================================================

func getMaxQualityLevel(width, height int) int {
	shortSide, longSide := width, height
	if height < width {
		shortSide, longSide = height, width
	}
	if longSide >= 1920 && shortSide >= 1000 {
		return 1080
	}
	if shortSide >= 1080 {
		return 1080
	}
	if shortSide >= 720 {
		return 720
	}
	if shortSide >= 480 {
		return 480
	}
	return 360
}

func calcResolution(srcWidth, srcHeight, targetShortSide int) (w, h int) {
	isPortrait := srcWidth < srcHeight
	srcAspect := float64(srcWidth) / float64(srcHeight)
	if isPortrait {
		w = targetShortSide
		h = int(math.Round(float64(w) / srcAspect))
	} else {
		h = targetShortSide
		w = int(math.Round(float64(h) * srcAspect))
	}
	return w &^ 1, h &^ 1
}

func getPresetBitrateKbps(preset qualityPreset, isPortrait, fps60 bool) int {
	var bitrate string
	if fps60 {
		bitrate = preset.Bitrate60H
		if isPortrait {
			bitrate = preset.Bitrate60V
		}
		if kbps := parseBitrateKbps(bitrate); kbps > 0 {
			return kbps
		}
	}
	bitrate = preset.BitrateH
	if isPortrait {
		bitrate = preset.BitrateV
	}
	return parseBitrateKbps(bitrate)
}

func scaleBitrateBySource(sourceMaxKbps, maxPresetKbps, currentPresetKbps int) int {
	if sourceMaxKbps <= 0 {
		return currentPresetKbps
	}
	if maxPresetKbps <= 0 || currentPresetKbps <= 0 {
		return sourceMaxKbps
	}
	if currentPresetKbps >= maxPresetKbps {
		return sourceMaxKbps
	}
	scaled := int(math.Round(float64(sourceMaxKbps) * float64(currentPresetKbps) / float64(maxPresetKbps)))
	if scaled < 200 {
		scaled = 200
	}
	if scaled > sourceMaxKbps {
		scaled = sourceMaxKbps
	}
	return scaled
}

func parseBitrateKbps(rate string) int {
	rate = strings.TrimSpace(strings.TrimSuffix(rate, "k"))
	if v, err := strconv.Atoi(rate); err == nil && v > 0 {
		return v
	}
	return 0
}

func formatBitrateKbps(rateKbps int) string {
	if rateKbps < 1 {
		rateKbps = 1
	}
	return fmt.Sprintf("%dk", rateKbps)
}
