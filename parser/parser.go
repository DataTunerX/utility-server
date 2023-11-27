package parser

import (
	"bytes"
	"regexp"
	"text/template"
)

func ReplaceTemplate(yamlContent string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("yamlTemplate").Parse(yamlContent)
	if err != nil {
		return "", err
	}

	var replacedYAML bytes.Buffer
	err = tmpl.Execute(&replacedYAML, data)
	if err != nil {
		return "", err
	}

	// 使用正则表达式替换模板变量为 ""
	re := regexp.MustCompile(`{{\s*\..*?\s*}}`)
	replacedStr := re.ReplaceAllString(replacedYAML.String(), "")

	return replacedStr, nil
}
