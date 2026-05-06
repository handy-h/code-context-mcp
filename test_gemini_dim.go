//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
)

func main() {
	// 测试不同的配置
	testCases := []struct {
		model string
		dim   int
	}{
		{"embedding-001", 768},
		{"gemini-embedding-2", 768},
		{"gemini-embedding-2", 3072},
	}

	for _, tc := range testCases {
		fmt.Printf("\n测试模型: %s, 维度: %d\n", tc.model, tc.dim)
		
		reqBody := map[string]interface{}{
			"model": fmt.Sprintf("models/%s", tc.model),
			"content": map[string]interface{}{
				"parts": []map[string]interface{}{
					{
						"text": "测试文本",
					},
				},
			},
			"taskType": "RETRIEVAL_DOCUMENT",
		}
		
		// 添加output_dimensionality参数
		if tc.dim > 0 {
			reqBody["output_dimensionality"] = tc.dim
		}
		
		jsonData, _ := json.MarshalIndent(reqBody, "", "  ")
		fmt.Printf("请求体:\n%s\n", string(jsonData))
	}
}