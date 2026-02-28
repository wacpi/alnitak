package service

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm/clause"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

// truncateString 按字符数截断字符串（支持中文等多字节字符）
func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen])
}

func UploadImg(ctx *gin.Context, file *multipart.FileHeader) (string, error) {
	suffix := path.Ext(file.Filename)
	userId := ctx.GetUint("userId")

	// 参数校验
	if !utils.IsImgType(suffix, global.Config.File.AllowedImgExts) { // 文件后缀
		return "", errors.New("文件类型错误")
	}

	//文件大小限制
	if !utils.FileSize(file.Size, 1, global.Config.File.MaxImgSize) {
		return "", errors.New("文件大小超出限制")
	}

	// 计算文件hash
	fileHash, err := calculateFileHash(file)
	if err != nil {
		return "", errors.New("计算文件hash失败")
	}

	// 查询是否已存在相同hash的图片
	var existingFile model.ImageFile
	global.Mysql.Where("hash = ?", fileHash).First(&existingFile)
	if existingFile.ID != 0 {
		// 已存在相同图片，直接返回已有的URL
		url := generateFileUrl("image/" + existingFile.FileName)
		cache.SetUploadImage(url, userId)
		return url, nil
	}

	// 生成新文件名并保存
	fileName := generateImgFilename(suffix)
	objectKey := "image/" + fileName
	filePath := "./upload/image/" + fileName

	//保存文件
	if err := ctx.SaveUploadedFile(file, filePath); err != nil {
		return "", errors.New("文件上传失败")
	}

	// 记录到数据库
	global.Mysql.Create(&model.ImageFile{
		Uid:      userId,
		FileName: fileName,
		Hash:     fileHash,
	})

	url := generateFileUrl(objectKey)
	if global.Config.Storage.OssType != "local" {
		// 上传到OSS
		global.Storage.PutObjectFromFile(objectKey, filePath)
	}

	// 缓存url
	cache.SetUploadImage(url, userId)

	return url, nil
}

// 计算上传文件的MD5 hash
func calculateFileHash(file *multipart.FileHeader) (string, error) {
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, src); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// UploadVideoCreate 创建新视频稿件（全局去重版本）
func UploadVideoCreate(ctx *gin.Context, videoFileReq dto.VideoFileReq) (vo.ResourceResp, error) {
	userId := ctx.GetUint("userId")

	var fileInfo model.VideoFile

	// 支持 FileID 和 Hash 两种方式查询
	if videoFileReq.FileID > 0 {
		if err := global.Mysql.Unscoped().First(&fileInfo, videoFileReq.FileID).Error; err != nil {
			utils.ErrorLog("视频文件信息不存在", "upload", fmt.Sprintf("fileID=%d", videoFileReq.FileID))
			return vo.ResourceResp{}, errors.New("视频文件不存在")
		}
	} else {
		// 【全局去重】按 hash + size 查询，包含软删除的记录
		if err := global.Mysql.Unscoped().Where("hash = ? AND size = ?", videoFileReq.Hash, videoFileReq.Size).First(&fileInfo).Error; err != nil {
			utils.ErrorLog("视频文件信息不存在", "upload", fmt.Sprintf("hash=%s, size=%d", videoFileReq.Hash, videoFileReq.Size))
			return vo.ResourceResp{}, errors.New("视频文件不存在")
		}
	}

	// 先创建视频记录
	suffix := utils.GetFileSuffix(fileInfo.OriginalName)
	uploadVideoPath := "./upload/video/" + fileInfo.DirName + "/upload" + suffix
	vid, err := initVideo(userId, uploadVideoPath, fileInfo.OriginalName)
	if err != nil || vid == 0 {
		utils.ErrorLog("创建视频失败", "upload", fmt.Sprintf("uid=%d, dirName=%s, originalName=%s, err=%v", userId, fileInfo.DirName, fileInfo.OriginalName, err))
		return vo.ResourceResp{}, errors.New("创建失败")
	}

	// 创建用户引用关系
	createFileRef(userId, fileInfo.ID, 0)

	resource, err := CompleteUploadVideo(vid, userId, fileInfo.ID, fileInfo.DirName, fileInfo.OriginalName, fileInfo.Status == model.FileStatusReady)
	if err != nil {
		return vo.ResourceResp{}, err
	}

	// 更新引用关系中的 ResourceID
	updateFileRefResourceID(userId, fileInfo.ID, resource.ID)

	return resource, nil
}

