package model

//NewPayForm 新的发行或转手
type NewPayForm struct {
	TransCoin string `json:"transCoin" validate:"required,lte=20" format:"trim"`         //交易的鸟币名
	Receiver  string `json:"receiver" validate:"required,lte=20" format:"trim"`          //收款方鸟币号
	Amount    uint64 `json:"amount" validate:"required,numeric,gte=1" format:"num,trim"` //转账数额，大于0的整数
	IsMarker  bool   `json:"isMarker"`                                                   //是否是血盟，血盟为true时，忽略技能快照组snap_set_id
}

//===========err trans=============

//NewPayFieldTrans 字段本地化，供validator使用
func (form NewPayForm) NewPayFieldTrans() FieldTrans {
	m := FieldTrans{}
	m["TransCoin"] = "交易的鸟币名"
	m["Receiver"] = "收款方鸟币号"
	m["IsMarker"] = "血盟标记"
	m["Amount"] = "转账数额"
	return m
}
