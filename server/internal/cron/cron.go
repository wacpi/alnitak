package cron

import (
	"github.com/jasonlvhit/gocron"
	"go.uber.org/zap"
)

// safeDo 包装定时任务函数，自动 recover panic 避免单个任务炸掉整个 cron goroutine
func safeDo(fn func()) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				zap.L().Error("cron任务panic恢复",
					zap.Any("recover", r),
					zap.Stack("stack"))
			}
		}()
		fn()
	}
}

func StartCronTask() {
	c := gocron.NewScheduler()

	// 每3小时刷新同步播放量数据
	c.Every(3).Hours().Do(safeDo(SyncClicks))

	// 每3小时刷新一次热点
	c.Every(3).Hours().Do(safeDo(RefreshPopular))

	// 每天晚上12点解封用户
	c.Every(1).Day().At("00:00").Do(safeDo(UnbanUser))

	// 每天凌晨3点清理孤立资源
	c.Every(1).Day().At("03:00").Do(safeDo(CleanupOrphanedResources))

	// 每30分钟重试备用OSS上传失败记录
	c.Every(30).Minutes().Do(safeDo(RetryBackupFailures))

	<-c.Start()
}