// UploadVideoAdd 向已有视频添加分P（全局去重版本）
func UploadVideoAdd(ctx *gin.Context, vid uint, videoFileReq dto.VideoFileReq) (vo.ResourceResp, error) {
	userId := ctx.GetUint("userId")

	// 【全局去重】按 hash + size 查询，包含软删除的记录
	var fileInfo model.VideoFile
	if err := global.Mysql.Unscoped().Where("hash = ? AND size = ?", videoFileReq.Hash, videoFileReq.Size).First(&fileInfo).Error; err != nil {
		utils.ErrorLog("视频文件信息不存在", "upload", fmt.Sprintf("hash=%s, size=%d", videoFileReq.Hash, videoFileReq.Size))
		return vo.ResourceResp{}, errors.New("视频文件不存在")
	}

	// 如果是软删除状态，恢复它
	if fileInfo.DeletedAt.Valid {
		if err := global.Mysql.Unscoped().Model(&fileInfo).Update("deleted_at", nil).Error; err != nil {
			utils.ErrorLog("恢复软删除记录失败", "upload", err.Error())
		}
	}

	// 创建用户引用关系
	createFileRef(userId, fileInfo.ID, 0)

	resource, err := CompleteUploadVideo(vid, userId, fileInfo.ID, fileInfo.DirName, fileInfo.OriginalName, fileInfo.Status == model.FileStatusReady)
	if err != nil {
		return vo.ResourceResp{}, err
	}

	// 更新引用关系中的 ResourceID
	updateFileRefResourceID(userId, fileInfo.ID, resource.ID)

	return resource, nil
}

