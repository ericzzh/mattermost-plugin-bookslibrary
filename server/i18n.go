package main

import (
	"encoding/json"
	"errors"
)

var TEXTJSON = `
    {
      "no-stock":{
        "en":"No stock",
        "zh":"无库存"
      },
      "borrowing-book-limited":{
        "zh":"到达借书上限"
      },
      "record-locked":{
        "zh":"数据被锁定，请稍后再试"
      },
      "not-found":{
        "zh":"没有找到相关消息数据"
      },
      "no-stock":{
        "zh":"无库存"
      },
      "renew-limited":{
        "zh":"到达续借上限"
      },
      "choose-in-stock":{
        "zh":"没有选择书册编号"
      }
    }
`

type text map[string]string
type texts map[string]text
type i18n struct {
	locale        string
	defaultLocale string
	texts         texts
}

func NewI18n(defaultLocale string) (*i18n, error) {
	if defaultLocale == "" {
		return nil, errors.New("require-default-locale")
	}
	i18n := i18n{
		locale:        "",
		defaultLocale: defaultLocale,
		texts:         texts{},
	}
	if err := json.Unmarshal([]byte(TEXTJSON), &i18n.texts); err != nil {
		return nil, err
	}

	return &i18n, nil
}

func (i18n *i18n) GetText(key string) string {
	currLocale := ""
	if i18n.locale == "" {
		currLocale = i18n.defaultLocale
	} else {
		currLocale = i18n.locale
	}

	return i18n.texts[key][currLocale]
}
