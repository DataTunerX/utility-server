package parser

import (
	"testing"
)

func TestReplaceTemplate(t *testing.T) {
	// 测试用例1：正常替换
	yamlTemplate := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: {{ .Name }}\nspec:\n  containers:\n  - name: my-container\n    image: {{ .Image }}"
	data := map[string]interface{}{
		"Name":  "my-pod",
		"Image": "nginx:latest",
	}
	expectedResult := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: my-pod\nspec:\n  containers:\n  - name: my-container\n    image: nginx:latest"

	result, err := ReplaceTemplate(yamlTemplate, data)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result != expectedResult {
		t.Errorf("Expected:\n%s\n\nGot:\n%s", expectedResult, result)
	}

	// 测试用例2：模板错误
	invalidYAML := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: {{ .Name\nspec:\n  containers:\n  - name: my-container\n    image: {{ .Image }}"
	_, err = ReplaceTemplate(invalidYAML, data)
	if err == nil {
		t.Error("Expected an error for invalid YAML template, but got none.")
	}
}