// UploadVideoCheck 检查视频上传进度（全局去重版本）
// 返回值: checks=已上传的分片索引, fileID=视频文件ID
// 返回 chunks=[-1] 表示秒传成功，客户端可以直接调用创建接口
func UploadVideoCheck(ctx *gin.Context, videoFileReq dto.VideoFileReq) (dto.VideoCheckResp, error) {
	resp := dto.VideoCheckResp{
		Chunks: []int{},
		FileID: 0,
	}

	// 【全局去重】按 hash + size 查询，包含软删除的记录
	var fileInfo model.VideoFile
	result := global.Mysql.Unscoped().Where("hash = ? AND size = ?", videoFileReq.Hash, videoFileReq.Size).First(&fileInfo)

	// 文件不存在，返回空列表（需要从头上传）
	if result.Error != nil {
		utils.InfoLog(fmt.Sprintf("【秒传检测】hash=%s, size=%d, VideoFile不存在，需要上传", videoFileReq.Hash, videoFileReq.Size), "upload")
		return resp, nil
	}

	resp.FileID = fileInfo.ID

	// 如果是软删除状态，恢复它
	if fileInfo.DeletedAt.Valid {
		if err := global.Mysql.Unscoped().Model(&fileInfo).Update("deleted_at", nil).Error; err != nil {
			utils.ErrorLog("恢复软删除记录失败", "upload", err.Error())
		} else {
			utils.InfoLog(fmt.Sprintf("【秒传检测】hash=%s, 恢复软删除记录 fileID=%d", videoFileReq.Hash, fileInfo.ID), "upload")
		}
	}

	utils.InfoLog(fmt.Sprintf("【秒传检测】hash=%s, size=%d, fileID=%d, Status=%d, RefCount=%d, DirName=%s",
		videoFileReq.Hash, videoFileReq.Size, fileInfo.ID, fileInfo.Status, fileInfo.RefCount, fileInfo.DirName), "upload")

	// 【秒传判断】文件状态已就绪
	if fileInfo.Status == model.FileStatusReady {
		// 检查本地文件是否存在
		suffix := utils.GetFileSuffix(fileInfo.OriginalName)
		uploadVideoPath := "./upload/video/" + fileInfo.DirName + "/upload" + suffix
		if !utils.IsFileExists(uploadVideoPath) {
			// 本地文件不存在，需要重新上传
			utils.InfoLog(fmt.Sprintf("【秒传检测】hash=%s, size=%d, 文件状态Ready但本地文件不存在，需要重新上传", videoFileReq.Hash, videoFileReq.Size), "upload")
			// 更新文件状态为待上传
			global.Mysql.Model(&fileInfo).Update("status", model.FileStatusMerged)
		} else {
			// 检查是否存在关联的 Resource 记录（确保有转码信息可复用）
			// 优先查 file_id 关联，如果没有则通过 VideoIndexFile 的 DirName 关联查询
			var resourceCount int64
			global.Mysql.Model(&model.Resource{}).Where("file_id = ?", fileInfo.ID).Count(&resourceCount)
			utils.InfoLog(fmt.Sprintf("【秒传检测】通过FileID查询: fileID=%d 关联Resource数量=%d", fileInfo.ID, resourceCount), "upload")

			if resourceCount > 0 {
				// 有已关联资源，可以秒传
				utils.InfoLog(fmt.Sprintf("【秒传成功】hash=%s, size=%d, fileID=%d, 返回[-1]", videoFileReq.Hash, videoFileReq.Size, fileInfo.ID), "upload")
				resp.Chunks = []int{-1}
				return resp, nil
			}

			// 兼容旧数据：Resource.FileID 为 0，通过 VideoIndexFile.DirName 查找对应的资源
			var videoIndex model.VideoIndexFile
			if err := global.Mysql.Where("dir_name = ?", fileInfo.DirName).First(&videoIndex).Error; err == nil {
				utils.InfoLog(fmt.Sprintf("【秒传检测】通过DirName查询: DirName=%s, ResourceID=%d", fileInfo.DirName, videoIndex.ResourceID), "upload")
				if videoIndex.ResourceID > 0 {
					// 找到了通过 DirName 关联的资源，可以秒传
					// 同时修复 Resource.FileID 以便后续直接使用
					global.Mysql.Model(&model.Resource{}).Where("id = ? AND file_id = 0", videoIndex.ResourceID).Update("file_id", fileInfo.ID)
					utils.InfoLog(fmt.Sprintf("【秒传成功】hash=%s, size=%d, fileID=%d (通过DirName关联), 返回[-1]", videoFileReq.Hash, videoFileReq.Size, fileInfo.ID), "upload")
					resp.Chunks = []int{-1}
					return resp, nil
				}
			}

			// 文件状态是Ready但没有Resource，可能是异常情况，继续检查分片
			utils.InfoLog(fmt.Sprintf("【秒传异常】hash=%s, size=%d, fileID=%d, 状态Ready但无可用Resource", videoFileReq.Hash, videoFileReq.Size, fileInfo.ID), "upload")
		}
	} else {
		utils.InfoLog(fmt.Sprintf("【秒传检测】hash=%s, Status=%d 不是Ready(3)，继续上传流程", videoFileReq.Hash, fileInfo.Status), "upload")
	}

	// 文件正在上传中，返回已上传的分片
	resp.Chunks = []int{}
	fileDir := "./upload/video/" + fileInfo.DirName
	for i := 0; i < fileInfo.ChunksCount; i++ {
		if utils.IsFileExists(fmt.Sprintf("%s/chunks/%d.part", fileDir, i)) {
			resp.Chunks = append(resp.Chunks, i)
		}
	}

	return resp, nil
}

