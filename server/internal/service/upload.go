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
	"strconv"

	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/cache"
	"interastral-peace.com/alnitak/internal/domain/dto"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/domain/vo"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/utils"
)

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

func UploadVideoCreate(ctx *gin.Context, videoFileReq dto.VideoFileReq) (vo.ResourceResp, error) {
	userId := ctx.GetUint("userId")
	var fileInfo model.VideoFile
	if err := global.Mysql.Where("hash = ? and uid = ?", videoFileReq.Hash, userId).Find(&fileInfo).Error; err != nil {
		utils.ErrorLog("视频文件信息不存在", "upload", videoFileReq.Hash)
		return vo.ResourceResp{}, errors.New("视频文件不存在")
	}

	// 先创建视频记录
	uploadVideoPath := "./upload/video/" + fileInfo.DirName + "/upload.mp4"
	vid, _ := initVideo(userId, uploadVideoPath, fileInfo.OriginalName)
	if vid == 0 {
		return vo.ResourceResp{}, errors.New("创建失败")
	}

	resource, err := CompleteUploadVideo(vid, userId, fileInfo.DirName, fileInfo.OriginalName)
	if err != nil {
		return vo.ResourceResp{}, err
	}

	return resource, nil
}

func UploadVideoAdd(ctx *gin.Context, vid uint, videoFileReq dto.VideoFileReq) (vo.ResourceResp, error) {
	userId := ctx.GetUint("userId")
	var fileInfo model.VideoFile
	if err := global.Mysql.Where("hash = ? and uid = ?", videoFileReq.Hash, userId).Find(&fileInfo).Error; err != nil {
		utils.ErrorLog("视频文件信息不存在", "upload", videoFileReq.Hash)
		return vo.ResourceResp{}, errors.New("视频文件不存在")
	}

	resource, err := CompleteUploadVideo(vid, userId, fileInfo.DirName, fileInfo.OriginalName)
	if err != nil {
		return vo.ResourceResp{}, err
	}

	return resource, nil
}

func UploadVideoCheck(ctx *gin.Context, videoFileReq dto.VideoFileReq) ([]int, error) {
	userId := ctx.GetUint("userId")
	var fileInfo model.VideoFile
	if err := global.Mysql.Where("hash = ? and uid = ?", videoFileReq.Hash, userId).Find(&fileInfo).Error; err != nil {
		utils.ErrorLog("视频文件信息不存在", "upload", videoFileReq.Hash)
		return nil, errors.New("视频文件不存在")
	}

	var checks []int
	fileDir := "./upload/video/" + fileInfo.DirName
	for i := 0; i < fileInfo.ChunksCount; i++ {
		if utils.IsFileExists(fmt.Sprintf("%s/chunks/%d.part", fileDir, i)) {
			checks = append(checks, i)
		}
	}

	return checks, nil
}

func UploadVideoChunk(ctx *gin.Context, file *multipart.FileHeader) error {
	userId := ctx.GetUint("userId")

	// 获取分片信息
	fileHash := ctx.PostForm("hash")
	fileName := ctx.PostForm("name")
	chunkIndex, _ := strconv.Atoi(ctx.PostForm("chunkIndex"))
	totalChunks, _ := strconv.Atoi(ctx.PostForm("totalChunks"))

	suffix := path.Ext(fileName)
	if !utils.IsVideoType(suffix, global.Config.File.AllowedVideoExts) { // 文件后缀
		return errors.New("视频上传失败")
	}

	if !utils.FileSize(file.Size, int64(totalChunks), global.Config.File.MaxVideoSize) {
		return errors.New("文件大小超出限制")
	}

	// 【并发安全】使用 FirstOrCreate 确保同一 hash+uid 只创建一条记录
	// 避免并发上传时多个请求同时查询为空，各自创建不同目录的问题
	var videoFileInfo model.VideoFile
	result := global.Mysql.Where("uid = ? AND hash = ?", userId, fileHash).
		Attrs(model.VideoFile{
			Uid:          userId,
			Hash:         fileHash,
			DirName:      generateVideoFilename(),
			OriginalName: fileName,
			ChunksCount:  totalChunks,
		}).
		FirstOrCreate(&videoFileInfo)

	if result.Error != nil {
		utils.ErrorLog("创建视频文件记录失败", "upload", result.Error.Error())
		return errors.New("文件上传失败")
	}

	fileDir := "./upload/video/" + videoFileInfo.DirName
	chunksPath := fileDir + "/chunks/" + strconv.Itoa(chunkIndex) + ".part"
	if err := ctx.SaveUploadedFile(file, chunksPath); err != nil {
		return errors.New("文件上传失败")
	}

	return nil
}

