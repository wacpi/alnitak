package ffmpeg

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// ============================================================
// 帧率 / 时长解析
// ============================================================

// ParseFPS 将帧率字符串（"30000/1001"、"29.97"）转为 float64。
func ParseFPS(fps string) float64 {
	if fps == "" {
		return 0
	}
	parts := strings.Split(fps, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den > 0 {
			return num / den
		}
	}
	v, err := strconv.ParseFloat(fps, 64)
	if err != nil {
		return 0
	}
	return v
}

// ParseFfmpegClockToSeconds 解析 ffmpeg clock 格式 "01:23:45.678" → 秒。
func ParseFfmpegClockToSeconds(clock string) float64 {
	parts := strings.Split(clock, ":")
	if len(parts) != 3 {
		return 0
	}
	h, _ := strconv.ParseFloat(parts[0], 64)
	m, _ := strconv.ParseFloat(parts[1], 64)
	s, _ := strconv.ParseFloat(parts[2], 64)
	return h*3600 + m*60 + s
}

// FfmpegOutputDurationArgs 生成 -t duration 参数。
func FfmpegOutputDurationArgs(seconds float64) []string {
	if seconds <= 0 || math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return nil
	}
	s := strings.TrimRight(strings.TrimRight(strconv.FormatFloat(seconds, 'f', 6, 64), "0"), ".")
	if s == "" {
		return nil
	}
	return []string{"-t", s}
}

// ============================================================
// 码率解析
// ============================================================

// ParseBitrateKbps 将 "8000k" "5000" 转为 int(kbps)。
func ParseBitrateKbps(rate string) int {
	rate = strings.TrimSpace(strings.TrimSuffix(rate, "k"))
	if v, err := strconv.Atoi(rate); err == nil && v > 0 {
		return v
	}
	return 0
}

// FormatBitrateKbps 将 int(kbps) 转为 "8000k"。
func FormatBitrateKbps(rateKbps int) string {
	if rateKbps < 1 {
		rateKbps = 1
	}
	return fmt.Sprintf("%dk", rateKbps)
}

// ============================================================
// B 帧对齐
// ============================================================

// BFramePresentationLeadMs 按帧率估计视频首帧展示延迟（ms）。
func BFramePresentationLeadMs(fps string) int {
	f := ParseFPS(fps)
	if f <= 0 {
		return 0
	}
	ms := int(math.Round(2000.0 / f))
	if ms < 1 {
		return 0
	}
	if ms > 200 {
		return 200
	}
	return ms
}

// AdelayPerChannelArg 生成 adelay 声道参数 "del0|del1|…"。
func AdelayPerChannelArg(delayMs, channels int) string {
	if delayMs <= 0 || channels < 1 {
		return ""
	}
	s := strconv.Itoa(delayMs)
	parts := make([]string, channels)
	for i := range parts {
		parts[i] = s
	}
	return strings.Join(parts, "|")
}

// ============================================================
// 分辨率 / 画质计算
// ============================================================

// ParseQualityInfo 从 quality name 解析宽、高、帧率、码率。
func ParseQualityInfo(quality string) (width, height int, bandwidth int, frameRate string) {
	// "1920x1080_8000k_30"
	parts := strings.Split(quality, "_")
	if len(parts) != 3 {
		return 0, 0, 0, ""
	}
	res := strings.Split(parts[0], "x")
	if len(res) == 2 {
		width, _ = strconv.Atoi(res[0])
		height, _ = strconv.Atoi(res[1])
	}
	bandwidth = ParseBitrateKbps(parts[1]) * 1000
	frameRate = parts[2]
	return
}

// ParseQualitySortKey 解析 quality name 用于排序（高度 > 宽度 > 帧率 > 码率）。
func parseQualitySortKey(quality string) (w, h, fps, br int) {
	parts := strings.Split(quality, "_")
	if len(parts) < 3 {
		return
	}
	res := strings.Split(parts[0], "x")
	if len(res) == 2 {
		w, _ = strconv.Atoi(res[0])
		h, _ = strconv.Atoi(res[1])
	}
	br = ParseBitrateKbps(parts[1])
	fpsVal := ParseFPS(parts[2])
	fps = int(math.Round(fpsVal))
	return
}

// ============================================================
// codec string
// ============================================================

// Avc1CodecString 从 ffprobe profile+level 生成 avc1 编码字符串。
func Avc1CodecString(profile string, level int) string {
	// https://stackoverflow.com/questions/24834877/avc1-codec-string
	profileMap := map[string]int{
		"Baseline": 66,
		"Main":     77,
		"High":     100,
	}
	pi, ok := profileMap[profile]
	if !ok {
		return ""
	}
	levelHex := level * 10
	return fmt.Sprintf("avc1.%02x%02x%02x", pi, byte(levelHex>>8)&0xff, byte(levelHex)&0xff)
}
