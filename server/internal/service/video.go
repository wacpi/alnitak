package service

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

func UploadVideoInfo(ctx *gin.Context, uploadVideoReq dto.UploadVideoReq) error {
	userId := ctx.GetUint("userId")
	v, _ := FindVideoById(uploadVideoReq.Vid)
	if cache.GetUploadImage(uploadVideoReq.Cover) != userId {
		// 查询是否与旧封面图一致
		if v.Cover != uploadVideoReq.Cover {
			return errors.New("文件链接无效")
		}
	}

	if v.PartitionId != 0 {
		return errors.New("视频信息已存在")
	}

	if !IsSubpartition(uploadVideoReq.PartitionId, global.CONTENT_TYPE_VIDEO) {
		return errors.New("分区不存在")
	}

	if err := global.Mysql.Model(&model.Video{}).Where("id = ?", uploadVideoReq.Vid).Updates(
		map[string]interface{}{
			"title":        uploadVideoReq.Title,
			"cover":        uploadVideoReq.Cover,
			"desc":         uploadVideoReq.Desc,
			"tags":         uploadVideoReq.Tags,
			"copyright":    uploadVideoReq.Copyright,
			"partition_id": uploadVideoReq.PartitionId,
			"status":       getVideoStatus(uploadVideoReq.Vid),
		},
	).Error; err != nil {
		utils.ErrorLog("修改视频失败", "video", err.Error())
		return errors.New("修改失败")
	}

	// 上传视频信息后删除缓存，让下次查询时重新加载最新数据
	cache.DelVideoInfo(uploadVideoReq.Vid)

	return nil
}

func GetVideoStatus(ctx *gin.Context, vid uint) (video vo.VideoStatusResp, err error) {
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Video{}).Select(vo.VIDEO_STATUS_FIELD).Where("id = ? and uid = ?", vid, userId).Scan(&video)
	if video.ID == 0 {
		return video, errors.New("视频不存在")
	}

	//查询分区下的视频资源
	video.Resources = GetReviewResourceList(vid)

	return video, nil
}

// GetVideoFile 获取视频文件（返回 DASH MPD / HLS M3U8 / 播放信息 JSON）
func GetVideoFile(ctx *gin.Context, resourceId uint, quality, format string) (string, error) {
	// ========== HLS 子清单请求（m3u8video / m3u8audio） ==========
	// 由主清单引用，key 由 query 参数传入（复用已有的 key）
	if format == "m3u8video" || format == "m3u8audio" {
		existingKey := ctx.Query("key")
		if existingKey == "" {
			return "", errors.New("missing key")
		}
		// 不校验 resourceId（主清单已校验过），通过 quality 查找索引
		var file model.VideoIndexFile
		if resourceId > 0 {
			if err := global.Mysql.Where("resource_id = ? AND quality = ?", resourceId, quality).First(&file).Error; err != nil {
				return "", errors.New("视频索引不存在")
			}
		} else {
			return "", errors.New("resourceId 无效")
		}
		// 确保 key 对应的目录缓存仍有效
		if cache.GetVideoSlice(existingKey) == "" {
			cache.SetVideoSlice(existingKey, file.DirName)
		}
		if format == "m3u8video" {
			return buildM3U8VideoSegmentBase(&file, existingKey)
		}
		return buildM3U8AudioSegmentBase(&file, existingKey)
	}

	// ========== 常规请求 ==========
	if !IsResourceExist(resourceId) {
		return "", errors.New("资源不存在")
	}

	key := uuid.New().String()

	// 从 VideoIndexFile 元数据动态生成
	var file model.VideoIndexFile
	err := global.Mysql.Where("resource_id = ? AND quality = ?", resourceId, quality).First(&file).Error
	if err != nil {
		return "", errors.New("视频索引不存在")
	}

	cache.SetVideoSlice(key, file.DirName)

	// B站风格：SegmentBase 模式（音视频分离）
	if file.IsSegmentBase() {
		// format=m3u8 返回 HLS v7 主清单（Safari/iOS 兼容）
		if format == "m3u8" {
			return buildM3U8MasterSegmentBase(&file, resourceId, key), nil
		}
		// format=dash 或 format=mpd 返回原始 MPD XML
		if format == "dash" || format == "mpd" {
			return buildMPDSegmentBase(&file, key), nil
		}
		// 默认返回 JSON 格式（类似B站 playurl 接口）
		return buildPlayURLJSON(&file, key), nil
	}

	// 兼容模式：SegmentList 切片模式
	if file.IsSegmentList() {
		if format == "mpd" {
			return buildMPDSegmentList(&file, key), nil
		}
		return buildM3U8SegmentList(&file, key), nil
	}

	// 兼容旧数据（Content字段存储完整m3u8）
	return buildFromLegacyContent(&file, key, format)
}

// =====================================================
// B站风格：SegmentBase 模式（音视频分离，字节范围请求）
// =====================================================

// buildMPDSegmentBase 生成 DASH MPD（SegmentBase 模式，类似B站）
func buildMPDSegmentBase(file *model.VideoIndexFile, key string) string {
	// ISO 8601 时长格式
	durationStr := formatDuration(file.TotalDuration)

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="%s" minBufferTime="PT1.5S" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011">`, durationStr))
	sb.WriteString("\n  <Period>\n")

	// ========== 视频 AdaptationSet ==========
	sb.WriteString(`    <AdaptationSet mimeType="video/mp4" segmentAlignment="true" startWithSAP="1">`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`      <Representation id="%s" bandwidth="%d" width="%d" height="%d" frameRate="%.3f" codecs="%s">`,
		file.Quality, file.VideoBandwidth, file.Width, file.Height, file.FrameRate, file.VideoCodec))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`        <BaseURL>/api/v1/video/stream/%s?key=%s</BaseURL>`, file.VideoFile, key))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`        <SegmentBase indexRange="%s">`, file.VideoIndexRange))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`          <Initialization range="%s"/>`, file.VideoInitRange))
	sb.WriteString("\n")
	sb.WriteString("        </SegmentBase>\n")
	sb.WriteString("      </Representation>\n")
	sb.WriteString("    </AdaptationSet>\n")

	// ========== 音频 AdaptationSet ==========
	sb.WriteString(`    <AdaptationSet mimeType="audio/mp4" segmentAlignment="true" startWithSAP="1">`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`      <Representation id="audio" bandwidth="%d" codecs="%s">`,
		file.AudioBandwidth, file.AudioCodec))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`        <BaseURL>/api/v1/video/stream/%s?key=%s</BaseURL>`, file.AudioFile, key))
	sb.WriteString("\n")
	if file.AudioIndexRange != "" {
		sb.WriteString(fmt.Sprintf(`        <SegmentBase indexRange="%s">`, file.AudioIndexRange))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`          <Initialization range="%s"/>`, file.AudioInitRange))
		sb.WriteString("\n")
		sb.WriteString("        </SegmentBase>\n")
	}
	sb.WriteString("      </Representation>\n")
	sb.WriteString("    </AdaptationSet>\n")

	sb.WriteString("  </Period>\n")
	sb.WriteString("</MPD>")

	return sb.String()
}

