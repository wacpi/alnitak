package vo

type RelationResp struct {
	Uid        uint         `json:"-"`
	TargetUid  uint         `json:"-"`
	Relation   int          `json:"relation"`
	MyRelation int          `json:"myRelation" gorm:"-"`
	User       UserInfoResp `json:"user" gorm:"-"`
}