// UploadVideoChunk 上传视频分片
func UploadVideoChunk(ctx *gin.Context, file *multipart.FileHeader) error {
	userId := ctx.GetUint("userId")

	fileHash := strings.TrimSpace(ctx.PostForm("hash"))
	fileName := strings.TrimSpace(ctx.PostForm("name"))
	fileSize := strings.TrimSpace(ctx.PostForm("size"))

	chunkIndex, err := strconv.Atoi(ctx.PostForm("chunkIndex"))
	if err != nil {
		return errors.New("非法分片索引")
	}

	totalChunks, err := strconv.Atoi(ctx.PostForm("totalChunks"))
	if err != nil || totalChunks <= 0 {
		return errors.New("非法分片总数")
	}

	size, err := strconv.ParseInt(fileSize, 10, 64)
	if err != nil || size <= 0 {
		return errors.New("非法文件大小")
	}

	// 1️⃣ 校验文件类型
	suffix := path.Ext(fileName)
	if !utils.IsVideoType(suffix, global.Config.File.AllowedVideoExts) {
		return errors.New("不支持的视频格式")
	}

	// 2️⃣ 查询或创建文件记录（hash + size 全局唯一）
	var videoFile model.VideoFile
	err = global.Mysql.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("hash = ? AND size = ?", fileHash, size).First(&videoFile)

		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			videoFile = model.VideoFile{
				Hash:         fileHash,
				Size:         size,
				DirName:      generateVideoFilename(),
				OriginalName: truncateString(fileName, 255),
				ChunksCount:  totalChunks,
				Status:       model.FileStatusUploading,
				UploaderUid:  userId,
			}
			if err := tx.Create(&videoFile).Error; err != nil {
				return err
			}
		} else if result.Error != nil {
			return result.Error
		}

		// 3️⃣ 校验 totalChunks 不允许被篡改
		if videoFile.ChunksCount != totalChunks {
			return errors.New("分片数量不匹配")
		}
		return nil
	})
	if err != nil {
		utils.ErrorLog("查询或创建视频记录失败", "upload", err.Error())
		return errors.New("文件上传失败")
	}

	// 4️⃣ 秒传判断
	if videoFile.Status == model.FileStatusReady {
		utils.InfoLog("秒传成功："+fileHash, "upload")
		return nil
	}

	// 5️⃣ 校验 chunkIndex 合法性
	if chunkIndex < 0 || chunkIndex >= totalChunks {
		return errors.New("非法分片索引")
	}

	fileDir := filepath.Join("./upload/video", videoFile.DirName)
	chunkDir := filepath.Join(fileDir, "chunks")

	// 6️⃣ 创建目录
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		return errors.New("服务器存储异常")
	}

	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d.part", chunkIndex))

	// 7️⃣ 幂等检查
	if utils.IsFileExists(chunkPath) {
		return nil
	}

	// 8️⃣ 保存分片
	if err := ctx.SaveUploadedFile(file, chunkPath); err != nil {
		return errors.New("分片保存失败")
	}

	return nil
}

// UploadVideoMerge 合并视频分片（全局去重版本）
func UploadVideoMerge(ctx *gin.Context, videoFileReq dto.VideoFileReq) error {
	var fileInfo model.VideoFile

	// 1️⃣ 查询文件：优先按 fileID，fallback 到 hash+size
	if videoFileReq.FileID > 0 {
		if err := global.Mysql.First(&fileInfo, videoFileReq.FileID).Error; err != nil {
			return errors.New("视频文件不存在")
		}
	} else if videoFileReq.Hash != "" && videoFileReq.Size > 0 {
		if err := global.Mysql.Where("hash = ? AND size = ?", videoFileReq.Hash, videoFileReq.Size).First(&fileInfo).Error; err != nil {
			return errors.New("视频文件不存在")
		}
	} else {
		return errors.New("缺少文件标识参数")
	}

	// 2️⃣ 原子切换状态：Uploading → Merging
	result := global.Mysql.Model(&fileInfo).
		Where("status = ?", model.FileStatusUploading).
		Update("status", model.FileStatusMerging)
	if result.RowsAffected == 0 {
		return nil
	}

	fileDir := filepath.Join("./upload/video", fileInfo.DirName)
	chunkDir := filepath.Join(fileDir, "chunks")
	suffix := utils.GetFileSuffix(fileInfo.OriginalName)
	outputFile := filepath.Join(fileDir, "upload"+suffix)

	// 3️⃣ 校验分片数量
	files, err := os.ReadDir(chunkDir)
	if err != nil {
		return errors.New("分片目录不存在")
	}
	if len(files) != fileInfo.ChunksCount {
		global.Mysql.Model(&fileInfo).
			Update("status", model.FileStatusUploading)
		return errors.New("分片数量不完整")
	}

	// 4️⃣ 执行流式合并
	if err := utils.MergeChunks(chunkDir, fileInfo.ChunksCount, outputFile); err != nil {
		global.Mysql.Model(&fileInfo).
			Update("status", model.FileStatusUploading)
		return errors.New("合并分片失败")
	}

	// 5️⃣ 二次校验 hash
	calculatedHash, err := utils.CalculateFileHash(outputFile)
	if err != nil || calculatedHash != fileInfo.Hash {
		os.Remove(outputFile)
		global.Mysql.Model(&fileInfo).
			Update("status", model.FileStatusUploading)
		return errors.New("文件校验失败")
	}

	// 6️⃣ 更新最终状态
	global.Mysql.Model(&fileInfo).
		Update("status", model.FileStatusMerged)

	// 7️⃣ 删除分片目录
	_ = os.RemoveAll(chunkDir)

	return nil
}