// buildPlayURLJSON 生成类似B站的 playurl JSON 格式
func buildPlayURLJSON(file *model.VideoIndexFile, key string) string {
	// 构建类似B站的响应格式
	videoURL := fmt.Sprintf("/api/v1/video/stream/%s?key=%s", file.VideoFile, key)
	audioURL := fmt.Sprintf("/api/v1/video/stream/%s?key=%s", file.AudioFile, key)

	json := fmt.Sprintf(`{
  "code": 0,
  "message": "OK",
  "data": {
    "quality": "%s",
    "duration": %.3f,
    "dash": {
      "duration": %.0f,
      "minBufferTime": 1.5,
      "video": [{
        "id": "%s",
        "baseUrl": "%s",
        "bandwidth": %d,
        "mimeType": "video/mp4",
        "codecs": "%s",
        "width": %d,
        "height": %d,
        "frameRate": "%.3f",
        "SegmentBase": {
          "Initialization": "%s",
          "indexRange": "%s"
        }
      }],
      "audio": [{
        "id": "audio",
        "baseUrl": "%s",
        "bandwidth": %d,
        "mimeType": "audio/mp4",
        "codecs": "%s",
        "SegmentBase": {
          "Initialization": "%s",
          "indexRange": "%s"
        }
      }]
    }
  }
}`,
		file.Quality, file.TotalDuration, file.TotalDuration,
		file.Quality, videoURL, file.VideoBandwidth, file.VideoCodec, file.Width, file.Height, file.FrameRate,
		file.VideoInitRange, file.VideoIndexRange,
		audioURL, file.AudioBandwidth, file.AudioCodec,
		file.AudioInitRange, file.AudioIndexRange,
	)

	return json
}

// =====================================================
// HLS v7 over fMP4（SegmentBase 模式的 Safari/iOS 兼容）
// =====================================================

// sidxEntry 表示 sidx box 中的一个引用条目（一个 fragment）
type sidxEntry struct {
	Offset   int64   // fragment 在文件中的字节偏移
	Size     int64   // fragment 的字节大小
	Duration float64 // fragment 的时长（秒）
}

// parseSidxBox 从 fMP4 文件中解析 sidx box，提取所有 fragment 的字节范围
// sidx box 结构参考 ISO 14496-12
func parseSidxBox(filePath string) ([]sidxEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}
	fileSize := fileInfo.Size()

	// 遍历顶层 box，找到 sidx
	offset := int64(0)
	header := make([]byte, 8)

	for offset < fileSize {
		if _, err := file.ReadAt(header, offset); err != nil {
			break
		}

		boxSize := int64(binary.BigEndian.Uint32(header[0:4]))
		boxType := string(header[4:8])

		// 扩展 size
		if boxSize == 1 {
			extHeader := make([]byte, 8)
			if _, err := file.ReadAt(extHeader, offset+8); err != nil {
				break
			}
			boxSize = int64(binary.BigEndian.Uint64(extHeader))
		}
		if boxSize == 0 {
			boxSize = fileSize - offset
		}

		if boxType == "sidx" {
			return decodeSidxBox(file, offset, boxSize)
		}

		// 遇到 moof 就停止搜索
		if boxType == "moof" {
			break
		}

		offset += boxSize
	}

	return nil, fmt.Errorf("sidx box not found in %s", filePath)
}

// decodeSidxBox 解码 sidx box 内容
func decodeSidxBox(reader io.ReaderAt, boxOffset, boxSize int64) ([]sidxEntry, error) {
	data := make([]byte, boxSize)
	if _, err := reader.ReadAt(data, boxOffset); err != nil {
		return nil, err
	}

	// box header: 8 bytes (size + type)
	pos := 8

	// FullBox header: version (1 byte) + flags (3 bytes)
	version := data[pos]
	pos += 4 // skip version + flags

	// reference_ID (4 bytes)
	pos += 4

	// timescale (4 bytes)
	var timescale uint32
	timescale = binary.BigEndian.Uint32(data[pos : pos+4])
	pos += 4

	if timescale == 0 {
		return nil, fmt.Errorf("sidx timescale is 0")
	}

	// earliest_presentation_time + first_offset
	var firstOffset int64
	if version == 0 {
		// 32-bit each
		pos += 4 // earliest_presentation_time
		firstOffset = int64(binary.BigEndian.Uint32(data[pos : pos+4]))
		pos += 4
	} else {
		// 64-bit each
		pos += 8 // earliest_presentation_time
		firstOffset = int64(binary.BigEndian.Uint64(data[pos : pos+8]))
		pos += 8
	}

	// reserved (2 bytes) + reference_count (2 bytes)
	pos += 2 // reserved
	refCount := int(binary.BigEndian.Uint16(data[pos : pos+2]))
	pos += 2

	// 第一个 fragment 的偏移 = sidx box 结束位置 + first_offset
	// ISO 14496-12: anchor_point = first byte after sidx box, first_offset 是额外偏移
	fragmentOffset := boxOffset + boxSize + firstOffset

	entries := make([]sidxEntry, 0, refCount)

	for i := 0; i < refCount; i++ {
		if pos+12 > len(data) {
			break
		}

		// reference_type (1 bit) + referenced_size (31 bits)
		refField := binary.BigEndian.Uint32(data[pos : pos+4])
		// referenceType := (refField >> 31) & 1  // 0 = media, 1 = index
		referencedSize := int64(refField & 0x7FFFFFFF)
		pos += 4

		// subsegment_duration (4 bytes)
		subsegmentDuration := binary.BigEndian.Uint32(data[pos : pos+4])
		pos += 4

		// SAP fields (4 bytes) - skip
		pos += 4

		duration := float64(subsegmentDuration) / float64(timescale)

		entries = append(entries, sidxEntry{
			Offset:   fragmentOffset,
			Size:     referencedSize,
			Duration: duration,
		})

		fragmentOffset += referencedSize
	}

	return entries, nil
}

