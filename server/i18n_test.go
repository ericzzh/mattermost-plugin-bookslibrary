package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestI18n(t *testing.T)  {
    
     i18n, err := NewI18n("zh")
     assert.Nilf(t, err, "should be no error")
     assert.Equalf(t, "无库存", i18n.GetText("no-stock"),"no-stock")

}