func UploadVideoMerge(ctx *gin.Context, videoFileReq dto.VideoFileReq) error {
	userId := ctx.GetUint("userId")
	var fileInfo model.VideoFile
	if err := global.Mysql.Where("hash = ? and uid = ?", videoFileReq.Hash, userId).Find(&fileInfo).Error; err != nil {
		utils.ErrorLog("视频文件信息不存在", "upload", videoFileReq.Hash)
		return errors.New("视频文件不存在")
	}

	fileDir := "./upload/video/" + fileInfo.DirName
	if err := mergeChunks(fileDir, fileInfo.ChunksCount); err != nil {
		utils.ErrorLog("合并分片失败", "upload", err.Error())
		return errors.New("合并分片失败")
	}

	if err := os.RemoveAll(fileDir + "/chunks/"); err != nil {
		utils.ErrorLog("删除临时文件夹失败", "upload", err.Error())
	}

	return nil
}

func mergeChunks(fileDir string, totalChunks int) error {
	outputPath := fileDir + "/upload.mp4"
	outFile, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	for i := 0; i < totalChunks; i++ {
		chunkPath := fmt.Sprintf("%s/chunks/%d.part", fileDir, i)
		chunk, err := os.ReadFile(chunkPath)
		if err != nil {
			return err
		}
		if _, err := outFile.Write(chunk); err != nil {
			return err
		}
	}

	return nil
}

func CompleteUploadVideo(vid, userId uint, videoName, title string) (vo.ResourceResp, error) {
	uploadVideoPath := "./upload/video/" + videoName + "/upload.mp4"
	transcodingInfo, err := ProcessVideoInfo(uploadVideoPath)
	if err != nil {
		return vo.ResourceResp{}, errors.New("读取视频信息失败")
	}

	// 去掉后缀名
	titleWithoutExt := title[:len(title)-len(path.Ext(title))]

	// 存入数据库
	resource := model.Resource{
		Vid:       vid,
		Uid:       userId,
		Title:     titleWithoutExt,
		CodecName: transcodingInfo.CodecName,
		Status:    global.VIDEO_PROCESSING,
		Duration:  transcodingInfo.Duration,
	}
	if err := global.Mysql.Create(&resource).Error; err != nil {
		return vo.ResourceResp{}, errors.New("保存视频失败")
	}

	// 启动转码服务
	transcodingInfo.VideoID = vid
	transcodingInfo.DirName = videoName
	transcodingInfo.ResourceID = resource.ID
	transcodingInfo.OutputDir = "./upload/video/" + videoName + "/"
	transcodingInfo.InputFile = transcodingInfo.OutputDir + "upload.mp4"
	go VideoTransCoding(transcodingInfo)

	return vo.ResourceToResourceResp(resource), nil
}

// 生成文件url
func generateFileUrl(objectKey string) string {
	if global.Config.Storage.OssType != "local" {
		global.Storage.GetObjectUrl(objectKey)
	}

	return "/api/" + objectKey
}

// 初始化视频
func initVideo(userId uint, videoPath, title string) (uint, error) {
	// 生成封面
	coverName := generateImgFilename(".jpg")
	objectKey := "image/" + coverName
	filePath := "./upload/image/" + coverName

	GenerateCover(videoPath, filePath)
	if global.Config.Storage.OssType != "local" {
		// 上传到OSS
		global.Storage.PutObjectFromFile(objectKey, filePath)
	}
	// 去掉后缀名
	titleWithoutExt := title[:len(title)-len(path.Ext(title))]

	videoId, err := CreateVideo(&model.Video{
		Uid:       userId,
		Cover:     generateFileUrl(objectKey),
		Title:     titleWithoutExt,
		Copyright: true,
		Status:    global.CREATED_VIDEO,
	})
	if err != nil {
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