// buildM3U8MasterSegmentBase 为 SegmentBase 模式生成 HLS v7 主播放列表（Master Playlist）
// Safari/iOS 通过此清单实现音视频分离播放
// format=m3u8 返回主清单，内部引用 m3u8video / m3u8audio 子清单
func buildM3U8MasterSegmentBase(file *model.VideoIndexFile, resourceId uint, key string) string {
	var sb strings.Builder

	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:7\n")
	sb.WriteString("\n")

	// 音频组：引用音频子清单
	sb.WriteString(fmt.Sprintf(
		"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"Audio\",DEFAULT=YES,AUTOSELECT=YES,URI=\"/api/v1/video/getVideoFile?resourceId=%d&quality=%s&format=m3u8audio&key=%s\"\n",
		resourceId, file.Quality, key))
	sb.WriteString("\n")

	// 视频流：引用视频子清单
	sb.WriteString(fmt.Sprintf("#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,FRAME-RATE=%.3f,CODECS=\"%s,%s\",AUDIO=\"audio\"\n",
		file.VideoBandwidth+file.AudioBandwidth, file.Width, file.Height, file.FrameRate,
		file.VideoCodec, file.AudioCodec))
	sb.WriteString(fmt.Sprintf("/api/v1/video/getVideoFile?resourceId=%d&quality=%s&format=m3u8video&key=%s\n",
		resourceId, file.Quality, key))

	return sb.String()
}

// buildM3U8VideoSegmentBase 为视频流生成 HLS v7 媒体播放列表（从文件实时解析 sidx）
func buildM3U8VideoSegmentBase(file *model.VideoIndexFile, key string) (string, error) {
	videoPath := "./upload/video/" + file.DirName + "/" + file.VideoFile
	entries, err := parseSidxBox(videoPath)
	if err != nil {
		return "", fmt.Errorf("parse video sidx failed: %w", err)
	}
	fmt.Printf("[HLS-DEBUG] video sidx: %d entries, initRange=%s\n", len(entries), file.VideoInitRange)
	if len(entries) > 0 {
		fmt.Printf("[HLS-DEBUG] video first entry: offset=%d size=%d dur=%.3f\n", entries[0].Offset, entries[0].Size, entries[0].Duration)
		last := entries[len(entries)-1]
		fmt.Printf("[HLS-DEBUG] video last entry: offset=%d size=%d dur=%.3f\n", last.Offset, last.Size, last.Duration)
	}
	streamURL := fmt.Sprintf("/api/v1/video/stream/%s?key=%s", file.VideoFile, key)
	m3u8 := buildByteRangeM3U8(entries, streamURL, file.VideoInitRange)
	fmt.Printf("[HLS-DEBUG] video m3u8 length: %d bytes\n", len(m3u8))
	return m3u8, nil
}

// buildM3U8AudioSegmentBase 为音频流生成 HLS v7 媒体播放列表（从文件实时解析 sidx）
func buildM3U8AudioSegmentBase(file *model.VideoIndexFile, key string) (string, error) {
	audioPath := "./upload/video/" + file.DirName + "/" + file.AudioFile
	entries, err := parseSidxBox(audioPath)
	if err != nil {
		return "", fmt.Errorf("parse audio sidx failed: %w", err)
	}
	fmt.Printf("[HLS-DEBUG] audio sidx: %d entries, initRange=%s\n", len(entries), file.AudioInitRange)
	if len(entries) > 0 {
		fmt.Printf("[HLS-DEBUG] audio first entry: offset=%d size=%d dur=%.3f\n", entries[0].Offset, entries[0].Size, entries[0].Duration)
	}
	streamURL := fmt.Sprintf("/api/v1/video/stream/%s?key=%s", file.AudioFile, key)
	return buildByteRangeM3U8(entries, streamURL, file.AudioInitRange), nil
}

// buildByteRangeM3U8 从 sidx 条目生成带 EXT-X-BYTERANGE 的 HLS v7 媒体播放列表
func buildByteRangeM3U8(entries []sidxEntry, streamURL, initRange string) string {
	var sb strings.Builder

	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:7\n")

	// 计算最大 segment 时长
	maxDuration := 0.0
	for _, e := range entries {
		if e.Duration > maxDuration {
			maxDuration = e.Duration
		}
	}
	sb.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(maxDuration)+1))
	sb.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	sb.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	// EXT-X-MAP: 初始化段（ftyp + moov）
	// initRange 格式为 "0-927"，需要转换为 BYTERANGE 的 length@offset
	initParts := strings.Split(initRange, "-")
	if len(initParts) == 2 {
		start := parseRangeInt(initParts[0])
		end := parseRangeInt(initParts[1])
		length := end - start + 1
		sb.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"%s\",BYTERANGE=\"%d@%d\"\n", streamURL, length, start))
	}

	// 每个 fragment 作为一个 segment
	for _, entry := range entries {
		sb.WriteString(fmt.Sprintf("#EXTINF:%.6f,\n", entry.Duration))
		sb.WriteString(fmt.Sprintf("#EXT-X-BYTERANGE:%d@%d\n", entry.Size, entry.Offset))
		sb.WriteString(streamURL + "\n")
	}

	sb.WriteString("#EXT-X-ENDLIST\n")
	return sb.String()
}

