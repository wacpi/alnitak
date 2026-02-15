package dto

type AddCommentReq struct {
	Cid           uint     `json:"cid" form:"cid"`
	Content       string   `json:"content" form:"content"`
	At            []string `json:"at" form:"at"`
	ParentID      uint     `json:"parentID" form:"parentID"`
	ReplyUserID   uint     `json:"replyUserID" form:"replyUserID"`
	ReplyUserName string   `json:"replyUserName" form:"replyUserName"`
	ReplyContent  string   `json:"replyContent" form:"replyContent"`
}
