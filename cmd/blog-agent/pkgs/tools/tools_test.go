package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestCalculate(t *testing.T) {
	tests := []struct {
		expression string
		want       float64
	}{
		{expression: "2+3*4", want: 14},
		{expression: "(2+3)*4", want: 20},
		{expression: "-3.5+8/2", want: 0.5},
		{expression: "10-2*3+1", want: 5},
	}
	for _, tt := range tests {
		result := Calculate(tt.expression)
		if result.Error != "" || result.Result != tt.want {
			t.Fatalf("Calculate(%q) = (%v, %q), want (%v, empty)", tt.expression, result.Result, result.Error, tt.want)
		}
	}
}

func TestCalculateRejectsInvalidExpressions(t *testing.T) {
	for _, expression := range []string{"", "1/0", "(1+2", "1..2", "2+abc"} {
		if result := Calculate(expression); result.Error == "" {
			t.Fatalf("Calculate(%q) 应返回错误", expression)
		}
	}
}

func TestToolHandlers(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
		form    url.Values
		check   func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "数据处理返回真实 MD5", handler: DataProcessHandler, method: http.MethodPost,
			form: url.Values{"action": {"md5"}, "input": {"hello"}},
			check: func(t *testing.T, body map[string]interface{}) {
				if body["output"] != "5d41402abc4b2a76b9719d911017c592" || body["valid"] != true {
					t.Fatalf("unexpected data response: %#v", body)
				}
			},
		},
		{
			name: "四则运算", handler: CalculatorHandler, method: http.MethodPost,
			form: url.Values{"expression": {"(2+3)*4"}},
			check: func(t *testing.T, body map[string]interface{}) {
				if body["result"] != float64(20) {
					t.Fatalf("unexpected calculator response: %#v", body)
				}
			},
		},
		{
			name: "BMI", handler: BMIHandler, method: http.MethodPost,
			form: url.Values{"height": {"170"}, "weight": {"70"}},
			check: func(t *testing.T, body map[string]interface{}) {
				if body["bmi"] != float64(24.22) {
					t.Fatalf("unexpected BMI response: %#v", body)
				}
			},
		},
		{
			name: "单位转换", handler: UnitConvertHandler, method: http.MethodPost,
			form: url.Values{"value": {"1"}, "from_unit": {"km"}, "to_unit": {"m"}, "unit_type": {"length"}},
			check: func(t *testing.T, body map[string]interface{}) {
				if body["converted_value"] != float64(1000) {
					t.Fatalf("unexpected unit response: %#v", body)
				}
			},
		},
		{
			name: "文本统计", handler: TextToolHandler, method: http.MethodPost,
			form: url.Values{"action": {"count"}, "text": {"你好 world"}},
			check: func(t *testing.T, body map[string]interface{}) {
				if body["characters"] != float64(8) || body["words"] != float64(2) {
					t.Fatalf("unexpected text response: %#v", body)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path
			if path == "" {
				path = "/"
			}
			request := httptest.NewRequest(tt.method, path, strings.NewReader(tt.form.Encode()))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response := httptest.NewRecorder()
			tt.handler(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			var body map[string]interface{}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			tt.check(t, body)
		})
	}
}