// parseRangeInt 将范围字符串解析为 int64
func parseRangeInt(s string) int64 {
	val := int64(0)
	for _, c := range s {
		if c >= '0' && c <= '9' {
			val = val*10 + int64(c-'0')
		}
	}
	return val
}

// =====================================================
// 兼容模式：SegmentList 切片模式
// =====================================================

// buildM3U8SegmentList 从切片元数据生成 m3u8 (HLS)
func buildM3U8SegmentList(file *model.VideoIndexFile, key string) string {
	var sb strings.Builder

	isFMP4 := file.InitFile != ""

	if isFMP4 {
		sb.WriteString("#EXTM3U\n")
		sb.WriteString("#EXT-X-VERSION:7\n")
	} else {
		sb.WriteString("#EXTM3U\n")
		sb.WriteString("#EXT-X-VERSION:3\n")
	}

	sb.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", int(file.SegmentDuration)+1))
	sb.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	sb.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	if isFMP4 {
		sb.WriteString(fmt.Sprintf("#EXT-X-MAP:URI=\"/api/v1/video/slice/%s?key=%s\"\n", file.InitFile, key))
	}

	for i := 0; i < file.SegmentCount; i++ {
		duration := file.SegmentDuration
		if i == file.SegmentCount-1 {
			duration = file.LastSegmentDuration
		}

		ext := ".ts"
		if isFMP4 {
			ext = ".m4s"
		}
		fileName := fmt.Sprintf("%s_%05d%s", file.Quality, i, ext)

		sb.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", duration))
		sb.WriteString(fmt.Sprintf("/api/v1/video/slice/%s?key=%s\n", fileName, key))
	}

	sb.WriteString("#EXT-X-ENDLIST\n")
	return sb.String()
}

// buildMPDSegmentList 从切片元数据生成 mpd (DASH SegmentList)
func buildMPDSegmentList(file *model.VideoIndexFile, key string) string {
	durationStr := formatDuration(file.TotalDuration)

	codec := file.Codec
	if codec == "" {
		codec = "avc1.640028"
	}

	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" type="static" mediaPresentationDuration="%s" minBufferTime="PT2S" profiles="urn:mpeg:dash:profile:isoff-on-demand:2011">`, durationStr))
	sb.WriteString("\n  <Period>\n")
	sb.WriteString(`    <AdaptationSet mimeType="video/mp4" segmentAlignment="true" startWithSAP="1">`)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(`      <Representation id="%s" bandwidth="%d" width="%d" height="%d" frameRate="%.0f" codecs="%s">`,
		file.Quality, file.Bandwidth, file.Width, file.Height, file.FrameRate, codec))
	sb.WriteString("\n")

	if file.InitFile != "" {
		sb.WriteString(fmt.Sprintf(`        <SegmentList timescale="1000" duration="%d">`, int(file.SegmentDuration*1000)))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`          <Initialization sourceURL="/api/v1/video/slice/%s?key=%s"/>`, file.InitFile, key))
		sb.WriteString("\n")
	} else {
		sb.WriteString(fmt.Sprintf(`        <SegmentList timescale="1000" duration="%d">`, int(file.SegmentDuration*1000)))
		sb.WriteString("\n")
	}

	for i := 0; i < file.SegmentCount; i++ {
		ext := ".m4s"
		if file.InitFile == "" {
			ext = ".ts"
		}
		fileName := fmt.Sprintf("%s_%05d%s", file.Quality, i, ext)
		sb.WriteString(fmt.Sprintf(`          <SegmentURL media="/api/v1/video/slice/%s?key=%s"/>`, fileName, key))
		sb.WriteString("\n")
	}

	sb.WriteString("        </SegmentList>\n")
	sb.WriteString("      </Representation>\n")
	sb.WriteString("    </AdaptationSet>\n")
	sb.WriteString("  </Period>\n")
	sb.WriteString("</MPD>")

	return sb.String()
}

// =====================================================
// 兼容旧数据
// =====================================================

// buildFromLegacyContent 兼容旧数据（Content字段存储完整m3u8）
func buildFromLegacyContent(file *model.VideoIndexFile, key, format string) (string, error) {
	if file.Content == "" {
		return "", fmt.Errorf("no content found")
	}

	if format == "mpd" {
		return "", fmt.Errorf("legacy data only supports m3u8")
	}

	var res strings.Builder
	for _, line := range strings.Split(file.Content, "\n") {
		if strings.Contains(line, ".ts") || strings.Contains(line, ".m4s") {
			res.WriteString("/api/v1/video/slice/" + line + "?key=" + key + "\n")
		} else {
			res.WriteString(line + "\n")
		}
	}
	return res.String(), nil
}

// formatDuration 格式化时长为 ISO 8601 格式
func formatDuration(seconds float64) string {
	totalSeconds := int(seconds)
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	secs := totalSeconds % 60
	return fmt.Sprintf("PT%dH%dM%dS", hours, minutes, secs)
}

// 获取视频文件（后台管理）
func GetVideoFileManage(ctx *gin.Context, resourceId uint, quality, format string) (string, error) {
	// HLS 子清单请求
	if format == "m3u8video" || format == "m3u8audio" {
		existingKey := ctx.Query("key")
		if existingKey == "" || resourceId == 0 {
			return "", errors.New("参数无效")
		}
		var file model.VideoIndexFile
		if err := global.Mysql.Where("resource_id = ? AND quality = ?", resourceId, quality).First(&file).Error; err != nil {
			return "", errors.New("视频索引不存在")
		}
		if cache.GetVideoSlice(existingKey) == "" {
			cache.SetVideoSlice(existingKey, file.DirName)
		}
		if format == "m3u8video" {
			return buildM3U8VideoSegmentBase(&file, existingKey)
		}
		return buildM3U8AudioSegmentBase(&file, existingKey)
	}

	key := uuid.New().String()

	var file model.VideoIndexFile
	err := global.Mysql.Where("resource_id = ? AND quality = ?", resourceId, quality).First(&file).Error
	if err != nil {
		return "", errors.New("视频索引不存在")
	}

	cache.SetVideoSlice(key, file.DirName)

	// B站风格：SegmentBase 模式
	if file.IsSegmentBase() {
		if format == "m3u8" {
			return buildM3U8MasterSegmentBase(&file, resourceId, key), nil
		}
		if format == "dash" || format == "mpd" {
			return buildMPDSegmentBase(&file, key), nil
		}
		return buildPlayURLJSON(&file, key), nil
	}

	// 兼容模式：SegmentList 切片模式
	if file.IsSegmentList() {
		if format == "mpd" {
			return buildMPDSegmentList(&file, key), nil
		}
		return buildM3U8SegmentList(&file, key), nil
	}

	// 兼容旧数据
	return buildFromLegacyContent(&file, key, format)
}

