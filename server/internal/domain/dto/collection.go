package dto

type AddCollectionReq struct {
	Name string `json:"name" form:"name"`
}

type EditCollectionReq struct {
	ID    uint   `json:"id" form:"id"`
	Cover string `json:"cover" form:"cover"`
	Name  string `json:"name" form:"name"`
	Desc  string `json:"desc" form:"desc"`
	Open  bool   `json:"open" form:"open"`
}
