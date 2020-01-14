package controller

import (
	"errors"
	"os"

	"github.com/asaskevich/govalidator"
	"github.com/go-xorm/xorm"
	"github.com/kataras/iris"
	"github.com/rs/xid"
	"gopkg.in/h2non/bimg.v1"
	"reqing.org/ibispay/config"
	"reqing.org/ibispay/db"
	"reqing.org/ibispay/model"
	"reqing.org/ibispay/util"
)

//NewPic 上传图片
func NewPic(ctx iris.Context) {
	e := new(model.CommonError)
	pq := GetPQ(ctx)
	coinName := GetJwtUser(ctx)[config.JwtNameKey].(string)

	_, header, err := ctx.FormFile("file")
	if err != nil {
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1015, nil)
	}

	//临时保存原图到路径：./files/udata/鸟币号/pic/鸟币号_guid-original.jpg
	guid := xid.New().String()
	pid := coinName + "_" + guid
	meta := db.NewJPGMeta(pid+config.Public.Pic.PicNameSuffixOriginal, 0, 0)
	dirOriginal := db.GetUserPicDir(coinName, meta)
	_, err = util.SaveFileTo(header, dirOriginal)
	if err != nil {
		os.Remove(dirOriginal)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1015, nil)
	}

	//取得hash值
	checksum, err := util.GetHash256(dirOriginal)
	if err != nil {
		os.Remove(dirOriginal)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1015, nil)
	}

	//检查图片hash是否已经存在于数据库
	img := db.Img{Hash: checksum}
	has, err := pq.Get(&img)
	if err != nil {
		util.LogDebugAll(err)
		os.Remove(dirOriginal)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1015, nil)
	}
	if has == true {
		//图片已经存在
		os.Remove(dirOriginal)
		ctx.JSON(&img)
		return
	}

	//新的client_hash
	img.Owner = coinName
	img.GUID = guid
	img.OriginalDir = dirOriginal
	ctx.JSON(&img)

	GenThumbnail(pq, &img, coinName)
}

//CheckPicHash 检查图片是否已经上传过
func CheckPicHash(ctx iris.Context) {
	e := new(model.CommonError)
	pq := GetPQ(ctx)
	hash := ctx.Params().Get("hash")

	//检查图片hash是否已经存在于数据库
	img := db.Img{Hash: hash}
	has, err := pq.Get(&img)
	if err != nil {
		util.LogDebugAll(err)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1004, nil)
	}
	if has == true {
		ctx.JSON(&model.ImgExistRes{Exist: true})
		return
	}

	ctx.JSON(&model.ImgExistRes{Exist: false})
}

//GenThumbnail 生成单张图片的缩略图
func GenThumbnail(pq *xorm.Engine, img *db.Img, coinName string) {
	images := []*db.Img{img}
	GenThumbnails(pq, images, coinName, nil)
}

