package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type testRouteParam struct {
	Key   string
	Value string
}

func setUserID(id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", id)
		c.Next()
	}
}

func validUserID() string { return primitive.NewObjectID().Hex() }

func newHandlerTestContext(t *testing.T, method, target string, body any, params ...testRouteParam) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = newHandlerTestRequest(t, method, target, body)
	for _, param := range params {
		c.Params = append(c.Params, gin.Param{Key: param.Key, Value: param.Value})
	}
	return c, w
}

func serveHandlerTestRequest(t *testing.T, h http.Handler, method, target string, body any) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, newHandlerTestRequest(t, method, target, body))
	return w
}

func newHandlerTestRequest(t *testing.T, method, target string, body any) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, target, testRequestBody(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

func testRequestBody(t *testing.T, body any) io.Reader {
	t.Helper()
	switch v := body.(type) {
	case nil:
		return nil
	case io.Reader:
		return v
	case []byte:
		return bytes.NewReader(v)
	case string:
		return bytes.NewBufferString(v)
	default:
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return bytes.NewReader(data)
	}
}
