package config

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"

	"github.com/genjidb/genji"
	"github.com/gin-gonic/gin"
	"github.com/pingcap/ng-monitoring/utils/testutil"
	"github.com/stretchr/testify/require"
)

type testSuite struct {
	tmpDir string
	db     *genji.DB
}

func (ts *testSuite) setup(t *testing.T) {
	var err error
	ts.tmpDir, err = ioutil.TempDir(os.TempDir(), "ngm-test-.*")
	require.NoError(t, err)
	ts.db = testutil.NewGenjiDB(t, ts.tmpDir)
	GetDB = func() *genji.DB {
		return ts.db
	}
	def := GetDefaultConfig()
	StoreGlobalConfig(&def)
	err = LoadConfigFromStorage(func() *genji.DB {
		return ts.db
	})
	require.NoError(t, err)
}

func (ts *testSuite) close(t *testing.T) {
	err := ts.db.Close()
	err = os.RemoveAll(ts.tmpDir)
	require.NoError(t, err)
}

func TestHTTPService(t *testing.T) {
	ts := testSuite{}
	ts.setup(t)
	defer ts.close(t)
	addr := setupHTTPService(t)
	resp, err := http.Get("http://" + addr + "/config")
	require.NoError(t, err)
	data, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	cfg := Config{}
	require.Equal(t, len(data) > 10, true)
	err = json.Unmarshal(data, &cfg)
	require.NoError(t, err)

	res, err := http.Post("http://"+addr+"/config", "application/json", bytes.NewReader([]byte(`{"continuous_profiling": {"enable": true,"profile_seconds":6,"interval_seconds":11}}`)))
	require.NoError(t, err)
	require.Equal(t, 200, res.StatusCode)
	globalCfg := GetGlobalConfig()
	require.Equal(t, true, globalCfg.ContinueProfiling.Enable)
	require.Equal(t, 6, globalCfg.ContinueProfiling.ProfileSeconds)
	require.Equal(t, 11, globalCfg.ContinueProfiling.IntervalSeconds)

	// test for post invalid config
	res, err = http.Post("http://"+addr+"/config", "application/json", bytes.NewReader([]byte(`{"continuous_profiling": {"enable": true,"profile_seconds":1000,"interval_seconds":11}}`)))
	require.NoError(t, err)
	require.Equal(t, 503, res.StatusCode)
	body, err := ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, `{"message":"new config is invalid: {\"data_retention_seconds\":259200,\"enable\":true,\"interval_seconds\":11,\"profile_seconds\":1000,\"timeout_seconds\":120}","status":"error"}`, string(body))

	// test empty body config
	res, err = http.Post("http://"+addr+"/config", "application/json", bytes.NewReader([]byte(``)))
	require.NoError(t, err)
	require.Equal(t, 503, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, `{"message":"EOF","status":"error"}`, string(body))

	// test unknown config
	res, err = http.Post("http://"+addr+"/config", "application/json", bytes.NewReader([]byte(`{"unknown_module": {"enable": true}}`)))
	require.NoError(t, err)
	require.Equal(t, 503, res.StatusCode)
	body, err = ioutil.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, `{"message":"config unknown_module not support modify or unknow","status":"error"}`, string(body))

	globalCfg = GetGlobalConfig()
	require.Equal(t, true, globalCfg.ContinueProfiling.Enable)
	require.Equal(t, 6, globalCfg.ContinueProfiling.ProfileSeconds)
	require.Equal(t, 11, globalCfg.ContinueProfiling.IntervalSeconds)
}

func setupHTTPService(t *testing.T) string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	gin.SetMode(gin.ReleaseMode)
	ng := gin.New()

	ng.Use(gin.Recovery())
	configGroup := ng.Group("/config")
	HTTPService(configGroup)
	httpServer := &http.Server{Handler: ng}

	go func() {
		if err = httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			require.NoError(t, err)
		}
	}()
	return listener.Addr().String()
}
