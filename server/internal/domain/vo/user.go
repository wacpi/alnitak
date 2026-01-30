package vo

import (
	"time"
)

const (
	USER_BASE_INFO_FIELD = "`id`,`username`,`sign`,`avatar`,`gender`"
)

type UserInfoResp struct {
	ID         uint      `json:"uid"`
	Username   string    `json:"name"`
	Sign       string    `json:"sign"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Status     int       `json:"status"`
	Avatar     string    `json:"avatar"`
	Gender     int       `json:"gender"`
	SpaceCover string    `json:"spaceCover"`
	Birthday   time.Time `json:"birthday"`
	CreatedAt  time.Time `json:"createdAt"`
	Fans       int64     `json:"fans" gorm:"-"` // 粉丝数量
}

type UserBaseInfoResp struct {
	ID       uint   `json:"uid"`
	Username string `json:"name"`
	Sign     string `json:"sign"`
	Avatar   string `json:"avatar"`
	Gender   int    `json:"gender"`
}

type UserInfoManageResp struct {
	ID         uint      `json:"uid"`
	Username   string    `json:"name"`
	Sign       string    `json:"sign"`
	Email      string    `json:"email"`
	Phone      string    `json:"phone"`
	Status     int       `json:"status"`
	Avatar     string    `json:"avatar"`
	Gender     int       `json:"gender"`
	SpaceCover string    `json:"spaceCover"`
	Birthday   time.Time `json:"birthday"`
	CreatedAt  time.Time `json:"createdAt"`
	Role       string    `json:"role"`
}

type UserBanRecordResp struct {
	ID        uint      `json:"id"`
	EndTime   time.Time `json:"endTime"`
	Reason    string    `json:"reason"`
	Status    int       `json:"status"`
	Operator  uint      `json:"operator"`
	CreatedAt time.Time `json:"createdAt"`
}

type BanResp struct {
	EndTime   time.Time `json:"endTime"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}
