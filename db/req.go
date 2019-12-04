package db

import (
	"time"
)

//Req 兑现请求(request)，对应req表
/**
兑现状态 state：
1.已发送兑现请求，等待对方确认/需要确认兑现请求（24h，否则自动标记失败）
2.对方接受了兑现请求，鸟币已被回收(显示"未兑现"和"已兑现"按钮)/鸟币已成功回收，提示请自行完成兑现
3.对方拒绝了你的兑现请求(包括未及时处理系统自动拒绝)/您拒绝了兑现请求
4.兑现未执行(兑现者手动选择的状态，需要添加note说明)/对方认为兑现失败
5.兑现已完成
6.兑现失败（由于鸟币不足等原因）
*/
type Req struct {
	ID uint64 `json:"reqID" xorm:"pk BIGINT autoincr 'id'"`
	//用于兑现的鸟币ID
	CoinID uint64 `json:"coinID" xorm:"not null 'coin_id' index(req_bearer_addr_coin_id_idx) index(req_bearer_id_coin_id_idx) index index(req_issuer_addr_coin_id_idx) index(req_issuer_id_coin_id_idx) BIGINT"`
	//具体要兑现的技能ID
	SkillID uint64 `json:"skillID" xorm:"not null BIGINT 'skill_id'"`
	//持有者userID
	BearerID uint64 `json:"bearerID" xorm:"not null 'bearer_id' index(req_bearer_id_coin_id_idx) index index(req_bearer_id_issuer_id_idx) BIGINT"`
	//发行者userID
	IssuerID uint64 `json:"issuerID" xorm:"not null 'issuer_id' index(req_bearer_addr_issuer_id_idx) index(req_bearer_id_issuer_id_idx) index(req_issuer_id_coin_id_idx) index BIGINT"`
	//持有者的鸟币地址
	BearerAddr string `json:"bearerAddr" xorm:"not null index(req_bearer_addr_coin_id_idx) index index(req_bearer_addr_issuer_id_idx) VARCHAR(1024)"`
	//发行者的鸟币地址
	IssuerAddr string `json:"issuerAddr" xorm:"not null index(req_issuer_addr_coin_id_idx) index VARCHAR(1024)"`
	//是否是血盟，是则忽略skill_id
	IsMarker bool `json:"isMarker" xorm:"not null BOOL"`
	//兑现的鸟币数量，大于0的整数
	Amount uint64 `json:"amount" xorm:"not null BIGINT"`
	//兑现状态（兑现时需要发行者确认，默认24小时响应，超时自动视为拒绝)
	State uint8 `json:"state" xorm:"not null default 1 SMALLINT"`
	//issuer对状态4的说明
	Note    string    `json:"note" xorm:"VARCHAR(512)"`
	Created time.Time `json:"created" xorm:"not null created"`
	Updated time.Time `json:"updated" xorm:"updated"`
}
