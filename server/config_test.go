package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	td := NewTestData()
	plugin := td.NewMockPlugin()
        plugin.expiredDays = 31
        plugin.maxRenewTimes = 3
	api := td.ApiMockCommon()
	plugin.SetAPI(api)

	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/config", bytes.NewReader([]byte{}))
	plugin.ServeHTTP(nil, w, r)

	result := w.Result()
	var resultObj *Result
	json.NewDecoder(result.Body).Decode(&resultObj)
	require.NotEqual(t, result.StatusCode, 404, "should find this service")
	require.Empty(t, resultObj.Error, "should not be error")

        var bc Config
        json.Unmarshal([]byte(resultObj.Messages["data"]), &bc)

        assert.Equalf(t, 3, bc.MaxRenewTimes, "max renew times")
        assert.Equalf(t, 31, bc.ExpiredDays, "expired times")
}