// createFileRef 创建用户-文件引用关系
func createFileRef(uid, fileID, resourceID uint) {
	ref := model.VideoFileRef{
		Uid:        uid,
		FileID:     fileID,
		ResourceID: resourceID,
	}
	global.Mysql.Create(&ref)

	// 增加引用计数
	global.Mysql.Model(&model.VideoFile{}).Where("id = ?", fileID).
		UpdateColumn("ref_count", gorm.Expr("ref_count + 1"))
}

// updateFileRefResourceID 更新引用关系中的 ResourceID
func updateFileRefResourceID(uid, fileID, resourceID uint) {
	global.Mysql.Model(&model.VideoFileRef{}).
		Where("uid = ? AND file_id = ? AND resource_id = 0", uid, fileID).
		Update("resource_id", resourceID)
}

// decreaseVideoFileRefCount 减少视频文件引用计数，如果计数为0则删除文件记录
// 用于删除稿件时正确处理全局去重的文件引用
func decreaseVideoFileRefCount(fileID, uid, resourceID uint, dirName string) {
	if fileID == 0 {
		return
	}

	// 删除用户的引用记录
	if err := global.Mysql.Where("file_id = ? AND uid = ? AND resource_id = ?", fileID, uid, resourceID).
		Delete(&model.VideoFileRef{}).Error; err != nil {
		utils.ErrorLog(fmt.Sprintf("删除VideoFileRef失败, fileID=%d, uid=%d, resourceID=%d", fileID, uid, resourceID), "upload", err.Error())
	}

	// 减少引用计数
	result := global.Mysql.Model(&model.VideoFile{}).Where("id = ? AND ref_count > 0", fileID).
		UpdateColumn("ref_count", gorm.Expr("ref_count - 1"))
	if result.Error != nil {
		utils.ErrorLog(fmt.Sprintf("减少VideoFile引用计数失败, fileID=%d", fileID), "upload", result.Error.Error())
		return
	}

	// 检查引用计数是否为0，如果是则删除VideoFile记录
	var vf model.VideoFile
	if err := global.Mysql.Where("id = ?", fileID).First(&vf).Error; err != nil {
		return
	}

	if vf.RefCount <= 0 {
		// 没有任何引用了，删除VideoFile记录
		if err := global.Mysql.Where("id = ?", fileID).Delete(&model.VideoFile{}).Error; err != nil {
			utils.ErrorLog(fmt.Sprintf("删除VideoFile失败, fileID=%d", fileID), "upload", err.Error())
		} else {
			utils.InfoLog(fmt.Sprintf("【删除VideoFile】fileID=%d, dirName=%s, 引用计数为0", fileID, dirName), "upload")
		}
	} else {
		utils.InfoLog(fmt.Sprintf("【减少引用计数】fileID=%d, 剩余引用=%d", fileID, vf.RefCount), "upload")
	}
}

