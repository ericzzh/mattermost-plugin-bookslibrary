package main

// import (
// 	"bytes"
// 	"encoding/json"
// 	"net/http/httptest"
// 	"testing"
// 
// 	"github.com/stretchr/testify/assert"
// 	"github.com/stretchr/testify/require"
// )
// 
// func TestBookConfig(t *testing.T) {
// 	td := NewTestData()
// 	plugin := td.NewMockPlugin()
// 	api := td.ApiMockCommon()
// 	plugin.SetAPI(api)
// 
// 	w := httptest.NewRecorder()
// 	r := httptest.NewRequest("GET", "/bookconfig", bytes.NewReader([]byte{}))
// 	plugin.ServeHTTP(nil, w, r)
// 
// 	result := w.Result()
// 	var resultObj *Result
// 	json.NewDecoder(result.Body).Decode(&resultObj)
// 	require.NotEqual(t, result.StatusCode, 404, "should find this service")
// 	require.Empty(t, resultObj.Error, "should not be error")
// 
//         var bc BookConfig
//         json.Unmarshal([]byte(resultObj.Messages["data"]), &bc)
// 
//         assert.Equalf(t, 2, bc.MaxRenewTimes, "max renew times")
//         assert.Equalf(t, 30, bc.ExpiredDays, "expired times")
// }
