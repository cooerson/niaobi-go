package db

import (
	"time"
)

//Coin 对应coin表，此表不可删除
//鸟币号=鸟币名称=用户名
//无需计算鸟币信用。3点：
//1.鸟币本身是基于已经存在的信用关系 2.信用本身也是不可证伪的，证伪=怀疑，正好和信用的本质相反 3.信用由直觉产生，无逻辑。每一次交易都可能产生不同的信用关系。
type Coin struct {
	ID uint64 `json:"coinID" xorm:"pk autoincr BIGINT 'id'"`

	Name     string `json:"name" xorm:"not null unique unique(coin_name_pwd_idx) VARCHAR(20)"`                      //鸟币号，不可重复、不可修改、少于20个字符，可用于登录。统一格式化为去除首尾空格的、以字母开头的、仅包含字母(Unicode)数字短横线的全小写格式，中间空格以短横线替换。
	Phone    string `json:"phone,omitempty" xorm:"not null unique unique(coin_phone_pwd_idx) VARCHAR(20)"`          //绑定手机号，不可重复，可修改，主要用于登录和找回密码。统一格式为为E164，eg.+8618612345678
	PhoneCC  string `json:"phoneCC,omitempty" xorm:"not null VARCHAR(3) 'phone_cc'"`                                //国家地区代码 Country Code
	Pwd      string `json:"-" xorm:"not null -> unique(coin_name_pwd_idx) unique(coin_phone_pwd_idx) VARCHAR(128)"` //密码加密，不从服务器返回前端
	Issued   uint64 `json:"issued" xorm:"not null default 0 index BIGINT"`                                          //普通鸟币——当前发行量
	Denied   uint64 `json:"denied" xorm:"not null default 0 index BIGINT"`                                          //普通鸟币——当前拒绝量
	BreakNum uint32 `json:"breakNum" xorm:"not null default 0 INTEGER"`                                             //超级鸟币——当前拒绝兑现的「次数」
	SkillNum uint32 `json:"skillNum" xorm:"not null default 0 INTEGER"`                                             //当前可用的技能数

	Bio    string `json:"bio,omitempty" xorm:"TEXT"`          //技能简介，少于5000字
	Email  string `json:"email,omitempty" xorm:"VARCHAR(30)"` //邮箱
	Avatar Pic    `json:"avatar,omitempty" xorm:"JSONB"`      //头像，大小参考config
	Qrc    Pic    `json:"qrc,omitempty" xorm:"JSONB"`         //收款二维码（根据鸟币号生成），大小参考config

	Created time.Time `json:"created" xorm:"not null created"`
	Updated time.Time `json:"updated" xorm:"updated"`
}

//TransLocks 交易锁
//只允许付款方和收款方同时进行一个交易
type TransLocks struct {
	Locks map[string]bool
}
