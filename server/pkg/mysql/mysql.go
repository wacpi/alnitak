package mysql

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"interastral-peace.com/alnitak/internal/config"
	"interastral-peace.com/alnitak/utils"
	"moul.io/zapgorm2"
)

var db *gorm.DB

func Init(c config.Mysql) *gorm.DB {
	dns := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s", c.Username, c.Password, c.Host, c.Port, c.Datasource, c.Param)

	// 配置zapgorm2，忽略ErrRecordNotFound错误
	zapLogger := zapgorm2.New(zap.L())
	zapLogger.SetAsDefault()
	zapLogger.IgnoreRecordNotFoundError = true // 关键配置：忽略ErrRecordNotFound错误
	zapLogger.LogLevel = logger.Warn
	zapLogger.SlowThreshold = 2 * time.Second

	if mysqlClient, err := gorm.Open(mysql.Open(dns), &gorm.Config{Logger: zapLogger}); err != nil {
		utils.ErrorLog("mysql连接失败", "db", err.Error())
		panic(err)
	} else {
		zap.L().Info("mysql连接成功", zap.String("module", "db"))
		db = mysqlClient
		return mysqlClient
	}
}

func GetMysqlClient() *gorm.DB {
	return db
}