// 获取视频切所在文件目录
func GetVideoSliceDir(key string) string {
	return cache.GetVideoSlice(key)
}

// 获取自己上传的视频
func GetUploadVideoList(ctx *gin.Context, page, pageSize int) (total int64, videos []vo.UploadVideoResp) {
	userId := ctx.GetUint("userId")

	global.Mysql.Model(&model.Video{}).Where("uid = ?", userId).Count(&total)
	global.Mysql.Model(&model.Video{}).Select(vo.UPLOAD_VIDEO_FIELD).
		Where("uid = ?", userId).Limit(pageSize).Offset((page - 1) * pageSize).Scan(&videos)

	// 更新播放量数据
	for i := 0; i < len(videos); i++ {
		videos[i].Clicks += GetVideoClicks(videos[i].ID)
	}

	return
}

func EditVideoInfo(ctx *gin.Context, editVideoReq dto.EditVideoReq) error {
	userId := ctx.GetUint("userId")
	oldVideo, _ := FindVideoById(editVideoReq.Vid)
	if cache.GetUploadImage(editVideoReq.Cover) != userId {
		// 查询是否与旧封面图一致
		if oldVideo.Cover != editVideoReq.Cover {
			return errors.New("文件链接无效")
		}
	}

	// 准备更新的字段
	updateData := map[string]any{
		"title": editVideoReq.Title,
		"cover": editVideoReq.Cover,
		"desc":  editVideoReq.Desc,
		"tags":  editVideoReq.Tags,
	}

	// 重新计算视频状态（所有编辑都需要重新审核，防止替换违规内容）
	newStatus := getVideoStatus(editVideoReq.Vid)

	// 特殊处理：如果视频原本已审核通过，编辑后需要重新审核
	// 设置为WAITING_REVIEW(500)而不是根据资源状态计算，避免因为有转码中资源而变成SUBMIT_REVIEW(300)
	if oldVideo.Status == global.AUDIT_APPROVED {
		newStatus = global.WAITING_REVIEW
		utils.InfoLog(fmt.Sprintf("已发布视频被编辑，VideoID=%d，状态从AUDIT_APPROVED(0)改为WAITING_REVIEW(500)，需要重新审核", editVideoReq.Vid), "video")
	}

	updateData["status"] = newStatus

	if err := global.Mysql.Model(&model.Video{}).Where("id = ?", editVideoReq.Vid).Updates(updateData).Error; err != nil {
		utils.ErrorLog("修改视频失败", "video", err.Error())
		return errors.New("修改失败")
	}

	// 如果是已发布视频被编辑，需要从分区列表中移除（因为状态变为待审核）
	if oldVideo.Status == global.AUDIT_APPROVED {
		cache.DelVideoId(oldVideo.PartitionId, oldVideo.ID)
	}

	// 删除视频信息缓存
	cache.DelVideoInfo(editVideoReq.Vid)

	return nil
}

func DeleteVideo(ctx *gin.Context, id uint) error {
	var video model.Video
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Video{}).Where("id = ? and uid = ?", id, userId).First(&video)
	if video.ID == 0 {
		return errors.New("视频不存在")
	}

	// 停止正在进行的转码进程并清理文件
	StopTranscodingAndCleanup(id)

	// 先查询关联的资源记录，用于后续删除相关文件
	var resources []model.Resource
	global.Mysql.Where("vid = ?", id).Find(&resources)

	// 删除关联的m3u8索引文件记录和处理视频文件记录（通过resource_id关联）
	for _, resource := range resources {
		// 查找VideoIndexFile获取DirName
		var indexFile model.VideoIndexFile
		global.Mysql.Where("resource_id = ?", resource.ID).First(&indexFile)

		// 处理 VideoFile 引用计数（全局去重）
		if resource.FileID != 0 {
			// 减少引用计数，如果计数为0则删除VideoFile记录
			decreaseVideoFileRefCount(resource.FileID, userId, resource.ID, indexFile.DirName)
		} else if indexFile.DirName != "" {
			// 兼容旧数据：通过 DirName 查找 VideoFile
			var vf model.VideoFile
			if global.Mysql.Where("dir_name = ?", indexFile.DirName).First(&vf).Error == nil {
				decreaseVideoFileRefCount(vf.ID, userId, resource.ID, indexFile.DirName)
			}
		}

		// 删除VideoIndexFile记录
		if err := global.Mysql.Where("resource_id = ?", resource.ID).Delete(&model.VideoIndexFile{}).Error; err != nil {
			utils.ErrorLog("删除视频关联m3u8索引文件失败", "video", err.Error())
		}
	}

	// 删除关联的资源记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.Resource{}).Error; err != nil {
		utils.ErrorLog("删除视频关联资源失败", "video", err.Error())
	}

	// 删除关联的评论
	if err := global.Mysql.Where("cid = ? and type = 0", id).Delete(&model.Comment{}).Error; err != nil {
		utils.ErrorLog("删除视频关联评论失败", "video", err.Error())
	}

	// 删除关联的弹幕
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.Danmaku{}).Error; err != nil {
		utils.ErrorLog("删除视频关联弹幕失败", "video", err.Error())
	}

	// 删除关联的收藏记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.CollectVideo{}).Error; err != nil {
		utils.ErrorLog("删除视频关联收藏记录失败", "video", err.Error())
	}

	// 删除关联的点赞记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.LikeVideo{}).Error; err != nil {
		utils.ErrorLog("删除视频关联点赞记录失败", "video", err.Error())
	}

	// 删除关联的历史记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.History{}).Error; err != nil {
		utils.ErrorLog("删除视频关联历史记录失败", "video", err.Error())
	}

	// 删除关联的AT消息（cid是内容ID，type=0表示视频）
	if err := global.Mysql.Where("cid = ? and type = 0", id).Delete(&model.AtMessage{}).Error; err != nil {
		utils.ErrorLog("删除视频关联AT消息失败", "video", err.Error())
	}

	if err := global.Mysql.Where("id = ?", id).Delete(&model.Video{}).Error; err != nil {
		utils.ErrorLog("删除视频失败", "video", err.Error())
		return errors.New("删除视频失败")
	}

	// 清理所有相关缓存
	// 1. 删除分区视频列表中的视频ID
	cache.DelVideoId(video.PartitionId, video.ID)

	// 2. 删除热门视频列表中的视频ID
	cache.DelSingleHotVideoId(video.ID)

	// 3. 删除视频信息缓存
	cache.DelVideoInfo(id)

	return nil
}

