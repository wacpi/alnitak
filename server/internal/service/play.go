package service

import (
	"errors"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"interastral-peace.com/alnitak/internal/domain/model"
	"interastral-peace.com/alnitak/internal/global"
	"interastral-peace.com/alnitak/pkg/playtoken"
)

const playGrantTTL = 8 * time.Minute
const streamSliceTTL = 2 * time.Hour

// IssuePlayGrantForResource 为分 P shortId 签发播放授权（公开稿件可无登录 uid=0）。
func IssuePlayGrantForResource(ctx *gin.Context, resourceShortID string) (token string, expiresUnix int64, err error) {
	var res model.Resource
	if err := global.Mysql.Where("short_id = ?", resourceShortID).First(&res).Error; err != nil || res.ID == 0 {
		return "", 0, errors.New("资源不存在")
	}
	var v model.Video
	if err := global.Mysql.Where("id = ?", res.Vid).First(&v).Error; err != nil || v.ID == 0 {
		return "", 0, errors.New("视频不存在")
	}
	if v.Status != global.AUDIT_APPROVED || res.Status != global.AUDIT_APPROVED {
		return "", 0, errors.New("内容不可播放")
	}
	uid := ctx.GetUint("userId")
	tok, err := playtoken.IssuePlayGrant(uid, v.ID, resourceShortID, playGrantTTL)
	if err != nil {
		return "", 0, err
	}
	return tok, time.Now().Add(playGrantTTL).Unix(), nil
}

// GetPlayURLs 校验 grant 后返回默认清晰度下音视频分离的直链（带 st）。
func GetPlayURLs(ctx *gin.Context, resourceShortID, grantToken, quality string) (videoURL, audioURL string, expires int64, err error) {
	if err := checkPlayAccess(ctx); err != nil {
		return "", "", 0, err
	}
	claims, err := playtoken.ParsePlayGrant(grantToken)
	if err != nil {
		return "", "", 0, errors.New("播放凭证无效")
	}
	if claims.ResourceShortID != resourceShortID {
		return "", "", 0, errors.New("播放凭证与资源不匹配")
	}

	var res model.Resource
	if err := global.Mysql.Where("short_id = ?", resourceShortID).First(&res).Error; err != nil || res.ID == 0 {
		return "", "", 0, errors.New("资源不存在")
	}
	if res.Vid != claims.VideoID {
		return "", "", 0, errors.New("播放凭证已失效")
	}
	var v model.Video
	if err := global.Mysql.Where("id = ?", res.Vid).First(&v).Error; err != nil || v.ID == 0 {
		return "", "", 0, errors.New("视频不存在")
	}
	if v.Status != global.AUDIT_APPROVED || res.Status != global.AUDIT_APPROVED {
		return "", "", 0, errors.New("内容不可播放")
	}

	var files []model.VideoIndexFile
	if err := global.Mysql.Where("resource_id = ?", res.ID).Find(&files).Error; err != nil || len(files) == 0 {
		return "", "", 0, errors.New("播放索引未就绪")
	}
	var candidates []model.VideoIndexFile
	for _, f := range files {
		if f.IsSegmentBase() {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		return "", "", 0, errors.New("当前资源不支持该播放方式")
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].VideoBandwidth > candidates[j].VideoBandwidth
	})
	var file model.VideoIndexFile
	if quality != "" {
		found := false
		for _, f := range candidates {
			if f.Quality == quality {
				file = f
				found = true
				break
			}
		}
		if !found {
			return "", "", 0, errors.New("清晰度不存在")
		}
	} else {
		file = candidates[0]
	}

	exp := time.Now().Add(streamSliceTTL).Unix()
	vTok, err := playtoken.IssueStreamToken(file.DirName, file.VideoFile, streamSliceTTL)
	if err != nil {
		return "", "", 0, err
	}
	aTok, err := playtoken.IssueStreamToken(file.DirName, file.AudioFile, streamSliceTTL)
	if err != nil {
		return "", "", 0, err
	}
	base := publicAPIBase(ctx)
	videoURL = base + "/api/v1/video/stream/" + url.PathEscape(file.VideoFile) + "?st=" + url.QueryEscape(vTok)
	audioURL = base + "/api/v1/video/stream/" + url.PathEscape(file.AudioFile) + "?st=" + url.QueryEscape(aTok)
	return videoURL, audioURL, exp, nil
}

func publicAPIBase(ctx *gin.Context) string {
	if d := strings.TrimSpace(global.Config.Storage.Domain); d != "" {
		if strings.HasPrefix(d, "http://") || strings.HasPrefix(d, "https://") {
			return strings.TrimRight(d, "/")
		}
		scheme := "https://"
		if !global.Config.Storage.UseSSL {
			scheme = "http://"
		}
		return scheme + strings.TrimRight(d, "/")
	}
	scheme := "http"
	if ctx.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + ctx.Request.Host
}

func checkPlayAccess(ctx *gin.Context) error {
	hosts := global.Config.Security.PlayAllowedRefererHosts
	if len(hosts) > 0 {
		ref := ctx.GetHeader("Referer")
		u, err := url.Parse(ref)
		if err != nil || u.Host == "" {
			return errors.New("拒绝访问")
		}
		h := strings.ToLower(u.Host)
		ok := false
		for _, allow := range hosts {
			allow = strings.ToLower(strings.TrimSpace(allow))
			if allow != "" && (h == allow || strings.HasSuffix(h, "."+allow)) {
				ok = true
				break
			}
		}
		if !ok {
			return errors.New("拒绝访问")
		}
	}
	cidrs := global.Config.Security.PlayAllowedCIDRs
	if len(cidrs) == 0 {
		return nil
	}
	ip := net.ParseIP(ctx.ClientIP())
	if ip == nil {
		return errors.New("拒绝访问")
	}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			continue
		}
		if n.Contains(ip) {
			return nil
		}
	}
	return errors.New("拒绝访问")
}
