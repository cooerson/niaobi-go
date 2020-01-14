package controller

import (
	"encoding/hex"
	"io"
	"os"
	"time"

	"golang.org/x/crypto/blake2b"

	"github.com/rs/xid"

	"github.com/kataras/iris"
	"reqing.org/ibispay/config"
	"reqing.org/ibispay/db"
	"reqing.org/ibispay/model"
	"reqing.org/ibispay/util"
)

//NewSkill 新建技能
func NewSkill(ctx iris.Context, form model.NewSkillForm) {
	e := new(model.CommonError)
	pq := GetPQ(ctx)
	coinName := GetJwtUser(ctx)[config.JwtNameKey].(string)
	lock := GetTxLocks(ctx)

	//转账和技能不能同时处理
	if lock.Locks[coinName] == true {
		e.ReturnError(ctx, iris.StatusOK, config.Public.Err.E1019)
	}
	lock.Locks[coinName] = true
	defer func() {
		delete(lock.Locks, coinName)
	}()

	//上架的技能数量不能超过200
	count, err := pq.Count(&db.Skill{Owner: coinName})
	if err != nil {
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
	}
	if count+1 > config.MaxSkillNum {
		e.ReturnError(ctx, iris.StatusOK, config.Public.Err.E1027)
	}

	//同一用户不能插入相同标题的技能
	has, err := pq.Exist(&db.Skill{Owner: coinName, Title: form.Title})
	if err != nil {
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
	}
	if has == true {
		e.ReturnError(ctx, iris.StatusInternalServerError, config.Public.Err.E1026)
	}

	err = ctx.Request().ParseMultipartForm(config.Public.Pic.MaxUploadPics)
	if err != nil {
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1016, nil)
	}

	files := ctx.Request().MultipartForm.File["files"]
	if len(files) > 9 {
		e.ReturnError(ctx, iris.StatusInternalServerError, config.Public.Err.E1036)
	}
	imgs := []*db.Img{}
	conf := config.Public.Pic
	for _, file := range files {
		//取得hash值
		f, err := file.Open()
		if err != nil {
			continue
		}
		defer f.Close()
		hash, err := blake2b.New256(nil)
		if _, err := io.Copy(hash, f); err != nil {
			continue
		}
		sum := hex.EncodeToString(hash.Sum(nil))

		//检查图片hash是否已经存在于数据库
		img := db.Img{Hash: sum}
		has, err := pq.Get(&img)
		if err != nil {
			util.LogDebugAll(err)
			continue
		}
		if has == false {
			//新的client_hash
			guid := xid.New().String()
			img.Owner = coinName
			img.GUID = guid
			//临时保存原图到路径：./files/udata/鸟币号/pic/鸟币号_guid-original.jpg
			pid := coinName + "_" + guid
			meta := db.NewJPGMeta(pid+conf.PicNameSuffixOriginal, 0, 0)
			dirOriginal := db.GetUserPicDir(coinName, meta)
			_, err = util.SaveFileTo(file, dirOriginal)
			if err != nil {
				continue
			}
			img.OriginalDir = dirOriginal
		}
		imgs = append(imgs, &img)
	}

	//出错时删除原图
	var delOnErr = func() {
		for _, img := range imgs {
			os.Remove(img.OriginalDir)
		}
	}

	//插入数据库
	skill := db.Skill{Owner: coinName, Title: form.Title, Price: form.Price, Desc: form.Desc, Tags: form.Tags, Pics: []*db.Pic{}}
	affected, err := pq.Insert(&skill)
	if err != nil {
		delOnErr()
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
	}
	if affected == 0 {
		delOnErr()
		e.ReturnError(ctx, iris.StatusInternalServerError, config.Public.Err.E1004)
	}

	ctx.JSON(&skill)

	if len(imgs) == 0 {
		return
	}
	GenThumbnails(pq, imgs, coinName, &skill)
}

//UpdateSkill 更新技能
//更新技能时，拖放图片直接上传，服务器返回图片hash值给前端，请求时仅带上图片的hash数组而不带图片
func UpdateSkill(ctx iris.Context, form model.UpdateSkillForm) {
	e := new(model.CommonError)
	pq := GetPQ(ctx)
	coinName := GetJwtUser(ctx)[config.JwtNameKey].(string)
	lock := GetTxLocks(ctx)

	//转账和技能不能同时处理
	if lock.Locks[coinName] == true {
		e.ReturnError(ctx, iris.StatusOK, config.Public.Err.E1019)
	}
	lock.Locks[coinName] = true
	defer func() {
		delete(lock.Locks, coinName)
	}()

	//检查是否是本人账号更新
	sid := form.SkillID
	skill := db.Skill{ID: sid, Owner: coinName}
	has, err := pq.Get(&skill)
	e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
	if has == false {
		e.ReturnError(ctx, iris.StatusInternalServerError, config.Public.Err.E1037)
	}

	pics := []*db.Pic{}
	if len(form.Pics) > 0 {
		for _, imgHash := range form.Pics {
			img := db.Img{Hash: imgHash}
			has, err := pq.Get(&img)
			if err != nil || has == false {
				continue
			}
			pics = append(pics, img.Thumb)
		}
	}

	skill = db.Skill{Price: form.Price, Desc: form.Desc, Tags: form.Tags, Pics: pics, Version: skill.Version}
	affected, err := pq.ID(sid).Update(&skill)
	e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
	if affected == 0 {
		e.ReturnError(ctx, iris.StatusOK, config.Public.Err.E1039)
	}

	ctx.JSON(&model.UpdateRes{Ok: true})
}

//SwitchSkill 上架下架技能，使用软删除
func SwitchSkill(ctx iris.Context) {
	e := new(model.CommonError)
	pq := GetPQ(ctx)
	coinName := GetJwtUser(ctx)[config.JwtNameKey].(string)
	lock := GetTxLocks(ctx)
	sid := ctx.Params().GetUint64Default("id", 0)
	ss, err := ctx.Params().GetBool("switch")
	e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1000, nil)

	//转账和技能不能同时处理
	if lock.Locks[coinName] == true {
		e.ReturnError(ctx, iris.StatusOK, config.Public.Err.E1019)
	}
	lock.Locks[coinName] = true
	defer func() {
		delete(lock.Locks, coinName)
	}()

	//检查是否是本人账号操作
	skill := db.Skill{ID: sid, Owner: coinName}
	has, err := pq.Exist(&skill)
	e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
	if has == false {
		e.ReturnError(ctx, iris.StatusInternalServerError, config.Public.Err.E1037)
	}

	if ss == true {
		//上架
		skill = db.Skill{Deleted: time.Time{}}
		affected, err := pq.ID(sid).Update(&skill)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
		if affected == 0 {
			e.ReturnError(ctx, iris.StatusOK, config.Public.Err.E1041)
		}
	} else {
		//下架
		affected, err := pq.Delete(&skill)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
		if affected == 0 {
			e.ReturnError(ctx, iris.StatusInternalServerError, config.Public.Err.E1040)
		}
	}

	ctx.JSON(&model.UpdateRes{Ok: true})
}
