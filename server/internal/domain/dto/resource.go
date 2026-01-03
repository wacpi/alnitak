package dto

// 修改资源标题
type ModifyResourceTitleReq struct {
	ID    uint
	Title string
}

// 替换资源请求
type ReplaceResourceReq struct {
	ResourceID uint   `json:"resourceId"` // 要替换的资源ID
	Hash       string `json:"hash"`       // 新视频文件的hash
}