// 获取视频信息
func GetVideoById(ctx *gin.Context, videoId uint) (vo.VideoResp, error) {
	video := GetVideoInfo(videoId)
	if video.ID == 0 {
		return video, errors.New("视频信息不存在")
	}

	// 增加播放量(一个ip在同一个视频下，每30分钟可重新增加1播放量)
	AddVideoClicks(videoId, ctx.ClientIP())
	video.Clicks += GetVideoClicks(video.ID)

	// 实时查询作者粉丝数（视频缓存中不包含此动态数据）
	var fans int64
	global.Mysql.Model(&model.Relation{}).
		Where("target_uid = ? and (relation = ? or relation = ?)", video.Uid, global.FOLLOWED, global.MUTUAL_FANS).
		Count(&fans)
	video.Author.Fans = fans

	return video, nil
}

// 获取所有的视频列表
func GetAllVideoList(ctx *gin.Context) (videos []vo.AllVideoResp) {
	userId := ctx.GetUint("userId")
	global.Mysql.Model(&model.Video{}).Select("`id`,`title`,`cover`").Where("uid = ?", userId).Scan(&videos)

	return
}

// 获取用户视频
func GetVideoByUser(ctx *gin.Context, userId uint, page, pageSize int) (total int64, videos []vo.UploadVideoResp) {
	global.Mysql.Model(&model.Video{}).
		Where("uid = ? and status = ?", userId, global.AUDIT_APPROVED).Count(&total)
	global.Mysql.Model(&model.Video{}).Select(vo.UPLOAD_VIDEO_FIELD).
		Where("uid = ? and status = ?", userId, global.AUDIT_APPROVED).
		Limit(pageSize).Offset((page - 1) * pageSize).Scan(&videos)

	// 更新播放量数据
	for i := 0; i < len(videos); i++ {
		videos[i].Clicks += GetVideoClicks(videos[i].ID)
	}

	return
}

// 获取视频列表(后台管理)
func GetVideoListManage(videoListReq dto.VideoListReq) (total int64, videos []vo.VideoInfoManageResp) {
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.AUDIT_APPROVED).Count(&total)
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.AUDIT_APPROVED).
		Limit(videoListReq.PageSize).Offset((videoListReq.Page - 1) * videoListReq.PageSize).Scan(&videos)

	// 更新播放量和作者数据
	for i := 0; i < len(videos); i++ {
		videos[i].Clicks += GetVideoClicks(videos[i].ID)
		videos[i].Author = GetUserBaseInfo(videos[i].Uid)
	}

	return
}

// 删除视频(后台管理)
func DeleteVideoManage(ctx *gin.Context, id uint) error {
	// 先查询视频信息，用于删除缓存
	var video model.Video
	global.Mysql.Model(&model.Video{}).Where("id = ?", id).First(&video)

	// 停止正在进行的转码进程并清理文件
	StopTranscodingAndCleanup(id)

	// 先查询关联的资源记录，用于后续删除相关文件
	var resources []model.Resource
	global.Mysql.Where("vid = ?", id).Find(&resources)

	// 删除关联的m3u8索引文件记录和处理视频文件记录（通过resource_id关联）
	for _, resource := range resources {
		// 查找VideoIndexFile获取DirName
		var indexFile model.VideoIndexFile
		global.Mysql.Where("resource_id = ?", resource.ID).First(&indexFile)

		// 处理 VideoFile 引用计数（全局去重）
		if resource.FileID != 0 {
			// 减少引用计数，如果计数为0则删除VideoFile记录
			decreaseVideoFileRefCount(resource.FileID, video.Uid, resource.ID, indexFile.DirName)
		} else if indexFile.DirName != "" {
			// 兼容旧数据：通过 DirName 查找 VideoFile
			var vf model.VideoFile
			if global.Mysql.Where("dir_name = ?", indexFile.DirName).First(&vf).Error == nil {
				decreaseVideoFileRefCount(vf.ID, video.Uid, resource.ID, indexFile.DirName)
			}
		}

		// 删除VideoIndexFile记录
		if err := global.Mysql.Where("resource_id = ?", resource.ID).Delete(&model.VideoIndexFile{}).Error; err != nil {
			utils.ErrorLog("删除视频关联m3u8索引文件失败", "video", err.Error())
		}
	}

	// 删除关联的资源记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.Resource{}).Error; err != nil {
		utils.ErrorLog("删除视频关联资源失败", "video", err.Error())
	}

	// 删除关联的评论
	if err := global.Mysql.Where("cid = ? and type = 0", id).Delete(&model.Comment{}).Error; err != nil {
		utils.ErrorLog("删除视频关联评论失败", "video", err.Error())
	}

	// 删除关联的弹幕
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.Danmaku{}).Error; err != nil {
		utils.ErrorLog("删除视频关联弹幕失败", "video", err.Error())
	}

	// 删除关联的收藏记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.CollectVideo{}).Error; err != nil {
		utils.ErrorLog("删除视频关联收藏记录失败", "video", err.Error())
	}

	// 删除关联的点赞记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.LikeVideo{}).Error; err != nil {
		utils.ErrorLog("删除视频关联点赞记录失败", "video", err.Error())
	}

	// 删除关联的历史记录
	if err := global.Mysql.Where("vid = ?", id).Delete(&model.History{}).Error; err != nil {
		utils.ErrorLog("删除视频关联历史记录失败", "video", err.Error())
	}

	// 删除关联的AT消息（cid是内容ID，type=0表示视频）
	if err := global.Mysql.Where("cid = ? and type = 0", id).Delete(&model.AtMessage{}).Error; err != nil {
		utils.ErrorLog("删除视频关联AT消息失败", "video", err.Error())
	}

	if err := global.Mysql.Where("id = ?", id).Delete(&model.Video{}).Error; err != nil {
		utils.ErrorLog("删除视频失败", "video", err.Error())
		return errors.New("删除视频失败")
	}

	// 清理所有相关缓存
	if video.ID != 0 {
		// 1. 删除分区视频列表中的视频ID
		cache.DelVideoId(video.PartitionId, id)

		// 2. 删除热门视频列表中的视频ID
		cache.DelSingleHotVideoId(id)
	}

	// 3. 删除视频信息缓存
	cache.DelVideoInfo(id)

	return nil
}

