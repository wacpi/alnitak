package cron

import "github.com/jasonlvhit/gocron"

func StartCronTask() {
	c := gocron.NewScheduler()

	// 每3小时刷新同步播放量数据
	c.Every(3).Hours().Do(SyncClicks)

	// 每3小时刷新一次热点
	c.Every(3).Hours().Do(RefreshPopular)

	// 每天晚上12点解封用户
	c.Every(1).Day().At("00:00").Do(UnbanUser)

	// 每天凌晨3点清理孤立资源
	c.Every(1).Day().At("03:00").Do(CleanupOrphanedResources)

	<-c.Start()
}