// CompleteUploadVideo 完成视频上传（支持全局去重）
// fileID: 关联的视频文件ID
// skipTranscode: 如果文件已转码完成，跳过转码直接使用
func CompleteUploadVideo(vid, userId, fileID uint, videoName, title string, skipTranscode bool) (vo.ResourceResp, error) {
	suffix := utils.GetFileSuffix(title)
	uploadVideoPath := "./upload/video/" + videoName + "/upload" + suffix

	// 去掉后缀名并截断过长标题
	titleWithoutExt := title[:len(title)-len(path.Ext(title))]
	titleWithoutExt = truncateString(titleWithoutExt, 255)

	// 如果文件已就绪（秒传），直接复用已有转码结果
	if skipTranscode {
		// 查询已有的资源获取视频信息（duration, codecName等）
		var existingResource model.Resource
		if err := global.Mysql.Where("file_id = ?", fileID).First(&existingResource).Error; err == nil {
			// 【重要】秒传的资源也需要审核，状态设为 WAITING_REVIEW
			// 审核员决定是否通过，不能因为别人上传过就自动通过
			resource := model.Resource{
				Vid:       vid,
				Uid:       userId,
				Title:     titleWithoutExt,
				CodecName: existingResource.CodecName,
				Status:    global.WAITING_REVIEW, // 等待审核，不直接通过
				Duration:  existingResource.Duration,
				FileID:    fileID,
			}
			if err := global.Mysql.Create(&resource).Error; err != nil {
				return vo.ResourceResp{}, errors.New("保存视频失败")
			}

			utils.InfoLog(fmt.Sprintf("【秒传成功】vid=%d, fileID=%d, uid=%d, 等待审核", vid, fileID, userId), "upload")
			return vo.ResourceToResourceResp(resource), nil
		}
		// 如果没找到已有资源，说明秒传判断有误，走正常转码流程
		utils.InfoLog(fmt.Sprintf("【秒传异常】fileID=%d 无已有资源记录，转为正常转码", fileID), "upload")
	}

	// 正常流程：读取视频信息并启动转码
	transcodingInfo, err := ProcessVideoInfo(uploadVideoPath)
	if err != nil {
		return vo.ResourceResp{}, errors.New("读取视频信息失败")
	}

	// 存入数据库
	resource := model.Resource{
		Vid:       vid,
		Uid:       userId,
		Title:     titleWithoutExt,
		CodecName: transcodingInfo.CodecName,
		Status:    global.VIDEO_PROCESSING,
		Duration:  transcodingInfo.Duration,
		FileID:    fileID,
	}
	if err := global.Mysql.Create(&resource).Error; err != nil {
		return vo.ResourceResp{}, errors.New("保存视频失败")
	}

	// 启动转码服务
	transcodingInfo.VideoID = vid
	transcodingInfo.DirName = videoName
	transcodingInfo.ResourceID = resource.ID
	transcodingInfo.OutputDir = "./upload/video/" + videoName + "/"
	transcodingInfo.InputFile = transcodingInfo.OutputDir + "upload" + suffix
	transcodingInfo.Suffix = suffix
	go VideoTransCoding(transcodingInfo)

	return vo.ResourceToResourceResp(resource), nil
}

// 生成文件url（存库用，必须短且不过期）
// 公开OSS: 直接拼接公开URL
// 私有OSS/local: 存 /api/ 路径，访问时后端302到签名URL
func generateFileUrl(objectKey string) string {
	if global.Config.Storage.OssType != "local" && !global.Config.Storage.Private {
		return global.GetOssUrl(objectKey)
	}

	return "/api/" + objectKey
}

// 初始化视频
func initVideo(userId uint, videoPath, title string) (uint, error) {
	// 生成封面
	coverName := generateImgFilename(".jpg")
	objectKey := "image/" + coverName
	filePath := "./upload/image/" + coverName

	if err := GenerateCover(videoPath, filePath); err != nil {
		utils.ErrorLog("生成封面失败", "upload", fmt.Sprintf("videoPath=%s, err=%v", videoPath, err))
	}
	if global.Config.Storage.OssType != "local" {
		// 上传到OSS
		global.Storage.PutObjectFromFile(objectKey, filePath)
	}
	// 去掉后缀名并截断过长标题
	titleWithoutExt := title[:len(title)-len(path.Ext(title))]
	titleWithoutExt = truncateString(titleWithoutExt, 255)

	coverUrl := generateFileUrl(objectKey)

	utils.InfoLog(fmt.Sprintf("initVideo: uid=%d, title=%s, cover=%s", userId, titleWithoutExt, coverUrl), "upload")

	videoId, err := CreateVideo(&model.Video{
		Uid:       userId,
		Cover:     coverUrl,
		Title:     titleWithoutExt,
		Copyright: true,
		Status:    global.CREATED_VIDEO,
	})
	if err != nil {
		utils.ErrorLog("CreateVideo失败", "upload", fmt.Sprintf("uid=%d, title=%s, err=%v", userId, titleWithoutExt, err))
		return 0, err
	}

	return videoId, nil
}

// 随机生成图片文件名
func generateImgFilename(suffix string) string {
	id := global.SnowflakeNode.Generate()
	return id.String() + suffix
}

// 随机视频文件名
func generateVideoFilename() string {
	id := global.SnowflakeNode.Generate()
	return id.String()
}