// 获取待审核视频列表
func GetReviewList(reviewListReq dto.ReviewListReq) (total int64, videos []vo.ReviewListResp) {
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.WAITING_REVIEW).Count(&total)
	global.Mysql.Model(&model.Video{}).Where("status = ?", global.WAITING_REVIEW).
		Limit(reviewListReq.PageSize).Offset((reviewListReq.Page - 1) * reviewListReq.PageSize).Scan(&videos)

	// 更新播放量和作者数据
	for i := 0; i < len(videos); i++ {
		videos[i].Author = GetUserBaseInfo(videos[i].Uid)
	}

	return
}

// 获取热门视频
func GetHotVideo(ctx *gin.Context, page, pageSize int) []vo.VideoResp {
	ids := cache.GetHotVideoId()
	videoIds := utils.SlicePagingStr(ids, page, pageSize)

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		id := utils.StringToUint(videoIds[i])
		if id == 0 {
			continue
		}
		videos[i] = GetVideoInfo(id)
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(id)
		// 同步弹幕数量
		videos[i].DanmakuCount = GetDanmakuCount(id)
	}

	return videos
}

// 获取分区视频
func GetVideoListByPartition(ctx *gin.Context, size int, partitionId uint) []vo.VideoResp {
	videoIds := cache.GetVideoIdByPartition(partitionId, int64(size))

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		id := utils.StringToUint(videoIds[i])
		if id == 0 {
			continue
		}
		videos[i] = GetVideoInfo(id)
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(id)
		// 同步弹幕数量
		videos[i].DanmakuCount = GetDanmakuCount(id)
	}

	return videos
}

// 获取相关推荐视频
func GetRelatedVideoList(ctx *gin.Context, videoId uint) []vo.VideoResp {
	video := GetVideoInfo(videoId)

	var videoIds []uint
	// 查询同作者的2个视频
	var authorVideoIds []uint
	global.Mysql.Model(&model.Video{}).
		Where("uid = ? and id != ? and `status` = ?", video.Uid, videoId, global.AUDIT_APPROVED).
		Limit(2).Pluck("id", &authorVideoIds)
	videoIds = append(videoIds, authorVideoIds...)

	// 查询同分区的7个视频（防止视频与同作者视频相同或者与当前视频相同）
	// 获取主分区ID用于推荐
	partitionId := video.PartitionId
	if parentId, exists := global.VideoPartitionMap[video.PartitionId]; exists && parentId != 0 {
		partitionId = parentId
	}

	for _, v := range cache.GetVideoIdByPartition(partitionId, 7) {
		id := utils.StringToUint(v)
		if id != videoId && !utils.IsUintInSlice(authorVideoIds, id) {
			videoIds = append(videoIds, id)
		}
	}

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		videos[i] = GetVideoInfo(videoIds[i])
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(videoIds[i])
		// 同步弹幕数量
		videos[i].DanmakuCount = GetDanmakuCount(videoIds[i])
	}

	return videos
}

// 搜索视频
func SearchVideo(ctx *gin.Context, searchVideoReq dto.SearchVideoReq) []vo.VideoResp {
	var videoIds []uint
	if len(searchVideoReq.KeyWords) == 0 {
		global.Mysql.Model(&model.Video{}).Where("`status` = ?", global.AUDIT_APPROVED).
			Limit(searchVideoReq.PageSize).Offset((searchVideoReq.Page-1)*searchVideoReq.PageSize).Pluck("id", &videoIds)
	} else {
		// 直接用mysql模糊查询，之后可能会更换为es
		keywords := "%" + searchVideoReq.KeyWords + "%"
		global.Mysql.Model(&model.Video{}).Where("`status` = ? and (title like ? or tags like ?)", global.AUDIT_APPROVED, keywords, keywords).
			Limit(searchVideoReq.PageSize).Offset((searchVideoReq.Page-1)*searchVideoReq.PageSize).Pluck("id", &videoIds)
	}

	len := len(videoIds)
	videos := make([]vo.VideoResp, len)
	for i := 0; i < len; i++ {
		videos[i] = GetVideoInfo(videoIds[i])
		// 同步播放量
		videos[i].Clicks += GetVideoClicks(videoIds[i])
		// 同步弹幕数量
		videos[i].DanmakuCount = GetDanmakuCount(videoIds[i])
	}

	return videos
}

func CreateVideo(video *model.Video) (uint, error) {
	if err := global.Mysql.Create(video).Error; err != nil {
		utils.ErrorLog("创建视频失败", "video", err.Error())
		return 0, errors.New("创建视频失败")
	}

	return video.ID, nil
}

