package model

import (
	zhongwen "github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/kataras/iris"
	"github.com/kataras/iris/hero"
	validator "gopkg.in/go-playground/validator.v9"
	zh_translations "gopkg.in/go-playground/validator.v9/translations/zh"
	"reqing.org/ibispay/config"
	"reqing.org/ibispay/util"
)

var (
	validate *validator.Validate
	trans    ut.Translator
)

//BindForm 绑定model到对应controller的handler
//例如：login()绑定loginform到对应的loginHandler
func BindForm() {
	//实例化需要转换的语言
	zh := zhongwen.New()
	uni := ut.New(zh, zh)
	trans, _ = uni.GetTranslator("zh")
	validate = validator.New()
	//注册转换的语言为默认语言
	zh_translations.RegisterDefaultTranslations(validate, trans)

	//=====bind & check=====
	//user
	register()
	login()
	newPwd()
	newProfie()
	//skill
	newSkill()
	//pay
	newPay()
}

func register() {
	hero.Register(func(ctx iris.Context) (form RegisterForm) {
		e := new(CommonError)
		//---bind form---
		err := ctx.ReadJSON(&form)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1000, nil)

		//---check struct---
		err = validate.Struct(&form)
		errors := NewValidatorErrorDetail(trans, err, form.RegisterFieldTrans())
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1001, errors)

		err = util.Strings(&form)
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1002, nil)
		return
	})
}

func login() {
	hero.Register(func(ctx iris.Context) (form LoginForm) {
		e := new(CommonError)
		//---bind form---
		err := ctx.ReadJSON(&form)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1000, nil)

		//---check struct---
		err = validate.Struct(&form)
		errors := NewValidatorErrorDetail(trans, err, form.LoginFieldTrans())
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1001, errors)

		err = util.Strings(&form)
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1002, nil)
		return
	})
}

func newPwd() {
	hero.Register(func(ctx iris.Context) (form NewPwdForm) {
		e := new(CommonError)
		//---bind form---
		err := ctx.ReadJSON(&form)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1000, nil)

		//---check struct---
		err = validate.Struct(&form)
		errors := NewValidatorErrorDetail(trans, err, form.NewPwdFieldTrans())
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1001, errors)

		err = util.Strings(&form)
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1002, nil)
		return
	})
}

func newProfie() {
	hero.Register(func(ctx iris.Context) (form ProfileForm) {
		e := new(CommonError)
		//---bind form---
		err := ctx.ReadJSON(&form)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1000, nil)

		//---check struct---
		err = validate.Struct(&form)
		errors := NewValidatorErrorDetail(trans, err, form.ProfileFieldTrans())
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1001, errors)

		err = util.Strings(&form)
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1002, nil)
		return
	})
}

func newSkill() {
	hero.Register(func(ctx iris.Context) (form NewSkillForm) {
		e := new(CommonError)
		// ---bind form---
		err := ctx.ReadForm(&form)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1000, nil)

		//---check struct---
		err = validate.Struct(&form)
		errors := NewValidatorErrorDetail(trans, err, form.NewSkillFieldTrans())
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1001, errors)

		err = util.Strings(&form)
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1002, nil)
		return
	})
}

func newPay() {
	hero.Register(func(ctx iris.Context) (form NewPayForm) {
		e := new(CommonError)
		// ---bind form---
		err := ctx.ReadJSON(&form)
		e.CheckError(ctx, err, iris.StatusInternalServerError, config.Public.Err.E1000, nil)

		//---check struct---
		err = validate.Struct(&form)
		errors := NewValidatorErrorDetail(trans, err, form.NewPayFieldTrans())
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1001, errors)

		err = util.Strings(&form)
		e.CheckError(ctx, err, iris.StatusNotAcceptable, config.Public.Err.E1002, nil)
		return
	})
}