//GenThumbnails 生成多张图片的缩略图，并且更新到技能
func GenThumbnails(pq *xorm.Engine, imgs []*db.Img, coinName string, skill *db.Skill) {
	//生成图片缩略图，并且更新对应字段
	go func(pq *xorm.Engine, images []*db.Img, coinName string, skill *db.Skill) {
		pics := []*db.Pic{}
		conf := config.Public.Pic
		newInsertedImg := []*db.Img{}
		for _, img := range images {
			pic := new(db.Pic)
			if govalidator.IsNull(img.OriginalDir) == false {
				//===图片首次上传===
				pidPrefix := coinName + "_" + img.GUID
				var delPic = func(err error) bool {
					if err != nil {
						util.LogDebugAll(err)
						os.Remove(img.OriginalDir)
						return true
					}
					return false
				}

				//-----获取图片宽高信息-----
				util.LogDebug(img.OriginalDir)

				file, err := os.Open(img.OriginalDir)
				if delPic(err) {
					continue
				}
				defer file.Close()

				w, h := util.GetPicDimensions(file)

				//-------计算缩略图大小，并把数据写入数组----------
				var getNewWH = func(w int, h int, longer float64, shorter float64, configSize float64) (newW uint, newH uint) {
					scale := configSize / longer
					longer = configSize
					shorter = shorter * scale
					if w > h {
						return uint(longer), uint(shorter)
					}
					return uint(shorter), uint(longer)
				}
				var getLongPicWH = func(w int, h int, longer float64, shorter float64, configSize float64) (newW uint, newH uint) {
					scale := configSize / shorter
					shorter = configSize
					longer = longer * scale
					if w > h {
						return uint(longer), uint(shorter)
					}
					return uint(shorter), uint(longer)
				}

				longer := float64(w)
				shorter := float64(h)
				if w < h {
					longer = float64(h)
					shorter = float64(w)
				}
				//图片太长，最大长宽比为短边:长边=0.025
				if shorter/longer < conf.SkillPicScaleMax {
					delPic(errors.New(config.Public.Err.E1017))
					continue
				}

				//注意：bimg默认的内存缓存是100M
				buffer, err := bimg.Read(img.OriginalDir)
				if (longer-shorter)/longer < 0.5 {
					//正常比例图，以长边对准设定值
					biggest := float64(conf.SkillPicBiggest)
					large := float64(conf.SkillPicLarge)
					middle := float64(conf.SkillPicMiddle)
					small := float64(conf.SkillPicSmall)
					if longer > biggest {
						w, h := getNewWH(w, h, longer, shorter, biggest)
						pic.Biggest = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixBiggest, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Biggest, true)) {
							continue
						}
					} else {
						pic.Biggest = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixBiggest, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Biggest, false)) {
							continue
						}
					}
					if longer > large {
						w, h := getNewWH(w, h, longer, shorter, large)
						pic.Large = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixLarge, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Large, true)) {
							continue
						}
					} else {
						pic.Large = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixLarge, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Large, false)) {
							continue
						}
					}
					if longer > middle {
						w, h := getNewWH(w, h, longer, shorter, middle)
						pic.Middle = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixMiddle, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Middle, true)) {
							continue
						}
					} else {
						pic.Middle = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixMiddle, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Middle, false)) {
							continue
						}
					}
					if longer > small {
						w, h := getNewWH(w, h, longer, shorter, small)
						pic.Small = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixSmall, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Small, true)) {
							continue
						}
					} else {
						pic.Small = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixSmall, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Small, false)) {
							continue
						}
					}
				} else {
					//长图，以短边对准设定值
					bigOri := float64(conf.SkillPicLongBigOri)
					ori := float64(conf.SkillPicLongOri)
					bigThum := float64(conf.SkillPicLongBigThum)
					thum := float64(conf.SkillPicLongThum)
					if shorter > bigOri {
						w, h := getLongPicWH(w, h, longer, shorter, bigOri)
						pic.Biggest = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixBiggest, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Biggest, true)) {
							continue
						}
					} else {
						pic.Biggest = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixBiggest, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Biggest, false)) {
							continue
						}
					}
					if shorter > ori {
						w, h := getLongPicWH(w, h, longer, shorter, ori)
						pic.Large = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixLarge, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Large, true)) {
							continue
						}
					} else {
						pic.Large = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixLarge, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Large, false)) {
							continue
						}
					}
					if shorter > bigThum {
						w, h := getLongPicWH(w, h, longer, shorter, bigThum)
						pic.Middle = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixMiddle, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Middle, true)) {
							continue
						}
					} else {
						pic.Middle = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixMiddle, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Middle, false)) {
							continue
						}
					}
					if shorter > thum {
						w, h := getLongPicWH(w, h, longer, shorter, thum)
						pic.Small = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixSmall, w, h)
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Small, true)) {
							continue
						}
					} else {
						pic.Small = db.NewJPGMeta(pidPrefix+conf.PicNameSuffixSmall, uint(w), uint(h))
						if delPic(db.CompressUserJPG(coinName, buffer, pic.Small, false)) {
							continue
						}
					}
				}
				img.Thumb = pic

				//数据库插入新图hash
				_, err = pq.Insert(img)
				if err == nil {
					newInsertedImg = append(newInsertedImg, img)
				}
			} else {
				//===图片已经上传过===
				pic = img.Thumb
			}
			//删除新原图
			os.Remove(img.OriginalDir)

			pics = append(pics, pic)
		}

		//出错时，删除缩略图
		var delOnErr = func() {
			for _, img := range newInsertedImg {
				pq.Delete(img)
			}
			for _, pic := range pics {
				os.Remove(db.GetUserPicDir(coinName, pic.Biggest))
				os.Remove(db.GetUserPicDir(coinName, pic.Large))
				os.Remove(db.GetUserPicDir(coinName, pic.Middle))
				os.Remove(db.GetUserPicDir(coinName, pic.Small))
			}
		}

		if skill != nil {
			//更新skill缩略图
			skill.Pics = pics
			affected, err := pq.ID(skill.ID).Update(skill)
			if affected == 0 || err != nil {
				delOnErr()
			}
		}
	}(pq, imgs, coinName, skill)
}