// 重新转码视频（后台管理专用）
func ReTranscodeVideo(ctx *gin.Context, videoId uint) error {
	var video model.Video
	global.Mysql.Where("id = ?", videoId).First(&video)
	if video.ID == 0 {
		return errors.New("视频不存在")
	}

	// 获取视频的所有资源
	var resources []model.Resource
	global.Mysql.Where("vid = ?", videoId).Find(&resources)
	if len(resources) == 0 {
		return errors.New("该视频没有可转码的资源")
	}

	// 获取原始上传文件信息
	var videoFile model.VideoFile
	if resources[0].FileID != 0 {
		global.Mysql.Where("id = ?", resources[0].FileID).First(&videoFile)
	} else {
		// 兼容旧数据：通过DirName查找
		var indexFile model.VideoIndexFile
		global.Mysql.Where("resource_id = ?", resources[0].ID).First(&indexFile)
		if indexFile.DirName != "" {
			global.Mysql.Where("dir_name = ?", indexFile.DirName).First(&videoFile)
		}
	}

	if videoFile.ID == 0 || videoFile.DirName == "" {
		return errors.New("找不到原始视频文件，无法重新转码")
	}

	// 使用数据库中正确的DirName构建路径
	dirName := videoFile.DirName
	inputPath := "./upload/video/" + dirName + "/upload.mp4"

	// 检查原始文件是否存在
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		utils.ErrorLog("原始视频文件不存在", "transcoding", inputPath)
		return errors.New("原始视频文件不存在，无法重新转码")
	}

	// 停止正在进行的转码进程并清理旧资源
	StopTranscodingAndCleanup(videoId)

	// 删除旧的 VideoIndexFile 记录
	for _, resource := range resources {
		if err := global.Mysql.Where("resource_id = ?", resource.ID).Delete(&model.VideoIndexFile{}).Error; err != nil {
			utils.ErrorLog("删除旧索引文件失败", "transcoding", err.Error())
		}
	}

	// 软删除旧的资源记录
	for _, resource := range resources {
		global.Mysql.Model(&resource).Update("deleted_at", time.Now())
	}

	// 复用原file_id
	newFileID := videoFile.ID

	// 为每个分P创建新的资源记录并启动转码
	for _, resource := range resources {
		// 创建新的资源记录
		newResource := model.Resource{
			Vid:       videoId,
			Uid:       video.Uid,
			Title:     resource.Title,
			CodecName: resource.CodecName,
			Status:    global.VIDEO_PROCESSING,
			Duration:  resource.Duration,
			FileID:    newFileID,
		}
		if err := global.Mysql.Create(&newResource).Error; err != nil {
			utils.ErrorLog("创建新资源记录失败", "transcoding", err.Error())
			continue
		}

		// 准备转码信息（使用原始文件目录作为输出目录）
		transcodingInfo := &dto.TranscodingInfo{
			VideoID:    videoId,
			ResourceID: newResource.ID,
			InputFile:  inputPath,
			OutputDir:  "./upload/video/" + dirName + "/", // 使用原始目录
		}

		// 解析视频信息
		info, err := ProcessVideoInfo(transcodingInfo.InputFile)
		if err != nil {
			utils.ErrorLog("读取视频信息失败", "transcoding", err.Error())
			continue
		}
		transcodingInfo.Width = info.Width
		transcodingInfo.Height = info.Height
		transcodingInfo.Duration = info.Duration
		transcodingInfo.DirName = dirName // 使用原始目录名
		transcodingInfo.CodecName = info.CodecName
		transcodingInfo.FPS = info.FPS
		transcodingInfo.FPS30 = info.FPS30
		transcodingInfo.FPS60 = info.FPS60

		// 启动转码（异步）
		go VideoTransCoding(transcodingInfo)
	}

	// 删除视频信息缓存
	cache.DelVideoInfo(videoId)

	utils.InfoLog(fmt.Sprintf("【重新转码】VideoID=%d, 资源数=%d", videoId, len(resources)), "transcoding")
	return nil
}

// 通过视频ID查询视频
func FindVideoById(id uint) (video model.Video, err error) {
	err = global.Mysql.Where("`id` = ?", id).First(&video).Error
	return
}

// 获取视频信息
func GetVideoInfo(videoId uint) (video vo.VideoResp) {
	video = cache.GetVideoInfo(videoId)
	if video.ID == 0 {
		// 缓存不存在，从数据库加载并写入缓存
		video = VideoWriteCache(videoId)
	}
	// 直接使用缓存中的完整数据，不再重新查询Resources和Author
	// 如果需要最新数据，应该先调用cache.DelVideoInfo删除缓存，再调用本函数

	return
}

// 视频信息写入缓存
func VideoWriteCache(videoId uint) (video vo.VideoResp) {
	global.Mysql.Model(&model.Video{}).Select(vo.VIDEO_FIELD).
		Where("id = ? and status = ?", videoId, global.AUDIT_APPROVED).Scan(&video)
	if video.ID == 0 {
		utils.ErrorLog("视频信息不存在", "video", "")
		return
	}

	// 获取作者信息
	video.Author = GetUserBaseInfo(video.Uid)
	// 获取视频资源
	video.Resources = GetVideoResourceByStatus(videoId, global.AUDIT_APPROVED)
	// 确保Resources不是nil，如果是nil则初始化为空切片，避免JSON序列化为null
	if video.Resources == nil {
		video.Resources = []vo.ResourceResp{}
	}

	// 存到redis
	cache.SetVideoInfo(video)

	return
}

// 获取视频状态
func getVideoStatus(videoId uint) int {
	var processingCount int64 // 转码中的资源数量
	var failedCount int64     // 转码失败的资源数量
	var totalCount int64      // 总资源数量

	global.Mysql.Model(&model.Resource{}).Where("vid = ?", videoId).Count(&totalCount)
	global.Mysql.Model(&model.Resource{}).Where("vid = ? and status = ?", videoId, global.VIDEO_PROCESSING).Count(&processingCount)
	global.Mysql.Model(&model.Resource{}).Where("vid = ? and status = ?", videoId, global.PROCESSING_FAIL).Count(&failedCount)

	// 如果所有资源都失败了，返回处理失败状态
	if failedCount == totalCount && totalCount > 0 {
		return global.PROCESSING_FAIL
	}

	// 如果还有转码中的资源，返回提交审核状态
	if processingCount > 0 {
		return global.SUBMIT_REVIEW
	}

	// 所有资源都完成了（至少有一个成功），返回待审核状态
	return global.WAITING_REVIEW
}
